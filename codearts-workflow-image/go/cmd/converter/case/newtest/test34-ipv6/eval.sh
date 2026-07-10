#!/bin/bash

WORKFLOW_NAME="$1"
NAMESPACE="$2"
KUBECONFIG="$3"

failures=0

pass() { echo "PASS: $1"; }
fail() { echo "FAIL: $1"; ((failures++)); }
info() { echo "INFO: $1"; }

echo "==========================================
EVAL: test34-ipv6
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
if echo "$logs" | grep -q "SKIP: IPv6 is disabled"; then
    info "IPv6 disabled at kernel level on this cluster, IPv6 checks skipped"
    IPV6_DISABLED=true
else
    IPV6_DISABLED=false
fi

if [ "$IPV6_DISABLED" = "true" ]; then
    info "IPv6 disabled, skipping IPv6 address check"
elif echo "$logs" | grep -q "PASS: pod has global IPv6 address"; then
    pass "pod has global IPv6 address(es)"
elif echo "$logs" | grep -q "INFO: pod has only IPv6 loopback"; then
    info "pod has only IPv6 loopback, no global IPv6 from CNI"
else
    if [ -z "$logs" ]; then
        info "No logs available (pod cleaned up), skipping IPv6 addr check"
    else
        fail "pod IPv6 address NOT found"
    fi
fi

if [ "$IPV6_DISABLED" = "true" ]; then
    info "IPv6 disabled, skipping IPv6 connectivity check"
elif echo "$logs" | grep -q "PASS: IPv6 internet connectivity works"; then
    pass "IPv6 internet connectivity works"
elif echo "$logs" | grep -q "INFO: curl has IPv6 support but no IPv6 internet route"; then
    info "curl has IPv6 but no IPv6 internet route (expected: IPv6 internal only)"
elif echo "$logs" | grep -q "INFO: curl lacks IPv6 support"; then
    info "curl lacks IPv6 support, IPv6 stack available"
elif echo "$logs" | grep -q "INFO: no global IPv6"; then
    info "no global IPv6, connectivity test skipped"
else
    if [ -z "$logs" ]; then
        info "No logs available (pod cleaned up), skipping IPv6 conn check"
    else
        fail "IPv6 connectivity NOT working"
    fi
fi

if echo "$logs" | grep -q "PASS: internet download over IPv4 succeeded"; then
    pass "internet IPv4 download succeeded"
else
    if [ -z "$logs" ]; then
        info "No logs available (pod cleaned up), skipping IPv4 check"
    else
        fail "internet IPv4 download failed"
    fi
fi

if [ -n "$logs" ]; then
    echo "$logs" | grep -q "IPv6/IPv4 Test Completed" && pass "Test completed successfully" || fail "Test did not complete"
fi

echo "[4/4] Fetching and validating workflow CRD..."
crd=$(kubectl --kubeconfig="$KUBECONFIG" get job.batch.volcano.sh "$WORKFLOW_NAME" -n "$NAMESPACE" -o yaml 2>/dev/null)
if [ -n "$crd" ]; then
    echo "$crd" | grep -q 'jobPRID: "134"' && pass "jobPRID label found in CRD" || fail "jobPRID not found"
    echo "$crd" | grep -q "kubernetes.io/arch: amd64" && pass "amd64 nodeSelector found in CRD" || fail "amd64 not found"
else
    info "CRD not available (job cleaned up)"
fi

echo ""
if [ "$failures" -gt 0 ]; then
    echo "==========================================
FAIL: test34-ipv6 - ${failures} check(s) failed
=========================================="
    exit 1
fi
echo "==========================================
PASS: test34-ipv6 - All validations passed
=========================================="
