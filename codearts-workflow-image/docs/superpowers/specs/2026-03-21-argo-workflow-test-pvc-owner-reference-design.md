# Argo Workflow Test - Framework Refactor + PVC OwnerReference Fix

**Date:** 2026-03-21  
**Status:** Draft  
**Type:** Bug Fix + Framework Refactor

## Problem Statement

1. **PVC OwnerReference flow is broken** - current approach uses `kubectl patch` after workflow submission, which is fragile
2. **Framework has dead/duplicate code** - `workflow-submitter.sh` and `result-validator.sh` are unused, logging functions duplicated across files, unclear module boundaries

## Solution Overview

1. **Fix PVC flow** - Submit → Get UID → Apply PVC (with ownerRef) → Wait → Conditional copy → Delete PVC → Eval
2. **Restructure to 4 clean files** - main.sh + lib/utils.sh + lib/argo.sh + lib/test-cases.sh
3. **Use local variables** - prevent global variable pollution, use local vars in functions

## New Flow (per test case)

1. **Run `go test`** - if pass, continue; if fail, exit entire test suite
2. **Parse test cases** from JSON config (respects `-t` selection)
3. **For each selected test case (async unit):**
   - Apply secret.yaml if exists
   - `argo submit` → get workflow name immediately (workflow PENDs)
   - Get UID via `kubectl get workflow -o jsonpath='{.metadata.uid}'`
   - Apply PVC with ownerRef directly (using UID)
   - Wait for workflow completion (with timeout)
   - **If workflow failed** → delete PVC → mark test as failed
   - **If workflow succeeded** → 
     - Start copy pod to copy artifacts from PVC to `$WORKSPACE$CP_ARTIFACTS_TEMP_FOLDER`
     - Wait for copy pod to complete
     - Delete PVC
   - Run eval script
4. **Collect all results** - wait for async jobs
5. **Report final pass/fail**

## Copy Pod Logic

The copy pod is defined in `expected-copy-pod.yaml` in each test case directory. It:
1. Mounts the PVC at `/output/artifact`
2. Copies artifacts to `$WORKSPACE$CP_ARTIFACTS_TEMP_FOLDER`
3. Uses local variables in all functions to prevent global variable pollution

### Copy Pod Variables (from test case env.sh)

| Variable | Description |
|----------|-------------|
| `CP_artifacts_temp_folder` | Mount path inside pod (e.g., `/output/artifact`) |
| `WORKSPACE` | Local test directory |
| `CP_ARTIFACTS_TEMP_FOLDER` | Relative path for copied artifacts |

## Target Structure (4 files)

```
scripts/
├── main.sh           # Orchestrator: parse args, run go test, dispatch async
└── lib/
    ├── utils.sh      # Logging, colors, common helpers
    ├── argo.sh       # Argo operations (submit, wait, PVC, copy, eval)
    └── test-cases.sh # Test case parsing
```

## CLI Interface

```
-t all                           # Run all tests (go test all + parse all)
-t test1-simple                  # Run specific test
-t "test1-simple,test2-secrets"  # Run specific subset
-k <kubeconfig>                  # Path to kubeconfig (required)
--list-tests                     # List available tests
-h, --help                       # Show help
```

## File Responsibilities

### main.sh
- Parse CLI args (-k, -t, --list-tests, -h)
- Run `go test` based on selection (all or specific tests)
- Parse test cases
- Dispatch each test case as async job
- Collect and report results

### lib/utils.sh
- Colors: RED, GREEN, YELLOW, BLUE, CYAN, MAGENTA, BOLD, NC
- Log functions: log_info, log_success, log_error, log_step, log_test, log_eval
- Common helpers: get_timestamp, validate_kubeconfig
- All variables use `local` keyword to prevent global pollution

### lib/argo.sh
- `argo_submit(workflow_file, namespace, kubeconfig)` → workflow_name
- `argo_get_uid(workflow_name, namespace, kubeconfig)` → uid
- `argo_wait(workflow_name, namespace, kubeconfig, timeout)` → status
- `argo_delete(workflow_name, namespace, kubeconfig)`
- `apply_secret(secret_file, namespace, kubeconfig)`
- `apply_pvc(pvc_file, uid, namespace, kubeconfig, workflow_name)` - adds ownerRef with UID
- `start_copy_pod(copy_pod_file, namespace, kubeconfig)` → pod_name
- `wait_copy_pod(pod_name, namespace, kubeconfig, timeout)` → status
- `delete_pvc(pvc_name, namespace, kubeconfig)`
- `run_eval(eval_script, workflow_name, namespace, kubeconfig, ...)`
- All variables use `local` keyword to prevent global pollution

### lib/test-cases.sh
- `parse_test_cases()` - parse from JSON
- `get_test_case_info(test_name, field)` → value
- `select_tests(selection)` → list of test names
- `list_tests()` - display available tests

## Wait Loop Implementation

```bash
argo_wait() {
    local workflow_name="$1"
    local namespace="$2"
    local kubeconfig="$3"
    local max_wait="${4:-300}"
    local interval=10
    local elapsed=0

    local status
    status=$(argo get "$workflow_name" -n "$namespace" \
        --kubeconfig "$kubeconfig" 2>/dev/null | \
        grep Status | awk '{print $2}')

    while [[ "$status" == "Running" || "$status" == "Pending" ]]; do
        sleep $interval
        elapsed=$((elapsed + interval))
        if [ $elapsed -ge $max_wait ]; then
            return 1  # timeout
        fi
        status=$(argo get "$workflow_name" -n "$namespace" \
            --kubeconfig "$kubeconfig" 2>/dev/null | \
            grep Status | awk '{print $2}')
    done

    echo "$status"
}
```

## Error Handling

| Step | On Failure |
|------|------------|
| `go test` fails | Exit entire test suite |
| `argo submit` fails | Fail test case |
| UID retrieval fails | Fail test case |
| PVC apply fails | Fail test case |
| Workflow timeout | Delete PVC, fail test case |
| Workflow failed | Delete PVC, fail test case |
| Copy pod fails | Delete PVC, fail test case |
| Eval fails | Fail test case |
| PVC delete fails | Log warning, continue |

## Files Removed

- `workflow-submitter.sh` - dead code, functionality merged into lib/argo.sh
- `result-validator.sh` - dead code, not used

## Backward Compatibility

- CLI interface unchanged
- Test case JSON structure unchanged
- Eval script interface unchanged
- -k, -t, --list-tests, -h flags preserved
