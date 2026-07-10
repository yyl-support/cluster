---
name: add-new-test-case
description: Guide for adding new E2E test cases for the converter. Use when creating test directories under go/cmd/converter/case/newtest/, updating convertv2_to_yaml_test.go, and adding entries to test-cases.json.
---

# Adding New Test Cases

This skill guides you through creating a new converter E2E test case, using test27-310p3 as the reference example.

## Overview

Two testing layers exist:

| Layer | Mechanism | Purpose |
|-------|-----------|---------|
| Go E2E | `Test_main` in `convertv2_to_yaml_test.go` | Validates converter YAML output (file-based, no cluster) |
| Volcano Job | `skill submit-test` + `test-cases.json` | Validates on real K8s cluster with `eval.sh` |

Both must be updated when adding a new test case.

## Step-by-Step Guide

### Step 1: Create Test Directory

```
go/cmd/converter/case/newtest/test<N>-<descriptive-name>/
```

Pick the next test number (look at existing directories). Use a descriptive name like `310p3`, `910b4`, `npu-generic`, `dataset`.

### Step 2: Create Required Files

Each test directory must contain:

#### env.sh (Required)

Shell-style env vars that the converter reads as input.

```bash
export CP_runs_on="arm64-310p3-1"           # The runs_on spec (arch-chip-count)
export CP_docker_image="swr.cn-southwest-2.myhuaweicloud.com/base_image/ascend-ci/cann:8.2.rc1-310p-ubuntu22.04-py3.11"
export CP_pipeline_run_id="test-310p3-123"   # Unique ID for this test
export CP_merge_id="110"                      # MR number (or empty for no-merge-id tests)
export CP_repo_url="https://github.com/testorg/testrepo-test27.git"
export JOB_ID="job-310p3"
export BUILDNUMBER="202403"
export CP_timestamp="1027"
export CP_image_proxy="swr.cn-southwest-2.myhuaweicloud.com"
```

Key env vars the converter uses:
- `CP_runs_on`: Determines arch, NPU type, NPU count, CPU, memory, nodeSelector, affinity
- `CP_docker_image`: Container image
- `CP_pipeline_run_id`: Pipeline run identifier
- `CP_merge_id`: Merge request ID
- `CP_repo_url`: Git repo URL (used for generateName derivation and git clone)
- `JOB_ID`, `BUILDNUMBER`: Passed as env vars to the container
- `CP_timestamp`: Used for naming/timestamps
- `CP_image_proxy`: Image proxy registry URL

#### shell.sh (Required)

The user's shell script. Its content becomes the `args` field in the Volcano Job task.

```bash
#!/bin/bash
echo "Running on 310P3 NPU"
echo "Testing specific NPU chip type"
```

#### expected.yaml (Required)

The expected Volcano Job CRD output. **Generate this by running the converter first**, then verify/copy the output.

To generate the expected output:
```bash
cd go/cmd/converter
source case/newtest/test27-310p3/env.sh
go run cmd/converter/convertv2_to_yaml.go -t case/workflow_templatev2.yaml -o ./workflow_trans.yaml
# Then copy workflow_trans.yaml to case/newtest/test27-310p3/expected.yaml
```

Or use the `-regenerate` flag in `Test_main` to auto-write it.

Key fields that vary by NPU type:

| `CP_runs_on` | NPU Resource | Memory/CPU (1 NPU) | nodeSelector | Affinity | Driver Volume |
|---------------|--------------|---------------------|--------------|----------|---------------|
| `arm64` | None | 8Gi/8 | `kubernetes.io/arch: arm64` | None | No |
| `arm64-npu-1` | `huawei.com/ascend-1980` | 48Gi/12 | `kubernetes.io/arch: arm64` | Exclude 310P3 | Yes |
| `arm64-910b4-1` | `huawei.com/ascend-1980` | 48Gi/12 | + `npu.chip.name: 910B4` | None | Yes |
| `arm64-310p3-1` | `huawei.com/ascend-310` | 32Gi/8 | + `npu.chip.name: 310P3` | None | Yes |

Resource values come from `arm310P3ResourceMap` or `arm1980ResourceMap` in `job_resource.go`.

#### eval.sh (Recommended for Volcano Job testing)

Post-deployment validation script run against a real K8s cluster. Follow this template:

```bash
#!/bin/bash

WORKFLOW_NAME="$1"
NAMESPACE="$2"
KUBECONFIG="$3"

failures=0
pass() { echo "PASS: $1"; }
fail() { echo "FAIL: $1"; ((failures++)); }
info() { echo "INFO: $1"; }

echo "==========================================
EVAL: test27-310p3
==========================================

[1/4] Waiting for workflow completion..."
timeout=300
while [ $timeout -gt 0 ]; do
    status=$(kubectl --kubeconfig="$KUBECONFIG" get job.batch.volcano.sh "$WORKFLOW_NAME" -n "$NAMESPACE" -o jsonpath='{.status.status.phase}' 2>/dev/null)
    if [ "$status" = "Completed" ]; then
        pass "Volcano Job status is Completed"
        break
    fi
    if [ "$status" = "Aborted" ] || [ "$status" = "Failed" ]; then
        fail "Volcano Job status is $status"
        break
    fi
    if [ -z "$status" ]; then
        info "Job not found (may have been cleaned up), proceeding with log validation"
        break
    fi
    sleep 10
    timeout=$((timeout - 10))
done

echo "[2/4] Fetching and validating logs..."
pod_name=$(kubectl --kubeconfig="$KUBECONFIG" get pods -n "$NAMESPACE" -l "volcano.sh/job-name=${WORKFLOW_NAME}" --sort-by='.metadata.creationTimestamp' -o jsonpath='{.items[-1].metadata.name}' 2>/dev/null)
if [ -z "$pod_name" ]; then
    info "Pod not found (may have been cleaned up), checking CRD only"
else
    logs=$(kubectl --kubeconfig="$KUBECONFIG" logs "$pod_name" -n "$NAMESPACE" 2>/dev/null)
    echo "$logs" | grep -q "Running on 310P3 NPU" && pass "Shell script output found in logs" || fail "Expected output not found"
fi

echo "[3/4] Fetching and validating workflow CRD..."
crd=$(kubectl --kubeconfig="$KUBECONFIG" get job.batch.volcano.sh "$WORKFLOW_NAME" -n "$NAMESPACE" -o yaml 2>/dev/null)
if [ -n "$crd" ]; then
    echo "$crd" | grep -q 'jobPRID: "110"' && pass "jobPRID label found in CRD" || fail "jobPRID not found"
    echo "$crd" | grep -q "kubernetes.io/arch: arm64" && pass "arm64 nodeSelector found in CRD" || fail "arm64 not found"
    echo "$crd" | grep -q "npu.chip.name: 310P3" && pass "310P3 chip nodeSelector found in CRD" || fail "310P3 chip not found"
else
    info "CRD not available (job cleaned up)"
fi

echo ""
if [ "$failures" -gt 0 ]; then
    echo "==========================================
FAIL: test27-310p3 - ${failures} check(s) failed
=========================================="
    exit 1
fi
echo "==========================================
PASS: test27-310p3 - All validations passed
=========================================="
```

Eval script receives parameters from `test-cases.json` validation.params:
- `$1` = WORKFLOW_NAME (the submitted job name)
- `$2` = NAMESPACE (e.g., "argo")
- `$3` = KUBECONFIG
- `$4` = WORKSPACE (optional, if in params)
- `$5` = CP_ARTIFACTS_TEMP_FOLDER (optional, for copy-pod tests)

#### expected-secret.yaml (Optional, only for secret tests)

Only needed when `wantSecret: true` in the Go test table AND secrets are actually generated. Currently most tests have `wantSecret: false`.

### Step 3: Update convertv2_to_yaml_test.go

Add a new entry to the `tests` slice in `Test_main`:

```go
{
    name:             "test27-310p3",
    testDir:          "case/newtest/test27-310p3",
    wantSecret:       false,
    wantCopyPod:      false,
    dynamicTimestamp: false,
},
```

Fields:
- `name`: Must match the Go subtest name pattern `Test_main/<name>`
- `testDir`: Relative path from `go/cmd/converter/`
- `wantSecret`: Set `true` if `expected-secret.yaml` should be compared
- `wantCopyPod`: Set `true` if `expected-copy-pod.yaml` should be compared
- `dynamicTimestamp`: Set `true` if the expected.yaml has timestamps that change each run (overwrites expected.yaml with generated output each run)

### Step 4: Update test-cases.json

Add an entry to `.opencode/skills/submit-test/test-cases.json`:

```json
"test27-310p3": {
    "test-dir": "go/cmd/converter/case/newtest/test27-310p3",
    "env": "go/cmd/converter/case/newtest/test27-310p3/env.sh",
    "shell-script": "go/cmd/converter/case/newtest/test27-310p3/shell.sh",
    "expected-yaml": "go/cmd/converter/case/newtest/test27-310p3/expected.yaml",
    "expected-secret": "",
    "validation": {
        "script": "eval.sh",
        "params": {
            "WORKFLOW_NAME": "${WORKFLOW_NAME}",
            "NAMESPACE": "argo",
            "KUBECONFIG": "${KUBECONFIG}"
        }
    }
}
```

For tests with secrets or dataset PVC, add extra params:
```json
"expected-secret": "go/cmd/converter/case/newtest/test2-with-secrets/expected-secret.yaml",
"workflow-template-v2": "go/cmd/converter/case/newtest/test14-exit1/workflow_templatev2.yaml",
"validation": {
    "params": {
        "WORKFLOW_NAME": "${WORKFLOW_NAME}",
        "NAMESPACE": "argo",
        "KUBECONFIG": "${KUBECONFIG}",
        "WORKSPACE": "${WORKSPACE}",
        "CP_ARTIFACTS_TEMP_FOLDER": "${CP_ARTIFACTS_TEMP_FOLDER}"
    }
}
```

### Step 5: Run Tests and Verify

1. **Go E2E test**:
```bash
cd go/cmd/converter && go test -v -run Test_main/test27-310p3 ./...
```

2. **Volcano Job test** (requires cluster access):
```bash
skill submit-test -k ~/.kube/karmada-proxy.config -t test27-310p3
```

### Step 6: Commit

Include ALL files in the commit: env.sh, shell.sh, expected.yaml, eval.sh, convertv2_to_yaml_test.go changes, and test-cases.json changes.

## Common Mistakes

1. **Forgetting to update expected.yaml when resource maps change**: If you change `arm310P3ResourceMap` or `arm1980ResourceMap`, ALL expected.yaml files using those values must be updated too. Search: `grep -r "48Gi" case/newtest/` for memory values, `grep -r "cpu: \"12\"" case/newtest/` for CPU values.

2. **Wrong NPU resource name**: 310P3 uses `huawei.com/ascend-310`, everything else uses `huawei.com/ascend-1980`.

3. **Missing eval.sh**: Without eval.sh, the Volcano Job test has no validation and just marks PASS regardless of actual behavior.

4. **expected.yaml indentation errors**: YAML indentation must be exact. Use the `-regenerate` flag or `Write` tool to generate the file, never edit resource values by hand with `Edit` tool (it breaks indentation).

5. **eval.sh using wrong pod label**: Volcano Jobs use `volcano.sh/job-name` label, NOT `workflows.argoproj.io/workflow` (that's for Argo Workflows). Always use:
   ```bash
   kubectl get pods -l "volcano.sh/job-name=${WORKFLOW_NAME}"
   ```

6. **eval.sh always returning PASS**: The eval script must track failures and exit non-zero. Use a `failures` counter and `exit 1` if any checks fail. When logs are unavailable (pod cleaned up), use INFO not FAIL for log-based checks. Reference `test33-goproxy/eval.sh` for the correct pattern.

7. **Regenerating expected.yaml after template changes**: If you change `workflow_templatev2.yaml`, ALL expected.yaml files must be regenerated. Use the `-regenerate` flag:
   ```bash
   cd go/cmd/converter && go test -v -run Test_main -regenerate .
   ```
   This overwrites all expected.yaml files with the converter's current output.

## Dataset PVC Rules

When creating test cases that use `CP_dataset`, the `CP_repo_url` **must** resolve to an existing PVC claim name. The PVC claim name is derived from `CP_repo_url` via `DatasetManager.GetClaimName()` in `go/cmd/converter/package/dataset_manager.go`.

### Existing PVCs on the cluster:

| PVC Claim Name | Notes |
|---|---|
| `testorg-testrepo-test15` | General-purpose dataset PVC |
| `ascend-op-plugin` | Ascend operator plugin PVC |
| `ascend-ragsdk` | Ascend RAG SDK PVC |
| `ascend-ascendnpu-ir` | Ascend NPU IR PVC |

### Repo URL → PVC Claim Name Mapping

The `DatasetManager` hardcoded mapping (`dataset_manager.go:7-17`):

| `CP_repo_url` repo name | Resolved PVC Claim Name |
|---|---|
| `testorg-testrepo-test15.git` | `testorg-testrepo-test15` |
| `testorg-testrepo-test16.git` | `testorg-testrepo-test15` |
| `testorg-testrepo-test21.git` | `testorg-testrepo-test15` |
| `ascend-op-plugin.git` | `ascend-op-plugin` |
| `ascend-pytorch.git` | `ascend-op-plugin` |
| `ascend-text-embeddings-inference.git` | `ascend-ragsdk` |
| `ascend-ragsdk.git` | `ascend-ragsdk` |
| `npu-ir-cicd.git` | `ascend-ascendnpu-ir` |
| `ascend-ascendnpu-ir.git` | `ascend-ascendnpu-ir` |
| Any other repo | Falls back to the repo name itself (must exist as PVC) |

**Rule**: When adding a dataset test, pick one of the above repo URLs. **Do not invent a new repo name** unless you also create the corresponding PVC on the cluster.

For read-only dataset tests, use `CP_dataset` with comma-separated key:value syntax:
```bash
# Read-only mount (shorthand)
export CP_dataset="/dataset,readonly"
```

## Checklist

- [ ] Create test directory with env.sh, shell.sh
- [ ] Generate expected.yaml by running converter
- [ ] Create eval.sh for Volcano Job validation
- [ ] Add entry to convertv2_to_yaml_test.go tests slice
- [ ] Add entry to test-cases.json
- [ ] Run Go E2E test and verify PASS
- [ ] Run Volcano Job test and verify PASS
- [ ] Commit all files together