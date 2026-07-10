#!/bin/bash
# Eval script for test5-no-merge-id
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
echo "EVAL: test5-no-merge-id"
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

if echo "$workflow_logs" | grep -q "Testing without merge id"; then
    echo "PASS: No-merge-id test message found in logs"
else
    echo "FAIL: No-merge-id test message NOT found in logs"
    echo "Logs:"
    echo "$workflow_logs"
    exit 1
fi

if echo "$workflow_logs" | grep -qE "total [0-9]+$|drwx"; then
    echo "PASS: ls -la output found in logs"
else
    echo "FAIL: ls -la output NOT found in logs"
    exit 1
fi

echo "Logs fetched successfully"

echo ""
echo "[3/4] Fetching and validating workflow CRD..."

workflow_crd=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml)

if echo "$workflow_crd" | grep -q "jobRepositoryName: testorg-testrepo-test5"; then
    echo "PASS: jobRepositoryName label found in CRD"
else
    echo "FAIL: jobRepositoryName label NOT found in CRD"
    exit 1
fi

echo "CRD validated successfully"

echo ""
echo "[4/4] Verifying jobPRID label is ABSENT..."

if echo "$workflow_crd" | grep -q "jobPRID"; then
    echo "FAIL: jobPRID label found but should not exist"
    exit 1
else
    echo "PASS: jobPRID label correctly absent"
fi

echo "jobPRID absence verified"

echo ""
echo "=========================================="
echo "PASS: test5-no-merge-id - All validations passed"
echo "=========================================="
exit 0
