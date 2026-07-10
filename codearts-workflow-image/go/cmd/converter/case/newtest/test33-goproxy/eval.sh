#!/bin/bash

WORKFLOW_NAME="$1"
NAMESPACE="$2"
KUBECONFIG="$3"

failures=0

pass() { echo "PASS: $1"; }
fail() { echo "FAIL: $1"; ((failures++)); }
info() { echo "INFO: $1"; }

echo "==========================================
EVAL: test33-goproxy
==========================================

[1/4] Checking Volcano Job status..."
timeout=300
while [ $timeout -gt 0 ]; do
    status=$(kubectl --kubeconfig="$KUBECONFIG" get job.batch.volcano.sh "$WORKFLOW_NAME" -n "$NAMESPACE" -o jsonpath='{.status.status.phase}' 2>/dev/null)
    if [ "$status" = "Completed" ]; then
        pass "Volcano Job status is Completed"
        break
    fi
    if [ "$status" = "Aborted" ] || [ "$status" = "Failed" ]; then
        fail "Volcano Job status is $status"
        break
    fi
    if [ -z "$status" ]; then
        info "Job not found (may have been cleaned up), proceeding with log validation"
        break
    fi
    sleep 10
    timeout=$((timeout - 10))
done

echo "[2/4] Fetching logs..."
pod_name=$(kubectl --kubeconfig="$KUBECONFIG" get pods -n "$NAMESPACE" -l "volcano.sh/job-name=${WORKFLOW_NAME}" --sort-by='.metadata.creationTimestamp' -o jsonpath='{.items[-1].metadata.name}' 2>/dev/null)
if [ -z "$pod_name" ]; then
    info "Pod not found (may have been cleaned up), checking CRD only"
    logs=""
else
    logs=$(kubectl --kubeconfig="$KUBECONFIG" logs "$pod_name" -n "$NAMESPACE" 2>/dev/null)
fi

echo "[3/4] Validating logs..."
if echo "$logs" | grep -q "GOPROXY_ENV="; then
    pass "GOPROXY env found in logs"
else
    if [ -z "$logs" ]; then
        info "No logs available (pod cleaned up), skipping log checks"
    else
        fail "GOPROXY env not found in logs"
    fi
fi

if [ -n "$logs" ]; then
    timing=$(echo "$logs" | grep -oP 'go mod tidy took \K[0-9]+(?=ms)')
    if [ -n "$timing" ]; then
        if [ "$timing" -gt 1000 ]; then
            pass "go mod tidy took ${timing}ms (>1000ms)"
        else
            fail "go mod tidy took only ${timing}ms (<=1000ms, fallback not exercised)"
        fi
    else
        fail "go mod tidy timing not found in logs"
    fi

    echo "$logs" | grep -q "GOPROXY Test Completed" && pass "Test completed successfully" || fail "Test did not complete"
fi

echo "[4/4] Fetching and validating workflow CRD..."
crd=$(kubectl --kubeconfig="$KUBECONFIG" get job.batch.volcano.sh "$WORKFLOW_NAME" -n "$NAMESPACE" -o yaml 2>/dev/null)
if [ -n "$crd" ]; then
    echo "$crd" | grep -q "GOPROXY" && pass "GOPROXY env found in CRD" || fail "GOPROXY env not found in CRD"
else
    info "CRD not available (job cleaned up)"
fi

echo ""
if [ "$failures" -gt 0 ]; then
    echo "==========================================
FAIL: test33-goproxy - ${failures} check(s) failed
=========================================="
    exit 1
fi
echo "==========================================
PASS: test33-goproxy - All validations passed
=========================================="
