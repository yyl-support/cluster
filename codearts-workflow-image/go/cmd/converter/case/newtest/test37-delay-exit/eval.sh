#!/bin/bash
# Eval script for test37-delay-exit
# Params: WORKFLOW_NAME NAMESPACE KUBECONFIG [WORKSPACE] [CP_ARTIFACTS_TEMP_FOLDER] [WORKFLOW_EXIT]

WORKFLOW_NAME="$1"
NAMESPACE="$2"
KUBECONFIG="$3"
WORKFLOW_EXIT="${6:-0}"

failures=0
pass() { echo "PASS: $1"; }
fail() { echo "FAIL: $1"; ((failures++)); }
info() { echo "INFO: $1"; }

echo "=========================================="
echo "EVAL: test37-delay-exit"
echo "=========================================="

# shell.sh exits 1 on purpose, so we expect the job to fail.
if [[ "$WORKFLOW_EXIT" -eq 1 ]]; then
    pass "Workflow execution failed as expected (exit code 1)"
else
    fail "Expected workflow exit code 1, got ${WORKFLOW_EXIT}"
fi

echo ""
echo "[1/3] Waiting for workflow completion..."
max_wait=120
interval=10
elapsed=0
job_status=""
while [ $elapsed -lt $max_wait ]; do
    job_status=$(kubectl get job.batch.volcano.sh "${WORKFLOW_NAME}" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o jsonpath='{.status.state.phase}' 2>&1)

    if [[ "${job_status}" == "Failed" ]] || [[ "${job_status}" == "Error" ]] || [[ "${job_status}" == "Aborted" ]]; then
        pass "Workflow ${WORKFLOW_NAME} status: ${job_status} (expected failure)"
        break
    fi
    if [[ "${job_status}" == "Completed" ]]; then
        fail "Workflow ${WORKFLOW_NAME} succeeded but should fail (shell.sh exits 1)"
        break
    fi

    sleep $interval
    ((elapsed += interval))
done

if [[ "${job_status}" != "Failed" ]] && [[ "${job_status}" != "Error" ]] && [[ "${job_status}" != "Aborted" ]] && [[ "${job_status}" != "Completed" ]]; then
    fail "Workflow ${WORKFLOW_NAME} timed out after ${max_wait}s (last status: ${job_status})"
fi

echo ""
echo "[2/3] Fetching and validating logs..."
pod_name=$(kubectl get pods -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -l "volcano.sh/job-name=${WORKFLOW_NAME}" --sort-by='.metadata.creationTimestamp' -o jsonpath='{.items[-1].metadata.name}' 2>/dev/null)
if [ -z "$pod_name" ]; then
    info "Pod not found (may have been cleaned up), checking CRD only"
else
    logs=$(kubectl logs "$pod_name" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" 2>/dev/null)
    echo "$logs" | grep -q "delay exit test" && pass "Shell script output found in logs" || fail "Expected output 'delay exit test' NOT found in logs"
fi

echo ""
echo "[3/3] Fetching and validating workflow CRD..."
workflow_crd=$(kubectl get job.batch.volcano.sh "${WORKFLOW_NAME}" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml 2>/dev/null)
if [ -n "$workflow_crd" ]; then
    echo "$workflow_crd" | grep -q "sleep 20" && pass "delay exit trap with sleep 20 found in CRD args" || fail "delay exit trap with sleep 20 NOT found in CRD args"
    echo "$workflow_crd" | grep -q 'pipeline/run-id: test-delay-exit-123' && pass "pipeline/run-id found" || fail "pipeline/run-id NOT found"
else
    info "CRD not available (job cleaned up)"
fi

echo ""
if [ "$failures" -gt 0 ]; then
    echo "=========================================="
    echo "FAIL: test37-delay-exit - ${failures} check(s) failed"
    echo "=========================================="
    exit 1
fi
echo "=========================================="
echo "PASS: test37-delay-exit - All validations passed"
echo "=========================================="
