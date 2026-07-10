#!/bin/bash
# Eval script for test6-empty-sensitive-value
# Params: WORKFLOW_NAME NAMESPACE KUBECONFIG [WORKSPACE] [CP_ARTIFACTS_TEMP_FOLDER] [WORKFLOW_EXIT]

WORKFLOW_NAME="$1"
NAMESPACE="$2"
KUBECONFIG="$3"
WORKSPACE="${4:-.}"
CP_ARTIFACTS_TEMP_FOLDER="${5:-}"
WORKFLOW_EXIT="${6:-0}"

if [[ "$WORKFLOW_EXIT" -ne 0 ]]; then
    echo "FAIL: submit exited with code ${WORKFLOW_EXIT}"
    exit 1
fi

echo "=========================================="
echo "EVAL: test6-empty-sensitive-value"
echo "=========================================="

TEST_DIR="${WORKSPACE}"

max_wait=120
interval=10
elapsed=0

echo ""
echo "[1/4] Waiting for workflow completion..."

while [ $elapsed -lt $max_wait ]; do
    job_status=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o jsonpath='{.status.state.phase}' 2>&1)
    
    if [[ "${job_status}" == "Completed" ]]; then
        break
    fi
    
    if [[ "${job_status}" == "Failed" ]] || [[ "${job_status}" == "Error" ]]; then
        echo "FAIL: Workflow ${WORKFLOW_NAME} status: ${job_status}"
        exit 1
    fi
    
    echo "  Status: ${job_status} (${elapsed}s elapsed)"
    sleep $interval
    ((elapsed += interval))
done

if [[ "${job_status}" != "Completed" ]]; then
    echo "FAIL: Workflow ${WORKFLOW_NAME} timed out after ${max_wait}s"
    exit 1
fi
echo "PASS: Volcano Job status is Completed"

echo ""
echo "[2/4] Fetching and validating logs..."

pod_name=$(kubectl get pods -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -l volcano.sh/job-name=${WORKFLOW_NAME} -o jsonpath='{.items[0].metadata.name}' 2>&1)
workflow_logs=$(kubectl logs "$pod_name" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" 2>&1)

if echo "$workflow_logs" | grep -q "Testing empty sensitive value"; then
    echo "PASS: Empty sensitive value test message found in logs"
else
    echo "FAIL: Empty sensitive value test message NOT found in logs"
    echo "Logs:"
    echo "$workflow_logs"
    exit 1
fi

if echo "$workflow_logs" | grep -q "GitCode token:"; then
    echo "PASS: GitCode token message found in logs"
else
    echo "FAIL: GitCode token message NOT found in logs"
    exit 1
fi

echo "Logs fetched successfully"

echo ""
echo "[3/4] Fetching and validating workflow CRD..."

workflow_crd=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml)

if echo "$workflow_crd" | grep -q "jobRepositoryName: testorg-testrepo-test6"; then
    echo "PASS: jobRepositoryName label found in CRD"
else
    echo "FAIL: jobRepositoryName label NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "jobPRID: \"19\""; then
    echo "PASS: jobPRID label found in CRD"
else
    echo "FAIL: jobPRID label NOT found in CRD"
    exit 1
fi

echo "CRD validated successfully"

echo ""
echo "[4/4] Verifying no secret was created (empty sensitive value filtered)..."

secret_name="gitcode-token-test-empty-789"
if kubectl get secret "$secret_name" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" 2>&1 | grep -q "NotFound"; then
    echo "PASS: No secret created for empty sensitive value (correct behavior)"
else
    echo "FAIL: Secret was created but should not have been for empty value"
    exit 1
fi

echo "Empty sensitive value handling verified"

echo ""
echo "=========================================="
echo "PASS: test6-empty-sensitive-value - All validations passed"
echo "=========================================="
exit 0
