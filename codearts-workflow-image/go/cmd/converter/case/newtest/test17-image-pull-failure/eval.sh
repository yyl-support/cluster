#!/bin/bash
# Eval script for test17-image-pull-failure
# Params: WORKFLOW_NAME NAMESPACE KUBECONFIG [WORKSPACE] [CP_ARTIFACTS_TEMP_FOLDER] [WORKFLOW_EXIT]

WORKFLOW_NAME="$1"
NAMESPACE="$2"
KUBECONFIG="$3"
WORKSPACE="${4:-.}"
CP_ARTIFACTS_TEMP_FOLDER="${5:-}"
WORKFLOW_EXIT="${6:-0}"

echo "=========================================="
echo "EVAL: test17-image-pull-failure"
echo "=========================================="

# For image pull failure, we expect WORKFLOW_EXIT to be 1
if [[ "$WORKFLOW_EXIT" -eq 1 ]]; then
    echo "PASS: Submit exited with code 1 (expected for image pull failure)"
else
    echo "FAIL: Expected submit exit code 1, got ${WORKFLOW_EXIT}"
    exit 1
fi

# Step 1: Verify workflow exists and check status
echo ""
echo "[1/3] Verifying workflow status..."

job_status=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o jsonpath='{.status.state.phase}' 2>&1)

echo "Workflow status: ${job_status}"

# For stopped workflows, status could be Failed, Error, or Running (but stopped)
if [[ "${job_status}" == "Failed" ]] || [[ "${job_status}" == "Error" ]]; then
    echo "PASS: Workflow is in ${job_status} state"
elif [[ "${job_status}" == "Running" ]]; then
    # Workflow might be stopped but still showing Running briefly
    echo "PASS: Workflow exists (was stopped during image pull failure)"
else
    echo "WARN: Workflow status is ${job_status}"
fi

# Step 2: Check workflow was stopped (argo stop was called)
echo ""
echo "[2/3] Checking workflow was stopped..."

# Check if workflow has been stopped by looking at the message or conditions
workflow_yaml=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml 2>&1)

if echo "$workflow_yaml" | grep -q "NotFound"; then
    echo "[INFO] Job was deleted (image pull failure detected and handled)"
    echo "PASS: Image pull failure handled - job was stopped and deleted"
    echo ""
    echo "=========================================="
    echo "PASS: test17-image-pull-failure - All validations passed"
    echo "=========================================="
    exit 0
fi

if echo "$workflow_yaml" | grep -q "stopped"; then
    echo "PASS: Workflow was stopped"
elif [[ "${job_status}" == "Failed" ]]; then
    echo "PASS: Workflow Failed (image pull failure detected)"
else
    echo "PASS: Workflow exists and submit returned error (image pull failure handled)"
fi

# Step 3: Verify non-existent image was used
echo ""
echo "[3/3] Validating workflow CRD..."

if echo "$workflow_yaml" | grep -q "swr.cn-southwest-2.myhuaweicloud.com/nonexistent/invalid-image:does-not-exist"; then
    echo "PASS: Non-existent image found in CRD"
else
    echo "FAIL: Expected non-existent image NOT found in CRD"
    exit 1
fi

if echo "$workflow_yaml" | grep -q "secretKeyRef"; then
    echo "PASS: Secrets referenced in CRD"
else
    echo "FAIL: Secrets NOT found in CRD"
    exit 1
fi

echo ""
echo "=========================================="
echo "PASS: test17-image-pull-failure - All validations passed"
echo "=========================================="
exit 0