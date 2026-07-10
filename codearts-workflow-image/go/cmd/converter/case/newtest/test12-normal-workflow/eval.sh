#!/bin/bash
# Eval script for test12-normal-workflow (Volcano Job multi-task)
# Params: WORKFLOW_NAME NAMESPACE KUBECONFIG WORKSPACE [WORKFLOW_EXIT]

WORKFLOW_NAME="$1"
NAMESPACE="$2"
KUBECONFIG="$3"
WORKSPACE="${4:-.}"
WORKFLOW_EXIT="${5:-0}"

if [[ "$WORKFLOW_EXIT" -ne 0 ]]; then
    echo "FAIL: submit exited with code ${WORKFLOW_EXIT}"
    exit 1
fi

echo "=========================================="
echo "EVAL: test12-normal-workflow"
echo "=========================================="

echo ""
echo "[1/5] Fetching Volcano Job CRD..."

workflow_crd=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml)

if echo "$workflow_crd" | grep -q "jobPRID: \"112\""; then
    echo "PASS: jobPRID label '112' found in CRD"
else
    echo "FAIL: jobPRID label '112' NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "pipeline/run-id: test-normal-workflow-123"; then
    echo "PASS: pipeline/run-id label found in CRD"
else
    echo "FAIL: pipeline/run-id label NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "name: main-script"; then
    echo "PASS: main-script task found"
else
    echo "FAIL: main-script task NOT found"
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

if echo "$workflow_crd" | grep -q "mountPath: /output"; then
    echo "PASS: mountPath '/output' found"
else
    echo "FAIL: mountPath '/output' NOT found"
    exit 1
fi

echo ""
echo "[2/5] Validating artifacts..."

if [ -f "${WORKSPACE}/test.txt" ]; then
    content=$(cat "${WORKSPACE}/test.txt")
    if [ "$content" = "xxxx" ]; then
        echo "PASS: Artifact file test.txt found with correct content"
    else
        echo "FAIL: Artifact file test.txt has incorrect content: $content"
        exit 1
    fi
else
    echo "FAIL: Artifact file test.txt NOT found"
    exit 1
fi

if [ -f "${WORKSPACE}/debug.log" ]; then
    content=$(cat "${WORKSPACE}/debug.log")
    if [ "$content" = "yyyy" ]; then
        echo "PASS: Artifact file debug.log found with correct content"
    else
        echo "FAIL: Artifact file debug.log has incorrect content: $content"
        exit 1
    fi
else
    echo "FAIL: Artifact file debug.log NOT found"
    exit 1
fi

echo ""
echo "[3/5] Verifying Volcano Job completion..."

job_status=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o jsonpath='{.status.state.phase}' 2>&1)

if [[ "${job_status}" == "Completed" ]]; then
    echo "PASS: Volcano Job status is Completed"
elif [[ "${job_status}" == "Running" ]]; then
    echo "PASS: Volcano Job status is Running"
else
    echo "FAIL: Volcano Job status is ${job_status}"
    exit 1
fi

echo ""
echo "[4/5] Validating logs..."

pod_name=$(kubectl get pods -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -l volcano.sh/job-name=${WORKFLOW_NAME} -o jsonpath='{.items[0].metadata.name}' 2>&1)
workflow_logs=$(kubectl logs "$pod_name" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" 2>&1)

if echo "$workflow_logs" | grep -q "Building artifacts..."; then
    echo "PASS: 'Building artifacts...' found in logs"
else
    echo "FAIL: 'Building artifacts...' NOT found in logs"
    exit 1
fi

if echo "$workflow_logs" | grep -q "Build complete"; then
    echo "PASS: 'Build complete' found in logs"
else
    echo "FAIL: 'Build complete' NOT found in logs"
    exit 1
fi

echo ""
echo "[5/5] Verifying workspace is not empty..."

if [ ! -d "${WORKSPACE}" ] || [ -z "$(ls -A "${WORKSPACE}" 2>/dev/null)" ]; then
    echo "FAIL: Workspace is empty"
    exit 1
fi
echo "PASS: Workspace has files"

echo ""
echo "=========================================="
echo "PASS: test12-normal-workflow - All validations passed"
echo "=========================================="
exit 0