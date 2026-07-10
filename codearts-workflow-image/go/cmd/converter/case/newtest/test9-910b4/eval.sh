#!/bin/bash
# Eval script for test9-910b4
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
echo "EVAL: test9-910b4"
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

if echo "$workflow_logs" | grep -qE "NPU\s+Name"; then
    echo "PASS: npu-smi info output found in logs"
else
    echo "FAIL: npu-smi info output NOT found in logs"
    echo "Logs:"
    echo "$workflow_logs"
    exit 1
fi

if echo "$workflow_logs" | grep -q "Running on 910B4 NPU"; then
    echo "PASS: Shell script output 'Running on 910B4 NPU' found in logs"
else
    echo "FAIL: Shell script output 'Running on 910B4 NPU' NOT found in logs"
    echo "Logs:"
    echo "$workflow_logs"
    exit 1
fi

echo ""
echo "[3/4] Fetching and validating workflow CRD..."

workflow_crd=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml)

if echo "$workflow_crd" | grep -q "jobPRID: \"100\""; then
    echo "PASS: jobPRID label '100' found in CRD"
else
    echo "FAIL: jobPRID label '100' NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "kubernetes.io/arch: arm64"; then
    echo "PASS: arm64 nodeSelector found in CRD"
else
    echo "FAIL: arm64 nodeSelector NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "node.kubernetes.io/npu.chip.name: 910B4"; then
    echo "PASS: 910B4 NPU chip nodeSelector found in CRD"
else
    echo "FAIL: 910B4 NPU chip nodeSelector NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "huawei.com/ascend-1980: \"1\""; then
    echo "PASS: huawei.com/ascend-1980 NPU resource '1' found in CRD"
else
    echo "FAIL: huawei.com/ascend-1980 NPU resource NOT found in CRD"
    exit 1
fi


if echo "$workflow_crd" | grep -q "memory: 48Gi"; then
    echo "PASS: Memory limit '48Gi' found in CRD"
else
    echo "FAIL: Memory limit '48Gi' NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "jobRepositoryName: testorg-testrepo-test9"; then
    echo "PASS: jobRepositoryName label found in CRD"
else
    echo "FAIL: jobRepositoryName label NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "pipeline/run-id: test-910b4-123"; then
    echo "PASS: pipeline/run-id label found in CRD"
else
    echo "FAIL: pipeline/run-id label NOT found in CRD"
    exit 1
fi

echo ""
echo "[4/4] Running test-specific validations..."

if echo "$workflow_crd" | grep -q "huawei.com/ascend-1980"; then
    echo "PASS: NPU resource huawei.com/ascend-1980 validated in CRD limits"
else
    echo "FAIL: NPU resource huawei.com/ascend-1980 NOT found in CRD limits"
    exit 1
fi

echo ""
echo "=========================================="
echo "PASS: test9-910b4 - All validations passed"
echo "=========================================="
exit 0
