#!/bin/bash

set -euo pipefail

KUBECONFIG=${KUBECONFIG:-/root/.kube/karmada-proxy.config}

echo "========================================="
echo "Karmada Configuration Sync Script"
echo "========================================="
echo "Kubeconfig: $KUBECONFIG"
echo "========================================="
echo ""
echo "This script will:"
echo "  1. Delete resources NOT in configs directory"
echo "  2. Apply/Update resources IN configs directory"
echo "  3. Handle special cases (protection labels, missing namespaces)"
echo ""

if [[ ! -f "$KUBECONFIG" ]]; then
    echo "Error: Kubeconfig file not found: $KUBECONFIG"
    exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_DIR="$SCRIPT_DIR"

echo "Config directory: $CONFIG_DIR"
echo ""

# ============================================
# Function: Extract names from YAML files
# ============================================
extract_names_from_file() {
    local file="$1"
    local kind="$2"
    
    if grep -q "kind: $kind" "$file"; then
        # Extract name from metadata blocks only
        # Metadata block ends at 'spec:' field
        # name in metadata has exactly 4 spaces indentation in List format, 2 spaces in single file
        sed -n '/metadata:/,/spec:/p' "$file" | grep -E '^\s{2,4}name:' | sed 's/.*name:\s*//' | grep -v '^$'
    fi
}

extract_namespace_from_file() {
    local file="$1"
    
    # Extract namespace from metadata blocks only
    sed -n '/metadata:/,/spec:/p' "$file" | grep -E '^\s{2,4}namespace:' | sed 's/.*namespace:\s*//' | grep -v '^$' | head -1
}

# ============================================
# Function: Get resource names from configs
# ============================================
get_config_clusterpropagationpolicies() {
    local names=""
    for file in "$CONFIG_DIR/propagation-policies"/*.yaml; do
        if [[ -f "$file" ]]; then
            local file_names=$(extract_names_from_file "$file" "ClusterPropagationPolicy")
            names="${names} ${file_names}"
        fi
    done
    echo "$names" | tr ' ' '\n' | grep -v '^$' | sort -u
}

get_config_propagationpolicies() {
    local names=""
    for file in "$CONFIG_DIR/propagation-policies"/*.yaml; do
        if [[ -f "$file" ]]; then
            if grep -q "kind: PropagationPolicy" "$file"; then
                local ns=$(extract_namespace_from_file "$file")
                local file_names=$(extract_names_from_file "$file" "PropagationPolicy")
                for name in $file_names; do
                    [[ -n "$name" ]] && names="${names} ${ns}/${name}"
                done
            fi
        fi
    done
    echo "$names" | tr ' ' '\n' | grep -v '^$' | sort -u
}

get_config_queues() {
    local names=""
    for file in "$CONFIG_DIR/queues"/*.yaml; do
        if [[ -f "$file" ]]; then
            local file_names=$(extract_names_from_file "$file" "Queue")
            names="${names} ${file_names}"
        fi
    done
    echo "$names" | tr ' ' '\n' | grep -v '^$' | sort -u
}

# ============================================
# Function: Get current resources from Karmada
# ============================================
get_current_clusterpropagationpolicies() {
    kubectl --kubeconfig="$KUBECONFIG" get clusterpropagationpolicies --no-headers 2>/dev/null | awk '{print $1}' | sort -u || echo ""
}

get_current_propagationpolicies() {
    kubectl --kubeconfig="$KUBECONFIG" get propagationpolicies --all-namespaces --no-headers 2>/dev/null | awk '{print $1"/"$2}' | sort -u || echo ""
}

get_current_queues() {
    kubectl --kubeconfig="$KUBECONFIG" get queues --no-headers 2>/dev/null | awk '{print $1}' | sort -u || echo ""
}

# ============================================
# Function: Remove deletion protection label
# ============================================
remove_deletion_protection_label() {
    local resource_type="$1"
    local name="$2"
    local namespace="${3:-}"
    
    local cmd="kubectl --kubeconfig=$KUBECONFIG get $resource_type $name"
    [[ -n "$namespace" ]] && cmd="$cmd -n $namespace"
    
    local protected=$(eval "$cmd -o yaml 2>/dev/null" | grep "resourcetemplate.karmada.io/deletion-protected" || true)
    
    if [[ -n "$protected" ]]; then
        echo "    Removing deletion protection label..."
        local label_cmd="kubectl --kubeconfig=$KUBECONFIG label $resource_type $name resourcetemplate.karmada.io/deletion-protected-"
        [[ -n "$namespace" ]] && label_cmd="$label_cmd -n $namespace"
        eval "$label_cmd 2>/dev/null || true"
    fi
}

# ============================================
# Function: Delete extra resources
# ============================================
delete_extra_clusterpropagationpolicies() {
    local config_names="$1"
    local current_names="$2"
    
    echo "[ClusterPropagationPolicy] Checking for extra resources..."
    
    for current in $current_names; do
        if ! echo "$config_names" | grep -qw "$current"; then
            echo "  - Deleting $current (not in configs)"
            remove_deletion_protection_label "clusterpropagationpolicy" "$current"
            kubectl --kubeconfig="$KUBECONFIG" delete clusterpropagationpolicy "$current" --ignore-not-found=true 2>&1 | grep -v "^$" || true
        fi
    done
    echo ""
}

delete_extra_propagationpolicies() {
    local config_names="$1"
    local current_names="$2"
    
    echo "[PropagationPolicy] Checking for extra resources..."
    
    for current in $current_names; do
        if ! echo "$config_names" | grep -qw "$current"; then
            local ns=$(echo "$current" | cut -d'/' -f1)
            local name=$(echo "$current" | cut -d'/' -f2)
            echo "  - Deleting $ns/$name (not in configs)"
            remove_deletion_protection_label "propagationpolicy" "$name" "$ns"
            kubectl --kubeconfig="$KUBECONFIG" delete propagationpolicy "$name" -n "$ns" --ignore-not-found=true 2>&1 | grep -v "^$" || true
        fi
    done
    echo ""
}

delete_extra_queues() {
    local config_names="$1"
    local current_names="$2"
    
    echo "[Queue] Checking for extra resources..."
    
    for current in $current_names; do
        if ! echo "$config_names" | grep -qw "$current"; then
            echo "  - Deleting $current (not in configs)"
            remove_deletion_protection_label "queue" "$current"
            kubectl --kubeconfig="$KUBECONFIG" delete queue "$current" --ignore-not-found=true 2>&1 | grep -v "^$" || true
        fi
    done
    echo ""
}

# ============================================
# Function: Apply file (handles special cases)
# ============================================
apply_file() {
    local file="$1"
    local filename=$(basename "$file")
    
    echo "  - $filename"
    
    # Dry-run validation
    local dry_run_output=$(kubectl --kubeconfig="$KUBECONFIG" apply -f "$file" --dry-run=client 2>&1)
    if echo "$dry_run_output" | grep -q "error"; then
        echo "    Error: Validation failed"
        echo "$dry_run_output" | head -5
        return 1
    fi
    
    # Check if file contains resources with deletion protection
    if grep -q "resourcetemplate.karmada.io/deletion-protected" "$file"; then
        # Extract resource types and names to remove protection labels
        local kinds=$(grep "kind:" "$file" | sed 's/kind:\s*//' | grep -v "List")
        for kind in $kinds; do
            local names=$(extract_names_from_file "$file" "$kind")
            for name in $names; do
                [[ -z "$name" ]] && continue
                local lowercase_kind=$(echo "$kind" | tr '[:upper:]' '[:lower:]')
                
                if [[ "$kind" == "PropagationPolicy" ]]; then
                    local ns=$(extract_namespace_from_file "$file")
                    remove_deletion_protection_label "$lowercase_kind" "$name" "$ns"
                else
                    remove_deletion_protection_label "$lowercase_kind" "$name"
                fi
            done
        done
    fi
    
    # Apply the file
    local apply_output=$(kubectl --kubeconfig="$KUBECONFIG" apply -f "$file" 2>&1)
    if echo "$apply_output" | grep -q "error"; then
        echo "    Error: Apply failed"
        echo "$apply_output" | grep "error" | head -5
        return 1
    fi
    
    # Show created/configured messages
    echo "$apply_output" | grep -E "(created|configured|unchanged)" || true
}

# ============================================
# Function: Apply configurations
# ============================================
apply_clusterpropagationpolicies() {
    echo "[ClusterPropagationPolicy] Applying configurations..."
    
    for file in "$CONFIG_DIR/propagation-policies"/*.yaml; do
        if [[ -f "$file" ]]; then
            if grep -q "kind: ClusterPropagationPolicy" "$file"; then
                apply_file "$file"
            fi
        fi
    done
    echo ""
}

apply_propagationpolicies() {
    echo "[PropagationPolicy] Applying configurations..."
    
    for file in "$CONFIG_DIR/propagation-policies"/*.yaml; do
        if [[ -f "$file" ]]; then
            if grep -q "kind: PropagationPolicy" "$file"; then
                # Ensure namespace exists before applying
                local ns=$(extract_namespace_from_file "$file")
                if [[ -n "$ns" ]]; then
                    if ! kubectl --kubeconfig="$KUBECONFIG" get namespace "$ns" >/dev/null 2>&1; then
                        echo "    Creating namespace $ns (doesn't exist)"
                        kubectl --kubeconfig="$KUBECONFIG" create namespace "$ns" 2>&1 | grep -v "^$" || true
                    fi
                fi
                apply_file "$file"
            fi
        fi
    done
    echo ""
}

apply_queues() {
    echo "[Queue] Applying configurations..."
    
    for file in "$CONFIG_DIR/queues"/*.yaml; do
        if [[ -f "$file" ]]; then
            apply_file "$file"
        fi
    done
    echo ""
}

# ============================================
# Main Execution
# ============================================

echo "========================================="
echo "Step 1: Analyzing current state"
echo "========================================="
echo ""

config_cpp=$(get_config_clusterpropagationpolicies)
current_cpp=$(get_current_clusterpropagationpolicies)

config_pp=$(get_config_propagationpolicies)
current_pp=$(get_current_propagationpolicies)

config_queue=$(get_config_queues)
current_queue=$(get_current_queues)

echo "Config ClusterPropagationPolicy count: $(echo "$config_cpp" | grep -c . || echo 0)"
echo "Current ClusterPropagationPolicy count: $(echo "$current_cpp" | grep -c . || echo 0)"
echo ""
echo "Config PropagationPolicy count: $(echo "$config_pp" | grep -c . || echo 0)"
echo "Current PropagationPolicy count: $(echo "$current_pp" | grep -c . || echo 0)"
echo ""
echo "Config Queue count: $(echo "$config_queue" | grep -c . || echo 0)"
echo "Current Queue count: $(echo "$current_queue" | grep -c . || echo 0)"
echo ""

echo "========================================="
echo "Step 2: Deleting extra resources"
echo "========================================="
echo ""

delete_extra_clusterpropagationpolicies "$config_cpp" "$current_cpp"
delete_extra_propagationpolicies "$config_pp" "$current_pp"
delete_extra_queues "$config_queue" "$current_queue"

echo "========================================="
echo "Step 3: Applying configurations"
echo "========================================="
echo ""

apply_clusterpropagationpolicies
apply_propagationpolicies
apply_queues

echo "========================================="
echo "Step 4: Verification"
echo "========================================="
echo ""

echo "ClusterPropagationPolicy:"
kubectl --kubeconfig="$KUBECONFIG" get clusterpropagationpolicies
echo ""

echo "PropagationPolicy:"
kubectl --kubeconfig="$KUBECONFIG" get propagationpolicies --all-namespaces
echo ""

echo "Queue:"
kubectl --kubeconfig="$KUBECONFIG" get queues
echo ""

echo "========================================="
echo "Sync completed successfully!"
echo "========================================="
echo ""
echo "Summary of features:"
echo "  ✓ Dry-run validation before apply"
echo "  ✓ Deletion protection label handling"
echo "  ✓ Missing namespace creation"
echo "  ✓ Multi-policy YAML files support"
echo "  ✓ Improved error reporting"
echo ""
echo "Karmada will automatically propagate these to member clusters (member1, member2)"
echo ""