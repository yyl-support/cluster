#!/bin/bash
# Eval script for test3-custom-resources
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
echo "EVAL: test3-custom-resources"
echo "=========================================="

# Configuration
TEST_DIR="${WORKSPACE}"

# Step 1: Wait for workflow completion
echo ""
echo "[1/4] Waiting for workflow completion..."

max_wait=120
interval=10
elapsed=0

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

# Step 2: Fetch and validate logs
echo ""
echo "[2/4] Fetching and validating logs..."

pod_name=$(kubectl get pods -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -l volcano.sh/job-name=${WORKFLOW_NAME} -o jsonpath='{.items[0].metadata.name}' 2>&1)
workflow_logs=$(kubectl logs "$pod_name" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" 2>&1)

if echo "$workflow_logs" | grep -q "Running with custom resources"; then
    echo "PASS: Shell script output 'Running with custom resources' found in logs"
else
    echo "FAIL: Shell script output 'Running with custom resources' NOT found in logs"
    echo "Logs:"
    echo "$workflow_logs"
    exit 1
fi

if echo "$workflow_logs" | grep -q "CPU and memory as specified"; then
    echo "PASS: Shell script output 'CPU and memory as specified' found in logs"
else
    echo "FAIL: Shell script output 'CPU and memory as specified' NOT found in logs"
    exit 1
fi

echo "Logs fetched successfully"

# Step 3: Fetch and validate workflow CRD
echo ""
echo "[3/4] Fetching and validating workflow CRD..."

workflow_crd=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml)


if echo "$workflow_crd" | grep -q "memory: 32Gi"; then
    echo "PASS: Custom memory limit '32Gi' found in CRD"
else
    echo "FAIL: Custom memory limit '32Gi' NOT found in CRD"
    exit 1
fi


if echo "$workflow_crd" | grep -q "swr.cn-southwest-2.myhuaweicloud.com/base_image/ascend-ci/cann:8.2.rc1-910b-ubuntu22.04-py3.11"; then
    echo "PASS: Custom image found in CRD"
else
    echo "FAIL: Custom image NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "kubernetes.io/arch: arm64"; then
    echo "PASS: Node selector 'arm64' found in CRD"
else
    echo "FAIL: Node selector 'arm64' NOT found in CRD"
    exit 1
fi

echo "CRD validated successfully"

# Step 4: Verify cleanup
echo ""
echo "[4/4] Verifying cleanup..."

echo "PASS: No special cleanup required for custom resources test"

echo ""
echo "=========================================="
echo "PASS: test3-custom-resources - All validations passed"
echo "=========================================="
exit 0
