# Load PVC from YAML Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Modify apply-pvc.sh to load PVC records from pvc-config.yaml instead of hardcoded values.

**Architecture:** Replace hardcoded `pvc_records` array with yq-based YAML parsing. Parse each PVC record and apply to the appropriate Kubernetes context.

**Tech Stack:** Bash, yq, kubectl

---

### Task 1: Check yq dependency and add validation

**Files:**
- Modify: `go/cmd/converter/pvcrecord/apply-pvc.sh`

- [ ] **Step 1: Add yq dependency check after kubeconfig validation**

Insert after line 36 (after kubeconfig check):

```bash
if ! command -v yq &> /dev/null; then
    echo "ERROR: yq is required but not installed"
    echo "Install: https://github.com/mikefarah/yq/#install"
    exit 1
fi
```

- [ ] **Step 2: Test yq check works**

Run: `./go/cmd/converter/pvcrecord/apply-pvc.sh --help 2>&1 | head -5`
Expected: Script continues or shows usage (no yq error if yq installed)

- [ ] **Step 3: Commit**

```bash
git add go/cmd/converter/pvcrecord/apply-pvc.sh
git commit -m "feat(pvc): add yq dependency check"
```

---

### Task 2: Replace hardcoded pvc_records with YAML parsing

**Files:**
- Modify: `go/cmd/converter/pvcrecord/apply-pvc.sh`

- [ ] **Step 1: Remove hardcoded pvc_records array and replace with YAML loading**

Replace lines 89-95 (the hardcoded pvc_records section) with:

```bash
# Load PVC records from pvc-config.yaml
if [[ ! -f "$CONFIG_FILE" ]]; then
    echo "ERROR: Config file not found: $CONFIG_FILE"
    exit 1
fi

pvc_count=$(yq eval '.pvc_records | length' "$CONFIG_FILE")
echo "Loaded $pvc_count PVC records from $CONFIG_FILE"
echo ""
```

- [ ] **Step 2: Update the for loop to iterate over YAML data**

Replace lines 100-113 (the for loop) with:

```bash
for ((i=0; i<pvc_count; i++)); do
    name=$(yq eval ".pvc_records[$i].name" "$CONFIG_FILE")
    storage=$(yq eval ".pvc_records[$i].storage" "$CONFIG_FILE")
    storage_class=$(yq eval ".pvc_records[$i].storageClassName" "$CONFIG_FILE")
    namespace=$(yq eval ".pvc_records[$i].namespace" "$CONFIG_FILE")
    pvc_context=$(yq eval ".pvc_records[$i].context" "$CONFIG_FILE")
    
    # Skip if context filter doesn't match
    if [[ -n "$CONTEXT_NAME" ]]; then
        if [[ "$pvc_context" != "$CONTEXT_NAME" ]]; then
            echo "Skipping $name (context mismatch: $pvc_context != $CONTEXT_NAME)"
            continue
        fi
    fi
    
    apply_pvc "$name" "$storage" "$storage_class" "$namespace" "$pvc_context"
done
```

- [ ] **Step 3: Test script loads YAML correctly**

Run: `cd go/cmd/converter/pvcrecord && ./apply-pvc.sh --kubeconfig ~/.kube/config 2>&1 | head -20`
Expected: Shows "Loaded 6 PVC records from ..." (or actual count from YAML)

- [ ] **Step 4: Commit**

```bash
git add go/cmd/converter/pvcrecord/apply-pvc.sh
git commit -m "feat(pvc): load PVC records from pvc-config.yaml"
```

---

### Task 3: Add dry-run mode for safe testing

**Files:**
- Modify: `go/cmd/converter/pvcrecord/apply-pvc.sh`

- [ ] **Step 1: Add dry-run flag parsing**

After line 25 (in the argument parsing while loop), add:

```bash
        --dry-run)
            DRY_RUN=true
            shift
            ;;
```

Add after line 13 (variable declarations):

```bash
DRY_RUN=false
```

- [ ] **Step 2: Modify apply_pvc function to support dry-run**

Replace line 66 (the kubectl apply line) with:

```bash
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
    fi
```

- [ ] **Step 3: Test dry-run mode**

Run: `cd go/cmd/converter/pvcrecord && ./apply-pvc.sh --dry-run --kubeconfig ~/.kube/config 2>&1 | head -30`
Expected: Shows "[DRY-RUN] Would apply PVC:" for each PVC, no actual kubectl commands

- [ ] **Step 4: Commit**

```bash
git add go/cmd/converter/pvcrecord/apply-pvc.sh
git commit -m "feat(pvc): add --dry-run flag for safe testing"
```

---

### Task 4: Update help text and usage

**Files:**
- Modify: `go/cmd/converter/pvcrecord/apply-pvc.sh`

- [ ] **Step 1: Update usage comment at top of script**

Replace line 3 with:

```bash
# Usage: ./apply-pvc.sh [--kubeconfig KUBECONFIG_PATH] [--context CONTEXT_NAME] [--dry-run]
```

- [ ] **Step 2: Add help function**

Insert after line 30 (after argument parsing section):

```bash
show_help() {
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  --kubeconfig PATH   Path to kubeconfig file (default: ~/.kube/a-merge-cluster)"
    echo "  --context NAME      Kubernetes context to apply to (default: all contexts in config)"
    echo "  --dry-run           Show what would be applied without making changes"
    echo "  --help              Show this help message"
    echo ""
    echo "PVC records are loaded from: pvc-config.yaml"
}

if [[ "$1" == "--help" ]] || [[ "$1" == "-h" ]]; then
    show_help
    exit 0
fi
```

- [ ] **Step 3: Test help output**

Run: `cd go/cmd/converter/pvcrecord && ./apply-pvc.sh --help`
Expected: Shows usage information with all options

- [ ] **Step 4: Commit**

```bash
git add go/cmd/converter/pvcrecord/apply-pvc.sh
git commit -m "docs(pvc): update usage and add help text"
```