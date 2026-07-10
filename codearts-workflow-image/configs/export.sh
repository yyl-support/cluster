#!/bin/bash

set -euo pipefail

KUBECONFIG=${KUBECONFIG:-/root/.kube/karmada-proxy.config}

echo "========================================="
echo "Karmada Configuration Export Script"
echo "========================================="
echo "Kubeconfig: $KUBECONFIG"
echo "========================================="

if [[ ! -f "$KUBECONFIG" ]]; then
    echo "Error: Kubeconfig file not found: $KUBECONFIG"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

mkdir -p "$SCRIPT_DIR/propagation-policies"
mkdir -p "$SCRIPT_DIR/override-policies"
mkdir -p "$SCRIPT_DIR/queues"
mkdir -p "$SCRIPT_DIR/rbac"

echo ""
echo "Exporting configurations to: $SCRIPT_DIR"
echo ""

echo "[Queues] Exporting Volcano Queue configurations..."
kubectl --kubeconfig="$KUBECONFIG" get queues --no-headers | awk '{print $1}' | while read name; do
    echo "  - $name.yaml"
    kubectl --kubeconfig="$KUBECONFIG" get queue "$name" -o yaml > "$SCRIPT_DIR/queues/${name}.yaml"
done
echo ""

echo "[PropagationPolicies] Exporting namespace-scoped policies..."
kubectl --kubeconfig="$KUBECONFIG" get propagationpolicies --all-namespaces --no-headers | awk '{print $1"/"$2}' | while read policy; do
    ns=$(echo "$policy" | cut -d'/' -f1)
    name=$(echo "$policy" | cut -d'/' -f2)
    filename="${ns}-${name}.yaml"
    echo "  - $filename"
    kubectl --kubeconfig="$KUBECONFIG" get propagationpolicy "$name" -n "$ns" -o yaml > "$SCRIPT_DIR/propagation-policies/${filename}"
done
echo ""

echo "[ClusterPropagationPolicies] Exporting cluster-scoped policies..."
kubectl --kubeconfig="$KUBECONFIG" get clusterpropagationpolicies --no-headers | awk '{print $1}' | while read name; do
    filename="cluster-${name}.yaml"
    echo "  - $filename"
    kubectl --kubeconfig="$KUBECONFIG" get clusterpropagationpolicy "$name" -o yaml > "$SCRIPT_DIR/propagation-policies/${filename}"
done
echo ""

echo "[OverridePolicies] Exporting override policies..."
kubectl --kubeconfig="$KUBECONFIG" get overridepolicies --all-namespaces --no-headers | awk '{print $1"/"$2}' | while read policy; do
    ns=$(echo "$policy" | cut -d'/' -f1)
    name=$(echo "$policy" | cut -d'/' -f2)
    filename="${ns}-${name}.yaml"
    echo "  - $filename"
    kubectl --kubeconfig="$KUBECONFIG" get overridepolicy "$name" -n "$ns" -o yaml > "$SCRIPT_DIR/override-policies/${filename}"
done || echo "  (No OverridePolicies found)"
echo ""

echo "[RBAC] Exporting relevant RBAC configurations..."
echo "  - vcjob-clusterroles.yaml"
kubectl --kubeconfig="$KUBECONFIG" get clusterrole vcjob-logger vcjob-submitter vcjob-proxy vcjob-unified queue-adjuster-role -o yaml > "$SCRIPT_DIR/rbac/vcjob-clusterroles.yaml" || true

echo "  - vcjob-clusterrolebindings.yaml"
kubectl --kubeconfig="$KUBECONFIG" get clusterrolebinding vcjob-logger vcjob-submitter vcjob-proxy vcjob-unified queue-adjuster-binding -o yaml > "$SCRIPT_DIR/rbac/vcjob-clusterrolebindings.yaml" || true

echo "  - volcano-global-serviceaccounts.yaml"
kubectl --kubeconfig="$KUBECONFIG" get serviceaccount -n volcano-global vcjob-logger vcjob-submitter -o yaml > "$SCRIPT_DIR/rbac/volcano-global-serviceaccounts.yaml" || true

echo "  - namespaces.yaml"
kubectl --kubeconfig="$KUBECONFIG" get namespace argo volcano-global -o yaml > "$SCRIPT_DIR/rbac/namespaces.yaml" || true
echo ""

echo "========================================="
echo "Configuration export completed!"
echo "========================================="
echo ""

# Clean runtime-generated fields
if [[ -f "$SCRIPT_DIR/clean.sh" ]]; then
    echo "Cleaning runtime-generated fields..."
    python3 "$SCRIPT_DIR/clean.sh" "$SCRIPT_DIR/propagation-policies" || true
    python3 "$SCRIPT_DIR/clean.sh" "$SCRIPT_DIR/override-policies" || true
    python3 "$SCRIPT_DIR/clean.sh" "$SCRIPT_DIR/queues" || true
    python3 "$SCRIPT_DIR/clean.sh" "$SCRIPT_DIR/rbac" || true
    echo "Cleaned successfully!"
    echo ""
fi

echo "Configuration files ready at: $SCRIPT_DIR"
echo ""
echo "To apply these configurations:"
echo "  cd configs && ./apply.sh"
echo ""
echo "Exporting configurations to: $CONFIG_DIR"
echo ""

echo "[Queues] Exporting Volcano Queue configurations..."
kubectl --kubeconfig="$KUBECONFIG" get queues --no-headers | awk '{print $1}' | while read name; do
    echo "  - $name.yaml"
    kubectl --kubeconfig="$KUBECONFIG" get queue "$name" -o yaml > "$CONFIG_DIR/queues/${name}.yaml"
done
echo ""

echo "[PropagationPolicies] Exporting namespace-scoped policies..."
kubectl --kubeconfig="$KUBECONFIG" get propagationpolicies --all-namespaces --no-headers | awk '{print $1"/"$2}' | while read policy; do
    ns=$(echo "$policy" | cut -d'/' -f1)
    name=$(echo "$policy" | cut -d'/' -f2)
    filename="${ns}-${name}.yaml"
    echo "  - $filename"
    kubectl --kubeconfig="$KUBECONFIG" get propagationpolicy "$name" -n "$ns" -o yaml > "$CONFIG_DIR/propagation-policies/${filename}"
done
echo ""

echo "[ClusterPropagationPolicies] Exporting cluster-scoped policies..."
kubectl --kubeconfig="$KUBECONFIG" get clusterpropagationpolicies --no-headers | awk '{print $1}' | while read name; do
    filename="cluster-${name}.yaml"
    echo "  - $filename"
    kubectl --kubeconfig="$KUBECONFIG" get clusterpropagationpolicy "$name" -o yaml > "$CONFIG_DIR/propagation-policies/${filename}"
done
echo ""

echo "[OverridePolicies] Exporting override policies..."
kubectl --kubeconfig="$KUBECONFIG" get overridepolicies --all-namespaces --no-headers | awk '{print $1"/"$2}' | while read policy; do
    ns=$(echo "$policy" | cut -d'/' -f1)
    name=$(echo "$policy" | cut -d'/' -f2)
    filename="${ns}-${name}.yaml"
    echo "  - $filename"
    kubectl --kubeconfig="$KUBECONFIG" get overridepolicy "$name" -n "$ns" -o yaml > "$CONFIG_DIR/override-policies/${filename}"
done || echo "  (No OverridePolicies found)"
echo ""

echo "[RBAC] Exporting relevant RBAC configurations..."
echo "  - vcjob-clusterroles.yaml"
kubectl --kubeconfig="$KUBECONFIG" get clusterrole vcjob-logger vcjob-submitter vcjob-proxy vcjob-unified queue-adjuster-role -o yaml > "$CONFIG_DIR/rbac/vcjob-clusterroles.yaml" || true

echo "  - vcjob-clusterrolebindings.yaml"
kubectl --kubeconfig="$KUBECONFIG" get clusterrolebinding vcjob-logger vcjob-submitter vcjob-proxy vcjob-unified queue-adjuster-binding -o yaml > "$CONFIG_DIR/rbac/vcjob-clusterrolebindings.yaml" || true

echo "  - volcano-global-serviceaccounts.yaml"
kubectl --kubeconfig="$KUBECONFIG" get serviceaccount -n volcano-global vcjob-logger vcjob-submitter -o yaml > "$CONFIG_DIR/rbac/volcano-global-serviceaccounts.yaml" || true

echo "  - namespaces.yaml"
kubectl --kubeconfig="$KUBECONFIG" get namespace argo volcano-global -o yaml > "$CONFIG_DIR/rbac/namespaces.yaml" || true
echo ""

echo "========================================="
echo "Configuration export completed!"
echo "========================================="
echo ""

# Clean runtime-generated fields
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
if [[ -f "$SCRIPT_DIR/clean.sh" ]]; then
    echo "Cleaning runtime-generated fields..."
    python3 "$SCRIPT_DIR/clean.sh" "$CONFIG_DIR/propagation-policies" || true
    python3 "$SCRIPT_DIR/clean.sh" "$CONFIG_DIR/override-policies" || true
    python3 "$SCRIPT_DIR/clean.sh" "$CONFIG_DIR/queues" || true
    python3 "$SCRIPT_DIR/clean.sh" "$CONFIG_DIR/rbac" || true
    echo "Cleaned successfully!"
    echo ""
fi

echo "Exported to: $CONFIG_DIR"
echo ""
echo "To apply these configurations:"
echo "  cd configs && ./apply.sh"
echo ""