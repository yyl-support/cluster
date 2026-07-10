#!/bin/bash
# Eval script for test4-custom-image
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
echo "EVAL: test4-custom-image"
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

if echo "$workflow_logs" | grep -q "Using custom Docker image"; then
    echo "PASS: Custom Docker image message found in logs"
else
    echo "FAIL: Custom Docker image message NOT found in logs"
    echo "Logs:"
    echo "$workflow_logs"
    exit 1
fi

if echo "$workflow_logs" | grep -q "Python 3"; then
    echo "PASS: Python version found in logs"
else
    echo "FAIL: Python version NOT found in logs"
    exit 1
fi

echo "Logs fetched successfully"

echo ""
echo "[3/4] Fetching and validating workflow CRD..."

workflow_crd=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml)

expected_image="swr.cn-southwest-2.myhuaweicloud.com/base_image/ascend-ci/cann:8.2.rc1-910b-ubuntu22.04-py3.11"
if echo "$workflow_crd" | grep -q "image: ${expected_image}"; then
    echo "PASS: Custom image found in CRD"
else
    echo "FAIL: Custom image NOT found in CRD"
    echo "Expected image: ${expected_image}"
    exit 1
fi

echo "CRD validated successfully"

echo ""
echo "[4/4] Verifying jobPRID label..."

if echo "$workflow_crd" | grep -q "jobPRID"; then
    echo "PASS: jobPRID label found"
else
    echo "FAIL: jobPRID label NOT found"
    exit 1
fi

echo "jobPRID label verified"

echo ""
echo "=========================================="
echo "PASS: test4-custom-image - All validations passed"
echo "=========================================="
exit 0
