#!/bin/bash
# Eval script for test11-git-clone-var-ref
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
echo "EVAL: test11-git-clone-var-ref"
echo "=========================================="

TEST_DIR="${WORKSPACE}"

echo ""
echo "[1/4] Waiting for workflow completion..."

max_wait=180
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
echo "[2/4] Fetching and validating logs..."

pod_name=$(kubectl get pods -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -l volcano.sh/job-name=${WORKFLOW_NAME} -o jsonpath='{.items[0].metadata.name}' 2>&1)
workflow_logs=$(kubectl logs "$pod_name" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" 2>&1)

if echo "$workflow_logs" | grep -q "Build completed"; then
    echo "PASS: Shell script output 'Build completed' found in logs"
else
    echo "FAIL: Shell script output 'Build completed' NOT found in logs"
    echo "Logs:"
    echo "$workflow_logs"
    exit 1
fi

if echo "$workflow_logs" | grep -q "Listing workspace files:"; then
    echo "PASS: Shell script output 'Listing workspace files:' found in logs"
else
    echo "FAIL: Shell script output 'Listing workspace files:' NOT found in logs"
    echo "Logs:"
    echo "$workflow_logs"
    exit 1
fi

echo ""
echo "[3/4] Fetching and validating workflow CRD..."

workflow_crd=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml)

if echo "$workflow_crd" | grep -q "kubernetes.io/arch: arm64"; then
    echo "PASS: arm64 nodeSelector found in CRD"
else
    echo "FAIL: arm64 nodeSelector NOT found in CRD"
    exit 1
fi


if echo "$workflow_crd" | grep -q "memory: 8Gi"; then
    echo "PASS: Memory limit '8Gi' found in CRD"
else
    echo "FAIL: Memory limit '8Gi' NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "jobRepositoryName: ascend-ascendnpu-ir"; then
    echo "PASS: jobRepositoryName label found in CRD"
else
    echo "FAIL: jobRepositoryName label NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "pipeline/run-id: test-git-clone-123"; then
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

echo ""
echo "[4/4] Running test-specific validations..."

if echo "$workflow_crd" | grep -q "AscendNPU-IR.git"; then
    echo "PASS: AscendNPU-IR repository reference found in CRD"
else
    echo "FAIL: AscendNPU-IR repository reference NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "jobPRID"; then
    echo "FAIL: jobPRID label should NOT exist (mergeID is empty)"
    exit 1
else
    echo "PASS: jobPRID label not found (as expected, mergeID is empty)"
fi

if echo "$workflow_crd" | grep -q "git merge"; then
    echo "FAIL: git merge command should NOT exist (mergeID is empty)"
    exit 1
else
    echo "PASS: git merge command not found (as expected, mergeID is empty)"
fi

echo ""
echo "=========================================="
echo "PASS: test11-git-clone-var-ref - All validations passed"
echo "=========================================="
exit 0
