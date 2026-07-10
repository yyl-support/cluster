#!/bin/bash
# Apply/Update PVC configuration to target cluster with specific context
# Usage: ./apply-pvc.sh [--kubeconfig KUBECONFIG_PATH] [--context CONTEXT_NAME] [--dry-run]

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CONFIG_FILE="${SCRIPT_DIR}/pvc-config.yaml"
DEFAULT_KUBECONFIG=~/.kube/a-merge-cluster
DEFAULT_CONTEXT=""

KUBECONFIG_PATH="$DEFAULT_KUBECONFIG"
CONTEXT_NAME=""
DRY_RUN=false

# Parse arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --kubeconfig)
            KUBECONFIG_PATH="$2"
            shift 2
            ;;
        --context)
            CONTEXT_NAME="$2"
            shift 2
            ;;
        --dry-run)
            DRY_RUN=true
            shift
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --kubeconfig PATH   Path to kubeconfig file (default: ~/.kube/a-merge-cluster)"
            echo "  --context NAME      Kubernetes context to apply to (default: all contexts in config)"
            echo "  --dry-run           Show what would be applied without making changes"
            echo "  --help, -h          Show this help message"
            echo ""
            echo "PVC records are loaded from: pvc-config.yaml"
            exit 0
            ;;
        *)
            KUBECONFIG_PATH="$1"
            shift
            ;;
    esac
done

if [[ ! -f "$KUBECONFIG_PATH" ]]; then
    echo "ERROR: Kubeconfig not found: $KUBECONFIG_PATH"
    exit 1
fi

if ! command -v yq &> /dev/null; then
    echo "ERROR: yq is required but not installed"
    echo "Install: pip install yq or https://github.com/kislyuk/yq"
    exit 1
fi

export KUBECONFIG="$KUBECONFIG_PATH"

echo "=========================================="
echo "PVC Apply Script"
echo "=========================================="
echo "Target kubeconfig: $KUBECONFIG_PATH"
if [[ "$DRY_RUN" == "true" ]]; then
    echo "Mode: DRY-RUN (no changes will be made)"
fi

# Get current context if not specified
if [[ -z "$CONTEXT_NAME" ]]; then
    CONTEXT_NAME=$(kubectl config current-context)
fi

echo "Target context: $CONTEXT_NAME"
echo ""

apply_pvc() {
    local name="$1"
    local storage="$2"
    local storage_class="$3"
    local namespace="$4"
    local target_context="$5"
    
    echo "Applying PVC: $name"
    echo "  Storage: $storage"
    echo "  StorageClass: $storage_class"
    echo "  Namespace: $namespace"
    echo "  Context: $target_context"
    
    if [[ "$DRY_RUN" == "true" ]]; then
        echo "  [DRY-RUN] Would apply PVC:"
        cat <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
    name: $name
spec:
    accessModes:
        - ReadWriteMany
    resources:
        requests:
            storage: $storage
    storageClassName: $storage_class
EOF
        echo "  PASS: PVC $name validated (dry-run)"
    else
        kubectl apply -n "$namespace" --context "$target_context" -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
    name: $name
spec:
    accessModes:
        - ReadWriteMany
    resources:
        requests:
            storage: $storage
    storageClassName: $storage_class
EOF
        
        if [[ $? -eq 0 ]]; then
            echo "  PASS: PVC $name applied successfully"
        else
            echo "  FAIL: PVC $name apply failed"
            return 1
        fi
    fi
    echo ""
}

# Load PVC records from pvc-config.yaml
if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "ERROR: Config file not found: $CONFIG_FILE"
    exit 1
fi

pvc_count=$(yq '.pvc_records | length' "$CONFIG_FILE")
echo "Loaded $pvc_count PVC records from $CONFIG_FILE"
echo ""

for ((i=0; i<pvc_count; i++)); do
    name=$(yq -r ".pvc_records[$i].name" "$CONFIG_FILE")
    storage=$(yq -r ".pvc_records[$i].storage" "$CONFIG_FILE")
    storage_class=$(yq -r ".pvc_records[$i].storageClassName" "$CONFIG_FILE")
    namespace=$(yq -r ".pvc_records[$i].namespace" "$CONFIG_FILE")
    pvc_context=$(yq -r ".pvc_records[$i].context" "$CONFIG_FILE")
    
    # Skip if context filter doesn't match
    if [[ -n "$CONTEXT_NAME" ]]; then
        if [[ "$pvc_context" != "$CONTEXT_NAME" ]]; then
            echo "Skipping $name (context mismatch: $pvc_context != $CONTEXT_NAME)"
            continue
        fi
    fi
    
    apply_pvc "$name" "$storage" "$storage_class" "$namespace" "$pvc_context"
done

if [[ "$DRY_RUN" == "true" ]]; then
    echo "=========================================="
    echo "[DRY-RUN] Skipped verification"
    echo "=========================================="
else
    echo "=========================================="
    echo "Verifying PVCs..."
    echo "=========================================="
    
    if [[ -n "$CONTEXT_NAME" ]]; then
        kubectl get pvc -n argo --context "$CONTEXT_NAME" -o custom-columns=NAME:.metadata.name,STORAGE:.spec.resources.requests.storage,STATUS:.status.phase,STORAGECLASS:.spec.storageClassName
    else
        echo "PVCs per context:"
        context_count=$(yq '.contexts | length' "$CONFIG_FILE")
        for ((i=0; i<context_count; i++)); do
            ctx=$(yq -r ".contexts[$i].name" "$CONFIG_FILE")
            echo ""
            echo "Context: $ctx"
            kubectl get pvc -n argo --context "$ctx" -o custom-columns=NAME:.metadata.name,STORAGE:.spec.resources.requests.storage,STATUS:.status.phase 2>/dev/null || echo "  No PVCs found"
        done
    fi
fi

echo ""
echo "=========================================="
echo "All PVCs applied successfully"
echo "=========================================="