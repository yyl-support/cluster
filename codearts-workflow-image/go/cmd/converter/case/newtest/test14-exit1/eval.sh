#!/bin/bash
# Eval script for test14-exit1
# Params: WORKFLOW_NAME NAMESPACE KUBECONFIG [WORKSPACE] [CP_ARTIFACTS_TEMP_FOLDER] [WORKFLOW_EXIT]

WORKFLOW_NAME="$1"
NAMESPACE="$2"
KUBECONFIG="$3"
WORKSPACE="${4:-.}"
CP_ARTIFACTS_TEMP_FOLDER="${5:-}"
WORKFLOW_EXIT="${6:-0}"

# For exit1 test, we expect WORKFLOW_EXIT to be 1 (pod failed)
if [[ "$WORKFLOW_EXIT" -eq 1 ]]; then
    echo "PASS: Workflow execution failed as expected (exit code 1)"
else
    echo "FAIL: Expected workflow exit code 1, got ${WORKFLOW_EXIT}"
    exit 1
fi

echo "=========================================="
echo "EVAL: test14-exit1"
echo "=========================================="

# Configuration
TEST_DIR="${WORKSPACE}"

# Step 1: Wait for workflow completion
echo ""
echo "[1/3] Waiting for workflow completion..."

max_wait=120
interval=10
elapsed=0

while [ $elapsed -lt $max_wait ]; do
    job_status=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o jsonpath='{.status.state.phase}' 2>&1)
    
    if [[ "${job_status}" == "Failed" ]] || [[ "${job_status}" == "Error" ]] || [[ "${job_status}" == "Aborted" ]]; then
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

if [[ "${job_status}" != "Failed" ]] && [[ "${job_status}" != "Error" ]] && [[ "${job_status}" != "Aborted" ]]; then
    echo "FAIL: Workflow ${WORKFLOW_NAME} timed out after ${max_wait}s"
    exit 1
fi
echo "PASS: Workflow failed as expected"

# Step 2: Fetch and validate logs
echo ""
echo "[2/3] Fetching and validating logs..."

pod_name=$(kubectl get pods -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -l volcano.sh/job-name=${WORKFLOW_NAME} -o jsonpath='{.items[0].metadata.name}' 2>&1)
workflow_logs=$(kubectl logs "$pod_name" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" 2>&1)

if echo "$workflow_logs" | grep -q "Hello from test script"; then
    echo "PASS: Shell script output 'Hello from test script' found in logs"
else
    echo "FAIL: Shell script output 'Hello from test script' NOT found in logs"
    echo "Logs:"
    echo "$workflow_logs"
    exit 1
fi

echo "Logs fetched successfully"

# Step 3: Fetch and validate workflow CRD
echo ""
echo "[3/3] Fetching and validating workflow CRD..."

workflow_crd=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml)


if echo "$workflow_crd" | grep -q "memory: 8Gi"; then
    echo "PASS: Memory limit '8Gi' found in CRD"
else
    echo "FAIL: Memory limit '8Gi' NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "swr.cn-southwest-2.myhuaweicloud.com/base_image/ascend-ci/cann:8.2.rc1-910b-ubuntu22.04-py3.11"; then
    echo "PASS: Custom image found in CRD"
else
    echo "FAIL: Custom image NOT found in CRD"
    exit 1
fi

echo "CRD validated successfully"

echo ""
echo "=========================================="
echo "PASS: test14-exit1 - All validations passed"
echo "=========================================="
exit 0
