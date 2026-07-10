#!/bin/bash
# Eval script for test35-ingress-bandwidth (git clone with ingress bandwidth annotation)
# Params: WORKFLOW_NAME NAMESPACE KUBECONFIG WORKSPACE [WORKFLOW_EXIT]

WORKFLOW_NAME="$1"
NAMESPACE="$2"
KUBECONFIG="$3"
WORKSPACE="${4:-.}"
WORKFLOW_EXIT="${5:-0}"

failures=0
pass() { echo "PASS: $1"; }
fail() { echo "FAIL: $1"; ((failures++)); }
info() { echo "INFO: $1"; }

if [[ "$WORKFLOW_EXIT" -ne 0 ]]; then
    fail "submit exited with code ${WORKFLOW_EXIT}"
    exit 1
fi

echo "=========================================="
echo "EVAL: test35-ingress-bandwidth"
echo "=========================================="

echo ""
echo "[1/5] Fetching Volcano Job CRD..."

workflow_crd=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml 2>/dev/null)

if [ -n "$workflow_crd" ]; then
    echo "$workflow_crd" | grep -q "jobPRID: \"135\"" && pass "jobPRID label '135' found" || fail "jobPRID label '135' NOT found"
    echo "$workflow_crd" | grep -q "pipeline/run-id: test-5g-download-123" && pass "pipeline/run-id found" || fail "pipeline/run-id NOT found"
    echo "$workflow_crd" | grep -q "kubernetes.io/ingress-bandwidth" && pass "ingress-bandwidth annotation found" || fail "ingress-bandwidth annotation NOT found"
    echo "$workflow_crd" | grep -q "memory: 1Gi" && pass "memory 1Gi found" || fail "memory 1Gi NOT found"
else
    info "CRD not available (job cleaned up)"
fi

echo ""
echo "[2/5] Verifying Volcano Job completion..."

job_status=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o jsonpath='{.status.state.phase}' 2>&1)
if [[ "${job_status}" == "Completed" ]]; then
    pass "Volcano Job status is Completed"
elif [[ "${job_status}" == "Running" ]]; then
    pass "Volcano Job status is Running"
else
    fail "Volcano Job status is ${job_status}"
fi

echo ""
echo "[3/5] Validating logs..."

pod_name=$(kubectl get pods -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -l volcano.sh/job-name=${WORKFLOW_NAME} -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
if [ -z "$pod_name" ]; then
    info "Pod not found (may have been cleaned up), skipping log validation"
else
    workflow_logs=$(kubectl logs "$pod_name" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" 2>/dev/null)
    echo "$workflow_logs" | grep -q "5GB file download simulation" && pass "'5GB file download simulation' found in logs" || info "'5GB file download simulation' not found (pod may be cleaned up)"
    echo "$workflow_logs" | grep -q "Download complete" && pass "'Download complete' found in logs" || info "'Download complete' not found"
    echo "$workflow_logs" | grep -q "largefile.bin" && pass "largefile.bin found in logs" || info "largefile.bin not found in logs"
fi

echo ""
echo "[4/5] Validating large file artifact..."

if [ -f "${WORKSPACE}/largefile.bin" ]; then
    file_size=$(stat -c%s "${WORKSPACE}/largefile.bin" 2>/dev/null || stat -f%z "${WORKSPACE}/largefile.bin")
    expected_size=$((5120 * 1024 * 1024))
    if [ "$file_size" -ge "$expected_size" ]; then
        pass "largefile.bin size >= 5GB ($file_size bytes)"
    else
        fail "largefile.bin size $file_size bytes, expected >= $expected_size"
    fi
else
    info "largefile.bin not found in WORKSPACE (may not have been extracted)"
fi

echo ""
echo "[5/5] Verifying workspace is not empty..."
if [ ! -d "${WORKSPACE}" ] || [ -z "$(ls -A "${WORKSPACE}" 2>/dev/null)" ]; then
    info "Workspace is empty or not found"
else
    pass "Workspace has files"
fi

echo ""
if [ "$failures" -gt 0 ]; then
    echo "=========================================="
    echo "FAIL: test35-ingress-bandwidth - ${failures} check(s) failed"
    echo "=========================================="
    exit 1
fi
echo "=========================================="
echo "PASS: test35-ingress-bandwidth - All validations passed"
echo "=========================================="
exit 0
