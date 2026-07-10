#!/bin/bash
# Eval script for test36-image-pull-policy
# Params: WORKFLOW_NAME NAMESPACE KUBECONFIG

WORKFLOW_NAME="$1"
NAMESPACE="$2"
KUBECONFIG="$3"
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
echo "EVAL: test36-image-pull-policy"
echo "=========================================="

echo ""
echo "[1/2] Fetching Volcano Job CRD..."

workflow_crd=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml 2>/dev/null)

if [ -n "$workflow_crd" ]; then
    echo "$workflow_crd" | grep -q "imagePullPolicy: IfNotPresent" && pass "imagePullPolicy IfNotPresent found" || fail "imagePullPolicy IfNotPresent NOT found"
    echo "$workflow_crd" | grep -q "pipeline/run-id: test-pull-policy-123" && pass "pipeline/run-id found" || fail "pipeline/run-id NOT found"
else
    info "CRD not available (job cleaned up)"
fi

echo ""
echo "[2/2] Verifying Volcano Job completion..."

job_status=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o jsonpath='{.status.state.phase}' 2>&1)
if [[ "${job_status}" == "Completed" ]]; then
    pass "Volcano Job status is Completed"
elif [[ "${job_status}" == "Running" ]]; then
    pass "Volcano Job status is Running"
else
    fail "Volcano Job status is ${job_status}"
fi

echo ""
if [ "$failures" -gt 0 ]; then
    echo "=========================================="
    echo "FAIL: test36-image-pull-policy - ${failures} check(s) failed"
    echo "=========================================="
    exit 1
fi
echo "=========================================="
echo "PASS: test36-image-pull-policy - All validations passed"
echo "=========================================="
exit 0
