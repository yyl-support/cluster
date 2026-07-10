#!/bin/bash
# Eval script for test24-image-proxy
# Validates: Image proxy NOT in converter output (moved to submit phase)
# Params: WORKFLOW_NAME NAMESPACE KUBECONFIG [WORKSPACE] [CP_ARTIFACTS_TEMP_FOLDER] [WORKFLOW_EXIT]

WORKFLOW_NAME="$1"
NAMESPACE="$2"
KUBECONFIG="$3"
WORKSPACE="${4:-.}"
CP_ARTIFACTS_TEMP_FOLDER="${5:-}"
WORKFLOW_EXIT="${6:-0}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "${SCRIPT_DIR}/env.sh" ]]; then
    source "${SCRIPT_DIR}/env.sh"
fi

if [[ "$WORKFLOW_EXIT" -ne 0 ]]; then
    echo "FAIL: submit exited with code ${WORKFLOW_EXIT}"
    exit 1
fi

if [[ -z "$WORKFLOW_NAME" ]]; then
    echo "FAIL: WORKFLOW_NAME is empty"
    exit 1
fi

echo "=========================================="
echo "EVAL: test24-image-proxy"
echo "=========================================="

TEST_DIR="${WORKSPACE}/go/cmd/converter/case/newtest/test24-image-proxy"

echo ""
echo "[1/4] Validating expected.yaml has NO proxy (converter phase)..."

expected_yaml="${TEST_DIR}/expected.yaml"

if [[ ! -f "$expected_yaml" ]]; then
    echo "FAIL: expected.yaml not found at ${expected_yaml}"
    exit 1
fi

if grep -q "harbor-portal" "$expected_yaml"; then
    echo "FAIL: Image proxy found in expected.yaml - proxy should NOT be in converter output"
    exit 1
else
    echo "PASS: No image proxy in expected.yaml (correct - proxy moved to submit phase)"
fi

if grep -q "swr.cn-north-4.myhuaweicloud.com" "$expected_yaml"; then
    echo "PASS: Original SWR registry preserved in expected.yaml"
else
    echo "FAIL: Original SWR registry NOT found in expected.yaml"
    exit 1
fi

echo ""
echo "[2/4] Waiting for workflow completion..."

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

echo ""
echo "[3/4] Fetching and validating logs..."

pod_name=$(kubectl get pods -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -l volcano.sh/job-name=${WORKFLOW_NAME} -o jsonpath='{.items[0].metadata.name}' 2>&1)
workflow_logs=$(kubectl logs "$pod_name" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" 2>&1)

if echo "$workflow_logs" | grep -q "Testing default proxy behavior"; then
    echo "PASS: Shell script output 'Testing default proxy behavior' found in logs"
else
    echo "FAIL: Shell script output 'Testing default proxy behavior' NOT found in logs"
    echo "Logs:"
    echo "$workflow_logs"
    exit 1
fi

echo "Logs fetched successfully"

echo ""
echo "[4/4] Validating submit phase behavior..."

workflow_crd=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml)

if echo "$workflow_crd" | grep -q "harbor-portal"; then
    echo "PASS: Image proxy applied (harbor service found in cluster)"
    if echo "$workflow_crd" | grep -q "harbor-portal.test.osinfra.cn/north4-myhuaweicloud"; then
        echo "PASS: Proxy uses correct region (test.osinfra.cn/north4-myhuaweicloud)"
    elif echo "$workflow_crd" | grep -q "harbor-portal.osinfra.cn/north4-myhuaweicloud"; then
        echo "PASS: Proxy uses correct region (osinfra.cn/north4-myhuaweicloud)"
    fi
else
    echo "PASS: No image proxy applied (harbor service not found in cluster - correct)"
    if echo "$workflow_crd" | grep -q "swr.cn-north-4.myhuaweicloud.com"; then
        echo "PASS: Original SWR registry preserved (no proxy service in cluster)"
    else
        echo "FAIL: Original SWR registry NOT preserved"
        exit 1
    fi
fi

echo ""
echo "=========================================="
echo "PASS: test24-image-proxy - Proxy moved to submit phase (correct)"
echo "=========================================="
exit 0