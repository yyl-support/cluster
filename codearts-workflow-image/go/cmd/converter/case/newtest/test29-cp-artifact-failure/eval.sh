#!/bin/bash
# Eval script for test29-cp-artifact-failure
# Tests: Main container fails (exit 1) with copy-artifact container
# Expected behavior: Pod NOT deleted, NO artifact extraction, logs accessible

WORKFLOW_NAME="$1"
NAMESPACE="$2"
KUBECONFIG="$3"
WORKSPACE="${4:-.}"
CP_ARTIFACTS_TEMP_FOLDER="${5:-}"
WORKFLOW_EXIT="${6:-0}"

echo "=========================================="
echo "EVAL: test29-cp-artifact-failure"
echo "=========================================="

# Expected: submit exits with code 1 (main container failed)
if [[ "$WORKFLOW_EXIT" -eq 1 ]]; then
    echo "PASS: Submit exited with code 1 (expected for main container failure)"
else
    echo "FAIL: Expected submit exit code 1, got ${WORKFLOW_EXIT}"
    exit 1
fi

echo ""
echo "[1/4] Waiting for workflow completion..."

max_wait=120
interval=10
elapsed=0

while [ $elapsed -lt $max_wait ]; do
    job_status=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o jsonpath='{.status.state.phase}' 2>&1)
    
    if [[ "${job_status}" == "Failed" ]] || [[ "${job_status}" == "Aborted" ]]; then
        echo "PASS: Workflow ${WORKFLOW_NAME} status: ${job_status} (expected)"
        break
    fi
    
    if [[ "${job_status}" == "Completed" ]]; then
        echo "FAIL: Workflow ${WORKFLOW_NAME} succeeded but should fail with exit 1"
        exit 1
    fi
    
    echo "  Status: ${job_status} (${elapsed}s elapsed)"
    sleep $interval
    ((elapsed += interval))
done

if [[ "${job_status}" != "Failed" ]] && [[ "${job_status}" != "Aborted" ]]; then
    echo "FAIL: Workflow ${WORKFLOW_NAME} timed out after ${max_wait}s"
    exit 1
fi

echo ""
echo "[2/4] Verifying pod exists (NOT deleted)..."

pod_name=$(kubectl get pods -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -l volcano.sh/job-name=${WORKFLOW_NAME} -o jsonpath='{.items[0].metadata.name}' 2>&1)

if [[ -z "$pod_name" ]] || echo "$pod_name" | grep -q "NotFound"; then
    echo "FAIL: Pod was deleted (should NOT be deleted on normal exit code failures)"
    echo "Expected behavior: Pod should stay alive for eval to fetch logs"
    exit 1
else
    echo "PASS: Pod ${pod_name} exists (NOT deleted)"
fi

echo ""
echo "[3/4] Fetching and validating logs..."

workflow_logs=$(kubectl logs "$pod_name" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -c ascend 2>&1)

if echo "$workflow_logs" | grep -q "Starting build with artifacts"; then
    echo "PASS: Shell script output 'Starting build with artifacts' found in logs"
else
    echo "FAIL: Shell script output 'Starting build with artifacts' NOT found in logs"
    echo "Logs:"
    echo "$workflow_logs"
    exit 1
fi

if echo "$workflow_logs" | grep -q "Build failed intentionally"; then
    echo "PASS: Shell script output 'Build failed intentionally' found in logs"
else
    echo "FAIL: Shell script output 'Build failed intentionally' NOT found in logs"
    echo "Logs:"
    echo "$workflow_logs"
    exit 1
fi

echo "Logs fetched successfully"

echo ""
echo "[4/4] Verifying NO artifact extraction..."

artifact_dest="${WORKSPACE}"

if [ -f "${artifact_dest}/test.txt" ]; then
    echo "FAIL: Artifact file test.txt found at ${artifact_dest}/test.txt (should NOT be extracted on failure)"
    exit 1
else
    echo "PASS: Artifact file test.txt NOT found (expected: NO extraction on failure)"
fi

if [ -f "${artifact_dest}/debug.log" ]; then
    echo "FAIL: Artifact file debug.log found at ${artifact_dest}/debug.log (should NOT be extracted on failure)"
    exit 1
else
    echo "PASS: Artifact file debug.log NOT found (expected: NO extraction on failure)"
fi

echo ""
echo "=========================================="
echo "PASS: test29-cp-artifact-failure - All validations passed"
echo "=========================================="
exit 0