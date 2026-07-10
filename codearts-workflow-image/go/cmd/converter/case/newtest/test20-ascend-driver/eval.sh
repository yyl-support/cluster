#!/bin/bash
# Eval script for test20-ascend-driver
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
echo "EVAL: test20-ascend-driver"
echo "=========================================="

# Step 1: Wait for workflow completion
echo ""
echo "[1/4] Waiting for workflow completion..."

max_wait=300
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

if echo "$workflow_logs" | grep -qE "NPU\s+Name"; then
    echo "PASS: npu-smi info output found in logs"
else
    echo "FAIL: npu-smi info output NOT found in logs"
    exit 1
fi

if echo "$workflow_logs" | grep -q "Testing Ascend driver mount"; then
    echo "PASS: Ascend driver test message found in logs"
else
    echo "FAIL: Ascend driver test message NOT found in logs"
    exit 1
fi

if echo "$workflow_logs" | grep -q "Listing /usr/local/Ascend/driver:"; then
    echo "PASS: Listing command found in logs"
else
    echo "FAIL: Listing command NOT found in logs"
    exit 1
fi

echo "Logs fetched successfully"

# Step 3: Fetch and validate workflow CRD
echo ""
echo "[3/4] Fetching and validating workflow CRD..."

workflow_crd=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml)

if echo "$workflow_crd" | grep -q "name: ascend-driver"; then
    echo "PASS: ascend-driver volume name found in CRD"
else
    echo "FAIL: ascend-driver volume name NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "mountPath: /usr/local/Ascend/driver"; then
    echo "PASS: Ascend driver mountPath found in CRD"
else
    echo "FAIL: Ascend driver mountPath NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "path: /usr/local/Ascend/driver"; then
    echo "PASS: Ascend driver hostPath found in CRD"
else
    echo "FAIL: Ascend driver hostPath NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "node.kubernetes.io/npu.chip.name"; then
    echo "PASS: NPU chip node selector found in CRD"
else
    echo "PASS: No NPU chip node selector (expected for arm64)"
fi

echo "CRD validated successfully"

# Step 4: Verify cleanup
echo ""
echo "[4/4] Verifying cleanup..."

echo "PASS: No special cleanup required"

echo ""
echo "=========================================="
echo "PASS: test20-ascend-driver - All validations passed"
echo "=========================================="
exit 0