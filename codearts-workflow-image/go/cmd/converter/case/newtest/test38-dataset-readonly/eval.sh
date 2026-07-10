#!/bin/bash
# Eval script for test38-dataset-readonly
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
echo "EVAL: test38-dataset-readonly"
echo "=========================================="

echo ""
echo "[1/3] Waiting for workflow completion..."

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
echo "[2/3] Fetching and validating logs..."

pod_name=$(kubectl get pods -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -l volcano.sh/job-name=${WORKFLOW_NAME} -o jsonpath='{.items[0].metadata.name}' 2>&1)
workflow_logs=$(kubectl logs "$pod_name" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" 2>&1)

if echo "$workflow_logs" | grep -q "Dataset read only test"; then
    echo "PASS: Dataset read only test log found"
else
    echo "FAIL: Dataset read only test log NOT found"
    echo "$workflow_logs"
    exit 1
fi

echo ""
echo "[3/3] Fetching and validating workflow CRD..."

workflow_crd=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml)

if echo "$workflow_crd" | grep -q "readOnly: true"; then
    echo "PASS: readOnly: true found in volumeMount"
else
    echo "FAIL: readOnly: true NOT found in volumeMount"
    exit 1
fi

if echo "$workflow_crd" | grep -q "claimName: testorg-testrepo-test15"; then
    echo "PASS: Shared PVC claim name found"
else
    echo "FAIL: Shared PVC claim name NOT found"
    exit 1
fi

if echo "$workflow_crd" | grep -q "mountPath: /dataset"; then
    echo "PASS: /dataset mount path found"
else
    echo "FAIL: /dataset mount path NOT found"
    exit 1
fi

echo ""
echo "=========================================="
echo "PASS: test38-dataset-readonly - All validations passed"
echo "=========================================="
exit 0
