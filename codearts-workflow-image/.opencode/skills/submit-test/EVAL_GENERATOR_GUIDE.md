# Eval Generator Guide

This guide explains how to create comprehensive eval scripts that validate Argo workflow execution.

## Philosophy: Point-to-General

Eval scripts should **prove the workflow worked correctly** by demonstrating:
1. The process completed successfully
2. All expected behaviors occurred (secrets mounted, resources allocated, artifacts produced)
3. Cleanup happened as expected

## Universal Validation Steps

Every eval script must perform these checks:

### 1. Status Check (Required)
```bash
workflow_status=$(argo get ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" 2>&1 | grep Status | awk '{print $2}')
if [[ "${workflow_status}" != "Succeeded" ]]; then
    echo "FAIL: Workflow status is ${workflow_status}"
    exit 1
fi
echo "PASS: Workflow status is Succeeded"
```

### 2. Log Validation (Required)
Fetch and validate logs to prove the main script ran correctly.

```bash
echo "Fetching workflow logs..."
workflow_logs=$(argo logs ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" 2>&1)

if echo "$workflow_logs" | grep -q "EXPECTED_OUTPUT_FROM_SHELL_SCRIPT"; then
    echo "PASS: Shell script output found in logs"
else
    echo "FAIL: Shell script output NOT found in logs"
    echo "Logs:"
    echo "$workflow_logs"
    exit 1
fi
```

### 3. CRD Validation (Required)
Get the workflow CRD from K8s and validate its spec matches expected values.

```bash
echo "Fetching workflow CRD..."
workflow_crd=$(kubectl get workflow ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml)

# Validate resource requests/limits
if echo "$workflow_crd" | grep -q "cpu: \"EXPECTED_CPU\""; then
    echo "PASS: CPU resources match"
else
    echo "FAIL: CPU resources do not match"
    exit 1
fi
```

## Test-Specific Validations

### For Tests WITH Secrets

Secrets tests must validate:
1. **Secret was created** - Check K8s for the secret existence during workflow run
2. **Secret values appeared in logs** - This proves the secret was mounted and accessible
3. **Secret was deleted** - Via `onExit: cleanup-secret` mechanism

```bash
# Get the secret name from expected-secret.yaml
secret_name=$(grep "^  name:" "${TEST_DIR}/expected-secret.yaml" | awk '{print $2}')

# Check if secret exists (it should during run, may be gone after)
# The secret should have been deleted by the cleanup-secret template

# Validate secret values in logs
if echo "$workflow_logs" | grep -q "ACCESS_KEY.*secret-access-key"; then
    echo "PASS: Secret values appeared in logs"
else
    echo "FAIL: Secret values NOT found in logs"
    exit 1
fi
```

### For Tests WITH Custom Resources

Validate the workflow CRD has the correct resource specifications.

```bash
# For custom CPU/memory
if echo "$workflow_crd" | grep -q "cpu: \"16\"" && echo "$workflow_crd" | grep -q "memory: 32Gi"; then
    echo "PASS: Custom resources match"
else
    echo "FAIL: Custom resources do not match"
    exit 1
fi

# For NPU resources (910B4)
if echo "$workflow_crd" | grep -q "huawei.com/ascend-1980"; then
    echo "PASS: NPU resources found"
else
    echo "FAIL: NPU resources not found"
    exit 1
fi
```

### For Tests WITH Artifacts (CP_artifacts_temp_folder)

1. **Workflow has artifact PVC mounted** - Check volumeMounts in CRD
2. **Artifacts were produced** - Verify files exist in the copied location
3. **PVC was cleaned up** - Double-check PVC deletion (explicit + ownerReferences)

```bash
# Check workflow CRD has artifact volumeMount
if echo "$workflow_crd" | grep -q "mountPath: /output/artifact"; then
    echo "PASS: Artifact volumeMount found in CRD"
else
    echo "FAIL: Artifact volumeMount NOT found in CRD"
    exit 1
fi

# Check PVC was cleaned up (PVC should be gone after workflow completion + delete)
# Note: We delete workflow first, then check PVC
pvc_name=$(grep "^  name:" "${TEST_DIR}/expected-artifact-pvc.yaml" | awk '{print $2}')

sleep 5  # Allow time for ownerReferences cleanup
if kubectl get pvc "$pvc_name" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" 2>&1 | grep -q "NotFound"; then
    echo "PASS: PVC was cleaned up"
else
    echo "FAIL: PVC still exists after cleanup"
    kubectl get pvc "$pvc_name" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG"
    exit 1
fi
```

### For Tests WITH Custom Image

Validate the workflow CRD uses the correct Docker image.

```bash
if echo "$workflow_crd" | grep -q "image: EXPECTED_IMAGE"; then
    echo "PASS: Custom image found in CRD"
else
    echo "FAIL: Custom image NOT found in CRD"
    exit 1
fi
```

### For Tests WITHOUT Merge ID

Validate the workflow CRD does NOT have `jobPRID` label.

```bash
if echo "$workflow_crd" | grep -q "jobPRID"; then
    echo "FAIL: jobPRID label found but should not exist"
    exit 1
else
    echo "PASS: jobPRID label correctly absent"
fi
```

## Eval Script Template

```bash
#!/bin/bash
# Eval script for TEST_NAME
# Params: WORKFLOW_NAME NAMESPACE KUBECONFIG [WORKSPACE] [CP_ARTIFACTS_TEMP_FOLDER]

WORKFLOW_NAME="$1"
NAMESPACE="$2"
KUBECONFIG="$3"
WORKSPACE="${4:-.}"
CP_ARTIFACTS_TEMP_FOLDER="${5:-}"

echo "=========================================="
echo "EVAL: TEST_NAME"
echo "=========================================="

# Configuration
TEST_DIR="${WORKSPACE}"  # This should be passed as WORKSPACE param

# Step 1: Wait for workflow completion
echo ""
echo "[1/5] Waiting for workflow completion..."

max_wait=120
interval=10
elapsed=0

while [ $elapsed -lt $max_wait ]; do
    workflow_status=$(argo get ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" 2>&1 | grep Status | awk '{print $2}')
    
    if [[ "${workflow_status}" == "Succeeded" ]]; then
        break
    fi
    
    if [[ "${workflow_status}" == "Failed" ]] || [[ "${workflow_status}" == "Error" ]]; then
        echo "FAIL: Workflow ${WORKFLOW_NAME} status: ${workflow_status}"
        exit 1
    fi
    
    echo "  Status: ${workflow_status} (${elapsed}s elapsed)"
    sleep $interval
    ((elapsed += interval))
done

if [[ "${workflow_status}" != "Succeeded" ]]; then
    echo "FAIL: Workflow ${WORKFLOW_NAME} timed out after ${max_wait}s"
    exit 1
fi
echo "PASS: Workflow status is Succeeded"

# Step 2: Fetch and validate logs
echo ""
echo "[2/5] Fetching and validating logs..."

workflow_logs=$(argo logs ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" 2>&1)

# INSERT LOG VALIDATION CHECKS HERE
# Example: if echo "$workflow_logs" | grep -q "EXPECTED_OUTPUT"; then echo "PASS"; else echo "FAIL"; exit 1; fi

echo "Logs fetched successfully"

# Step 3: Fetch and validate workflow CRD
echo ""
echo "[3/5] Fetching and validating workflow CRD..."

workflow_crd=$(kubectl get workflow ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml)

# INSERT CRD VALIDATION CHECKS HERE
# Example: if echo "$workflow_crd" | grep -q "cpu: \"16\""; then echo "PASS"; else echo "FAIL"; exit 1; fi

echo "CRD validated successfully"

# Step 4: Test-specific validations (secrets, artifacts, etc.)
echo ""
echo "[4/5] Running test-specific validations..."

# INSERT TEST-SPECIFIC CHECKS HERE

echo "Test-specific validations passed"

# Step 5: Cleanup verification (if applicable)
echo ""
echo "[5/5] Verifying cleanup..."

# INSERT CLEANUP VERIFICATION HERE

echo "Cleanup verified"

echo ""
echo "=========================================="
echo "PASS: TEST_NAME - All validations passed"
echo "=========================================="
exit 0
```

## Validation Checklist by Test Type

| Test Type | Status | Logs | CRD | Secret | Artifacts | Cleanup |
|-----------|--------|------|-----|--------|-----------|---------|
| Simple | ✓ | ✓ | ✓ | - | - | - |
| With Secrets | ✓ | ✓ | ✓ | ✓ | - | ✓ |
| Custom Resources | ✓ | ✓ | ✓ | - | - | - |
| Custom Image | ✓ | ✓ | ✓ | - | - | - |
| No Merge ID | ✓ | ✓ | ✓ | - | - | - |
| Empty Sensitive | ✓ | ✓ | ✓ | - | - | - |
| Workspace Filtered | ✓ | ✓ | ✓ | - | - | - |
| Git Clone | ✓ | ✓ | ✓ | - | - | - |
| 910B4 (NPU) | ✓ | ✓ | ✓ | - | - | - |
| CP Artifacts | ✓ | ✓ | ✓ | - | ✓ | ✓ |

## How to Generate Eval Scripts

1. **Start with the template** above
2. **Set TEST_NAME** to the actual test case name
3. **Configure max_wait**:
   - Simple tests: 120s
   - Tests with secrets or artifacts: 300s
4. **Add LOG VALIDATION** based on shell.sh output:
   - For `echo "message"`: Check for `message` in logs
   - For `ls -la`: Check for file listings
   - For secrets: Check for secret values in logs
5. **Add CRD VALIDATION** based on expected.yaml:
   - CPU/memory values
   - Image name
   - Volume mounts
   - Node selectors
6. **Add TEST-SPECIFIC checks**:
   - Secrets: Check secret deletion via onExit
   - Artifacts: Check file copy and PVC cleanup
   - NPU: Check `huawei.com/ascend-1980` resource
7. **Add CLEANUP verification** if applicable

## Important Notes

1. **DO NOT apply YAML files in eval** - Eval is ONLY for validation
2. **Logs must prove behavior** - Not just "workflow ran" but "specific things happened"
3. **CRD validation ensures K8s resources match expectations** - This catches configuration errors
4. **Secret cleanup is automatic** - via `onExit: cleanup-secret` template, but we verify it happened
5. **PVC cleanup requires double-delete** - explicit delete + ownerReferences cleanup

## Example: with-secrets Eval Structure

```bash
# Log validation: Secret values should appear
if echo "$workflow_logs" | grep -q "Using API token:.*secret-api-token"; then
    echo "PASS: API_TOKEN secret value found in logs"
else
    echo "FAIL: API_TOKEN secret value NOT in logs"
    exit 1
fi

# CRD validation: Verify secretKeyRef exists
if echo "$workflow_crd" | grep -q "secretKeyRef"; then
    echo "PASS: secretKeyRef found in CRD"
else
    echo "FAIL: secretKeyRef NOT in CRD"
    exit 1
fi

# Secret cleanup: After workflow completes, secret should be deleted
# Note: The onExit cleanup-secret template handles this
```

## Example: cp-artifacts Eval Structure

```bash
# CRD validation: Check artifact volumeMount
if echo "$workflow_crd" | grep -q "mountPath: /output/artifact"; then
    echo "PASS: Artifact volumeMount found"
else
    echo "FAIL: Artifact volumeMount NOT found"
    exit 1
fi

# Artifact validation: Files should be copied to local workspace
if [ -f "${WORKSPACE}/test.txt" ]; then
    echo "PASS: Artifact file test.txt found"
else
    echo "FAIL: Artifact file test.txt NOT found"
    exit 1
fi

# PVC cleanup: PVC should be deleted
pvc_name="pipeline-artifact-test-cp-artifacts-123-dd1f27a6e7052317"
if kubectl get pvc "$pvc_name" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" 2>&1 | grep -q "NotFound"; then
    echo "PASS: PVC was cleaned up"
else
    echo "FAIL: PVC still exists"
    exit 1
fi
```
