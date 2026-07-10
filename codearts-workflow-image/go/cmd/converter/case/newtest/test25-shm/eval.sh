#!/bin/bash
# Eval script for test25-shm
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

if [[ -z "$WORKFLOW_NAME" ]]; then
    echo "FAIL: WORKFLOW_NAME is empty"
    exit 1
fi

echo "=========================================="
echo "EVAL: test25-shm"
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
        echo "FAIL: Volcano Job ${WORKFLOW_NAME} status: ${job_status}"
        exit 1
    fi
    
    echo "  Status: ${job_status} (${elapsed}s elapsed)"
    sleep $interval
    ((elapsed += interval))
done

if [[ "${job_status}" != "Completed" ]]; then
    echo "FAIL: Volcano Job ${WORKFLOW_NAME} timed out after ${max_wait}s"
    exit 1
fi
echo "PASS: Volcano Job status is Completed"

# Step 2: Fetch and validate logs
echo ""
echo "[2/4] Fetching and validating logs..."

pod_name=$(kubectl get pods -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -l volcano.sh/job-name=${WORKFLOW_NAME} -o jsonpath='{.items[0].metadata.name}' 2>&1)
workflow_logs=$(kubectl logs "$pod_name" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" 2>&1)

if echo "$workflow_logs" | grep -q "Testing /dev/shm volume mount"; then
    echo "PASS: Shell script output 'Testing /dev/shm volume mount' found in logs"
else
    echo "FAIL: Shell script output 'Testing /dev/shm volume mount' NOT found in logs"
    echo "Logs:"
    echo "$workflow_logs"
    exit 1
fi

echo "Logs fetched successfully"

# Step 3: Fetch and validate workflow CRD (shm volume)
echo ""
echo "[3/4] Fetching and validating workflow CRD..."

workflow_crd=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml)

if echo "$workflow_crd" | grep -q "name: shm"; then
    echo "PASS: Volume name 'shm' found in CRD"
else
    echo "FAIL: Volume name 'shm' NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "medium: Memory"; then
    echo "PASS: EmptyDir medium 'Memory' found in CRD"
else
    echo "FAIL: EmptyDir medium 'Memory' NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "sizeLimit: 128Mi"; then
    echo "PASS: Size limit '128Mi' found in CRD"
else
    echo "FAIL: Size limit '128Mi' NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "mountPath: /dev/shm"; then
    echo "PASS: Mount path '/dev/shm' found in CRD"
else
    echo "FAIL: Mount path '/dev/shm' NOT found in CRD"
    exit 1
fi

echo "CRD validated successfully"

# Step 4: Verify cleanup
echo ""
echo "[4/4] Verifying cleanup..."

echo "PASS: No special cleanup required for shm test"

echo ""
echo "=========================================="
echo "PASS: test25-shm - All validations passed"
echo "=========================================="
exit 0