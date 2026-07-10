#!/bin/bash
# Eval script for test31-git-clone-cp-artifacts
# Combines: git clone (var-ref style) + cp artifact extraction

WORKFLOW_NAME="$1"
NAMESPACE="$2"
KUBECONFIG="$3"
WORKSPACE="${4:-.}"
WORKFLOW_EXIT="${6:-0}"

if [[ "$WORKFLOW_EXIT" -ne 0 ]]; then
    echo "FAIL: submit exited with code ${WORKFLOW_EXIT}"
    exit 1
fi

echo "=========================================="
echo "EVAL: test31-git-clone-cp-artifacts"
echo "=========================================="

echo ""
echo "[1/5] Fetching Volcano Job CRD..."

workflow_crd=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml)

if echo "$workflow_crd" | grep -q "kubernetes.io/arch: arm64"; then
    echo "PASS: arm64 nodeSelector found in CRD"
else
    echo "FAIL: arm64 nodeSelector NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "jobRepositoryName: ascend-ascendnpu-ir"; then
    echo "PASS: jobRepositoryName label found in CRD"
else
    echo "FAIL: jobRepositoryName label NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "pipeline/run-id: test-git-clone-cp-123"; then
    echo "PASS: pipeline/run-id label found in CRD"
else
    echo "FAIL: pipeline/run-id label NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "git clone"; then
    echo "PASS: Git clone command found in CRD"
else
    echo "FAIL: Git clone command NOT found in CRD"
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

if echo "$workflow_crd" | grep -q "AscendNPU-IR.git"; then
    echo "PASS: AscendNPU-IR repository reference found in CRD"
else
    echo "FAIL: AscendNPU-IR repository reference NOT found in CRD"
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

max_wait=180
interval=10
elapsed=0

while [ $elapsed -lt $max_wait ]; do
    job_status=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o jsonpath='{.status.state.phase}' 2>&1)

    if [[ "${job_status}" == "Completed" ]]; then
        break
    fi

    if [[ "${job_status}" == "Running" ]]; then
        echo "  Status: ${job_status} (${elapsed}s elapsed)"
        sleep $interval
        ((elapsed += interval))
        continue
    fi

    if [[ "${job_status}" == "Failed" ]] || [[ "${job_status}" == "Error" ]]; then
        echo "FAIL: Workflow ${WORKFLOW_NAME} status: ${job_status}"
        exit 1
    fi

    echo "  Status: ${job_status} (${elapsed}s elapsed)"
    sleep $interval
    ((elapsed += interval))
done

if [[ "${job_status}" == "Completed" ]]; then
    echo "PASS: Volcano Job status is Completed"
elif [[ "${job_status}" == "Running" ]]; then
    echo "PASS: Volcano Job status is Running"
else
    echo "FAIL: Workflow ${WORKFLOW_NAME} timed out after ${max_wait}s"
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
echo "PASS: test31-git-clone-cp-artifacts - All validations passed"
echo "=========================================="
exit 0
