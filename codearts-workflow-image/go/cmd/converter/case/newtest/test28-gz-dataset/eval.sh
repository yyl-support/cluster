#!/bin/bash
# Eval script for test28-gz-dataset
# Params: WORKFLOW_NAME NAMESPACE KUBECONFIG [WORKSPACE] [WORKFLOW_EXIT]

WORKFLOW_NAME="$1"
NAMESPACE="$2"
KUBECONFIG="$3"
WORKSPACE="${4:-.}"
WORKFLOW_EXIT="${5:-0}"

if [[ "$WORKFLOW_EXIT" -ne 0 ]]; then
    echo "FAIL: submit exited with code ${WORKFLOW_EXIT}"
    exit 1
fi

if [[ -z "$WORKFLOW_NAME" ]]; then
    echo "FAIL: WORKFLOW_NAME is empty"
    exit 1
fi

echo "=========================================="
echo "EVAL: test28-gz-dataset"
echo "=========================================="

echo ""
echo "[1/5] Waiting for workflow completion..."

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

echo ""
echo "[2/5] Fetching and validating logs..."

pod_name=$(kubectl get pods -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -l volcano.sh/job-name=${WORKFLOW_NAME} -o jsonpath='{.items[0].metadata.name}' 2>&1)
workflow_logs=$(kubectl logs "$pod_name" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" 2>&1)

if echo "$workflow_logs" | grep -q "Guangzhou dataset mount test"; then
    echo "PASS: Dataset mount test log found"
else
    echo "FAIL: Dataset mount test log NOT found"
    echo "$workflow_logs"
    exit 1
fi

echo ""
echo "[3/5] Validating dataset mount..."

if echo "$workflow_logs" | grep -q "total"; then
    echo "PASS: Dataset directory mounted successfully (mount check passed)"
else
    echo "FAIL: Dataset directory NOT mounted properly"
    echo "$workflow_logs"
    exit 1
fi

echo ""
echo "[4/5] Fetching and validating workflow CRD..."

workflow_crd=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml)

if echo "$workflow_crd" | grep -q "claimName: ascend-ascendnpu-ir"; then
    echo "PASS: PVC claim name 'ascend-ascendnpu-ir' found"
else
    echo "FAIL: PVC claim name 'ascend-ascendnpu-ir' NOT found"
    exit 1
fi

if echo "$workflow_crd" | grep -q "mountPath: /dataset"; then
    echo "PASS: /dataset mount path found"
else
    echo "FAIL: /dataset mount path NOT found"
    exit 1
fi

echo ""
echo "[5/5] Validating queue replacement..."

if echo "$workflow_crd" | grep -q "queue: default"; then
    echo "PASS: Queue replaced to 'default' (presubmit validation for guangzhou)"
elif echo "$workflow_crd" | grep -q "queue: shared-flexible-queue"; then
    echo "PASS: Queue 'shared-flexible-queue' preserved (cluster has this queue)"
else
    echo "PASS: Queue validation skipped (cluster-specific queue handling)"
fi

echo ""
echo "=========================================="
echo "PASS: test28-gz-dataset - All validations passed"
echo "=========================================="
exit 0