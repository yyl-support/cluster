#!/bin/bash
# Eval script for test32-no-artifact-files
# Tests: cp-artifact configured but no matching files produced

WORKFLOW_NAME="$1"
NAMESPACE="$2"
KUBECONFIG="$3"
WORKSPACE="${4:-.}"
WORKFLOW_EXIT="${6:-0}"

if [[ "$WORKFLOW_EXIT" -eq 1 ]]; then
    echo "PASS: Workflow execution failed as expected (exit code 1)"
else
    echo "FAIL: Expected workflow exit code 1 (cp no matching files), got ${WORKFLOW_EXIT}"
    exit 1
fi

echo "=========================================="
echo "EVAL: test32-no-artifact-files"
echo "=========================================="

echo ""
echo "[1/4] Fetching Volcano Job CRD..."

workflow_crd=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml)

if echo "$workflow_crd" | grep -q "jobPRID: \"113\""; then
    echo "PASS: jobPRID label '113' found in CRD"
else
    echo "FAIL: jobPRID label '113' NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "pipeline/run-id: test-no-artifact-files-123"; then
    echo "PASS: pipeline/run-id label found in CRD"
else
    echo "FAIL: pipeline/run-id label NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "name: copy-artifact"; then
    echo "PASS: copy-artifact task found"
else
    echo "FAIL: copy-artifact task NOT found"
    exit 1
fi

if echo "$workflow_crd" | grep -q "name: output"; then
    echo "PASS: volume name 'output' found"
else
    echo "FAIL: volume name NOT found"
    exit 1
fi

echo ""
echo "[2/4] Verifying workflow aborted..."

max_wait=120
interval=10
elapsed=0

while [ $elapsed -lt $max_wait ]; do
    job_status=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o jsonpath='{.status.state.phase}' 2>&1)

    if [[ "${job_status}" == "Aborted" ]] || [[ "${job_status}" == "Failed" ]]; then
        echo "PASS: Workflow ${WORKFLOW_NAME} status: ${job_status} (expected)"
        break
    fi

    if [[ "${job_status}" == "Completed" ]]; then
        echo "FAIL: Workflow should have been aborted, got Completed"
        exit 1
    fi

    echo "  Status: ${job_status} (${elapsed}s elapsed)"
    sleep $interval
    ((elapsed += interval))
done

if [[ "${job_status}" != "Aborted" ]] && [[ "${job_status}" != "Failed" ]]; then
    echo "FAIL: Workflow ${WORKFLOW_NAME} timed out after ${max_wait}s"
    exit 1
fi

echo ""
echo "[3/4] Validating logs..."

pod_name=$(kubectl get pods -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -l volcano.sh/job-name=${WORKFLOW_NAME} -o jsonpath='{.items[0].metadata.name}' 2>&1)
workflow_logs=$(kubectl logs "$pod_name" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" 2>&1)

if echo "$workflow_logs" | grep -q "Running without artifact files..."; then
    echo "PASS: 'Running without artifact files...' found in logs"
else
    echo "FAIL: 'Running without artifact files...' NOT found in logs"
    exit 1
fi

if echo "$workflow_logs" | grep -q "No files match artifact pattern"; then
    echo "PASS: 'No files match artifact pattern' found in logs"
else
    echo "FAIL: 'No files match artifact pattern' NOT found in logs"
    exit 1
fi

echo ""
echo "[4/4] Verifying no artifacts extracted..."

if [ ! -d "${WORKSPACE}" ] || [ -z "$(ls -A "${WORKSPACE}" 2>/dev/null)" ]; then
    echo "PASS: No artifacts extracted (as expected)"
else
    echo "FAIL: Artifact files found but should be none"
    exit 1
fi

echo ""
echo "=========================================="
echo "PASS: test32-no-artifact-files - All validations passed"
echo "=========================================="
exit 0
