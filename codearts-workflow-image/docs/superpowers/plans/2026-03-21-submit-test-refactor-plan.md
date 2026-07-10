# Argo Workflow Test - Framework Refactor + PVC OwnerReference Fix

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Refactor submit-test from messy multi-file structure to 4 clean files, and fix PVC ownerReference flow to use proper submit → UID → PVC → wait → (if fail: delete PVC) → (if success: copy pod) → delete PVC → eval sequence.

**Architecture:** Restructure bash scripts into 4 focused files: main.sh (orchestrator), lib/utils.sh (logging), lib/argo.sh (k8s operations), lib/test-cases.sh (test case parsing). Remove workflow-submitter.sh and result-validator.sh as dead code. Implement async test execution with proper error handling. Use local variables instead of global env vars to prevent pollution.

**Tech Stack:** Bash, kubectl, argo CLI

---

## File Structure

```
.opencode/skills/submit-test/scripts/
├── main.sh           # Orchestrator: parse args, run go test, dispatch async
└── lib/
    ├── utils.sh      # Logging, colors, common helpers
    ├── argo.sh       # Argo operations (submit, wait, PVC, eval)
    └── test-cases.sh # Test case parsing (refactored)
```

**Files Removed:**
- `workflow-submitter.sh` - dead code
- `result-validator.sh` - dead code

---

## Chunk 1: Create lib/utils.sh

**Files:**
- Create: `.opencode/skills/submit-test/scripts/lib/utils.sh`

- [ ] **Step 1: Create lib directory**

```bash
mkdir -p .opencode/skills/submit-test/scripts/lib
```

- [ ] **Step 2: Create lib/utils.sh with colors and logging**

```bash
cat > .opencode/skills/submit-test/scripts/lib/utils.sh << 'EOF'
#!/bin/bash

# Shared utilities for submit-test

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
BOLD='\033[1m'
NC='\033[0m'

# Log functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_step() {
    echo -e "${CYAN}[STEP]${NC} $1"
}

log_test() {
    echo -e "${YELLOW}[TEST]${NC} $1"
}

log_eval() {
    echo -e "${MAGENTA}[EVAL]${NC} $1"
}

# Get timestamp in ISO format
get_timestamp() {
    date -u +"%Y-%m-%dT%H:%M:%SZ"
}

# Validate kubeconfig exists
validate_kubeconfig() {
    local kubeconfig="$1"
    if [ -z "$kubeconfig" ]; then
        log_error "Kubeconfig path required"
        return 1
    fi
    if [ ! -f "$kubeconfig" ]; then
        log_error "Kubeconfig file not found: ${kubeconfig}"
        return 1
    fi
    return 0
}

# Display usage
usage() {
    echo -e "${BOLD}Usage:${NC}"
    echo "  $0 -k <kubeconfig> -t <tests> [OPTIONS]"
    echo ""
    echo -e "${BOLD}Options:${NC}"
    echo "  -k, --kubeconfig    Path to kubeconfig file (required)"
    echo "  -t, --tests         Test cases (comma-separated, or 'all')"
    echo "  --list-tests        List available test cases"
    echo "  -h, --help          Show this help message"
    echo ""
    echo -e "${BOLD}Examples:${NC}"
    echo "  $0 -k /path/to/kubeconfig -t all"
    echo "  $0 -k /path/to/kubeconfig -t simple,with-secrets"
}
EOF
```

- [ ] **Step 3: Verify file created**

```bash
ls -la .opencode/skills/submit-test/scripts/lib/utils.sh
```

---

## Chunk 2: Create lib/argo.sh

**Files:**
- Create: `.opencode/skills/submit-test/scripts/lib/argo.sh`

- [ ] **Step 1: Create lib/argo.sh with Argo operations**

```bash
cat > .opencode/skills/submit-test/scripts/lib/argo.sh << 'EOF'
#!/bin/bash

# Argo workflow operations for submit-test

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "${SCRIPT_DIR}")/../../.."

source "${SCRIPT_DIR}/utils.sh"

# Submit workflow and return name
argo_submit() {
    local workflow_file="$1"
    local namespace="${2:-argo}"
    local kubeconfig="$3"

    log_info "Submitting workflow: ${workflow_file}"

    local output
    output=$(argo submit "$workflow_file" \
        --kubeconfig "$kubeconfig" \
        -n "$namespace" \
        -o name 2>&1)

    if [ $? -ne 0 ]; then
        log_error "Workflow submission failed: ${output}"
        return 1
    fi

    local workflow_name
    workflow_name=$(echo "$output" | grep -E '^[a-z0-9]+(-[a-z0-9]+)+$' | tail -1)
    if [ -z "$workflow_name" ]; then
        workflow_name=$(echo "$output" | tr -d ' \n')
    fi

    echo "$workflow_name"
}

# Get workflow UID
argo_get_uid() {
    local workflow_name="$1"
    local namespace="${2:-argo}"
    local kubeconfig="$3"

    local uid
    uid=$(kubectl get workflow "$workflow_name" -n "$namespace" \
        --kubeconfig "$kubeconfig" \
        -o jsonpath='{.metadata.uid}' 2>/dev/null)

    if [ -z "$uid" ]; then
        log_error "Could not get UID for workflow: ${workflow_name}"
        return 1
    fi

    echo "$uid"
}

# Wait for workflow completion
argo_wait() {
    local workflow_name="$1"
    local namespace="${2:-argo}"
    local kubeconfig="$3"
    local max_wait="${4:-300}"
    local interval=10
    local elapsed=0

    local status
    status=$(argo get "$workflow_name" -n "$namespace" \
        --kubeconfig "$kubeconfig" 2>/dev/null | \
        grep Status | awk '{print $2}')

    while [[ "$status" == "Running" || "$status" == "Pending" ]]; do
        sleep $interval
        elapsed=$((elapsed + interval))
        if [ $elapsed -ge $max_wait ]; then
            log_error "Workflow timed out after ${max_wait}s"
            return 1
        fi
        status=$(argo get "$workflow_name" -n "$namespace" \
            --kubeconfig "$kubeconfig" 2>/dev/null | \
            grep Status | awk '{print $2}')
    done

    echo "$status"
}

# Delete workflow
argo_delete() {
    local workflow_name="$1"
    local namespace="${2:-argo}"
    local kubeconfig="$3"

    argo delete "$workflow_name" -n "$namespace" \
        --kubeconfig "$kubeconfig" 2>/dev/null || true
}

# Apply secret if exists
apply_secret() {
    local secret_file="$1"
    local namespace="${2:-argo}"
    local kubeconfig="$3"

    if [ ! -f "$secret_file" ]; then
        return 0
    fi

    log_info "Applying secret: ${secret_file}"
    kubectl apply -f "$secret_file" --kubeconfig "$kubeconfig" -n "$namespace" 2>&1

    if [ $? -ne 0 ]; then
        log_error "Failed to apply secret"
        return 1
    fi

    log_success "Secret applied"
    return 0
}

# Apply PVC with ownerReference
apply_pvc() {
    local pvc_file="$1"
    local uid="$2"
    local namespace="${3:-argo}"
    local kubeconfig="$4"

    if [ ! -f "$pvc_file" ]; then
        return 0
    fi

    local pvc_name
    pvc_name=$(grep "^  name:" "$pvc_file" | awk '{print $2}')

    log_info "Applying PVC: ${pvc_name} with ownerRef UID: ${uid}"

    kubectl apply -f "$pvc_file" --kubeconfig "$kubeconfig" -n "$namespace" 2>&1

    if [ $? -ne 0 ]; then
        log_error "Failed to apply PVC"
        return 1
    fi

    kubectl patch pvc "$pvc_name" -n "$namespace" \
        --kubeconfig "$kubeconfig" \
        --type='json' \
        -p="[{\"op\":\"add\",\"path\":\"/metadata/ownerReferences\",\"value\":[{\"apiVersion\":\"argoproj.io/v1alpha1\",\"kind\":\"Workflow\",\"name\":\"${workflow_name}\",\"uid\":\"${uid}\"}]}]" 2>/dev/null

    log_success "PVC applied with ownerReference"
    return 0
}

# Delete PVC
delete_pvc() {
    local pvc_name="$1"
    local namespace="${2:-argo}"
    local kubeconfig="$3"

    kubectl delete pvc "$pvc_name" -n "$namespace" \
        --kubeconfig "$kubeconfig" 2>/dev/null || true
}

# Run eval script
run_eval() {
    local eval_script="$1"
    local workflow_name="$2"
    local namespace="$3"
    local kubeconfig="$4"
    shift 4
    local extra_params="$@"

    if [ ! -f "$eval_script" ]; then
        log_error "Eval script not found: ${eval_script}"
        return 1
    fi

    log_eval "Running eval: ${eval_script}"

    bash "$eval_script" "$workflow_name" "$namespace" "$kubeconfig" $extra_params
    return $?
}
EOF
```

- [ ] **Step 2: Verify file created**

```bash
ls -la .opencode/skills/submit-test/scripts/lib/argo.sh
```

---

## Chunk 3: Create lib/test-cases.sh

**Files:**
- Create: `.opencode/skills/submit-test/scripts/lib/test-cases.sh`

- [ ] **Step 1: Create lib/test-cases.sh (refactored from existing)**

```bash
cat > .opencode/skills/submit-test/scripts/lib/test-cases.sh << 'EOF'
#!/bin/bash

# Test case parsing for submit-test

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "${SCRIPT_DIR}")/../../.."
TEST_CASES_JSON="${PROJECT_ROOT}/.opencode/skills/submit-test/test-cases.json"

source "${SCRIPT_DIR}/utils.sh"

# Global array of test case names
TEST_CASES=()

# Parse test cases from JSON
parse_test_cases() {
    if [ ! -f "$TEST_CASES_JSON" ]; then
        log_error "Test cases configuration not found: ${TEST_CASES_JSON}"
        return 1
    fi

    TEST_CASES=()
    while IFS= read -r test_name; do
        TEST_CASES+=("$test_name")
    done < <(jq -r 'keys[]' "$TEST_CASES_JSON" 2>/dev/null)

    if [ ${#TEST_CASES[@]} -eq 0 ]; then
        log_error "No test cases found"
        return 1
    fi

    log_success "Found ${#TEST_CASES[@]} test cases"
}

# Get test case info field
get_test_case_info() {
    local test_name="$1"
    local field="$2"

    if [ -z "$test_name" ] || [ -z "$field" ]; then
        log_error "Test name and field required"
        return 1
    fi

    local jq_query
    if [[ "$field" == *"."* ]]; then
        jq_query=".[\"${test_name}\"]"
        IFS='.' read -ra parts <<< "$field"
        for part in "${parts[@]}"; do
            jq_query="${jq_query}.${part}"
        done
    else
        jq_query=".[\"${test_name}\"].${field}"
    fi

    jq_query="${jq_query} // empty"

    local value
    value=$(jq -r "$jq_query" "$TEST_CASES_JSON" 2>/dev/null)

    if [ -z "$value" ] || [ "$value" = "null" ]; then
        return 1
    fi

    echo "$value"
}

# Select tests based on selection string
select_tests() {
    local selection="$1"

    if [ -z "$selection" ] || [ "$selection" = "all" ]; then
        printf '%s\n' "${TEST_CASES[@]}"
        return
    fi

    IFS=',' read -ra selected <<< "$selection"
    for sel in "${selected[@]}"; do
        sel=$(echo "$sel" | xargs)
        for tc in "${TEST_CASES[@]}"; do
            if [ "$tc" = "$sel" ]; then
                echo "$tc"
            fi
        done
    done
}

# List available tests
list_tests() {
    if [ ${#TEST_CASES[@]} -eq 0 ]; then
        log_error "No test cases available"
        return 1
    fi

    echo ""
    echo -e "${YELLOW}Available Test Cases:${NC}"
    echo "═══════════════════════════════════════════════════════════════"

    local count=1
    for test_name in "${TEST_CASES[@]}"; do
        printf "  %2d. %s\n" "$count" "$test_name"
        ((count++))
    done

    echo ""
    echo "═══════════════════════════════════════════════════════════════"
}
EOF
```

- [ ] **Step 2: Verify file created**

```bash
ls -la .opencode/skills/submit-test/scripts/lib/test-cases.sh
```

---

## Chunk 4: Create main.sh (Orchestrator)

**Files:**
- Create: `.opencode/skills/submit-test/scripts/main.sh`
- Modify: `go/cmd/converter/case/newtest/test10-cp-artifacts/expected-artifact-pvc.yaml` (remove ${WORKFLOW_UID} placeholder, use UID directly)

- [ ] **Step 1: Create main.sh**

```bash
cat > .opencode/skills/submit-test/scripts/main.sh << 'EOF'
#!/bin/bash

# Argo Workflow Test - Main Orchestrator

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "${SCRIPT_DIR}")/../../.."

source "${SCRIPT_DIR}/lib/utils.sh"
source "${SCRIPT_DIR}/lib/argo.sh"
source "${SCRIPT_DIR}/lib/test-cases.sh"

# Display banner
display_banner() {
    echo -e "${MAGENTA}"
    echo "╔═══════════════════════════════════════════════════════════════╗"
    echo "║          Argo Workflow Test Skill (Refactored)              ║"
    echo "║                                                               ║"
    echo "║  Step 1: Run go test                                          ║"
    echo "║  Step 2: Parse test cases                                    ║"
    echo "║  Step 3: Submit → UID → PVC → Wait → Delete → Eval          ║"
    echo "╚═══════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

# Parse CLI arguments
parse_args() {
    KUBECONFIG_PATH=""
    TEST_SELECTION=""

    while [[ $# -gt 0 ]]; do
        case "$1" in
            -k|--kubeconfig)
                KUBECONFIG_PATH="$2"
                shift 2
                ;;
            -t|--tests)
                TEST_SELECTION="$2"
                shift 2
                ;;
            --list-tests)
                parse_test_cases
                list_tests
                exit 0
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done

    if [ -z "$KUBECONFIG_PATH" ]; then
        echo -e "${CYAN}[PROMPT]${NC} Please provide kubeconfig path:"
        read -p "Enter kubeconfig (default: ~/.kube/config): " KUBECONFIG_PATH
        KUBECONFIG_PATH="${KUBECONFIG_PATH:-$HOME/.kube/config}"
    fi

    validate_kubeconfig "$KUBECONFIG_PATH" || exit 1

    if [ -z "$TEST_SELECTION" ]; then
        TEST_SELECTION="all"
        log_info "No tests specified, running all tests"
    fi
}

# Run go test for selection
run_go_test() {
    local selection="$1"

    log_step "Step 1/3: Running go test..."

    cd "${PROJECT_ROOT}/go/cmd/converter"

    if [ "$selection" = "all" ]; then
        log_info "Running all Go tests..."
        go test -v -run "Test_main" 2>&1
    else
        IFS=',' read -ra selected <<< "$selection"
        for sel in "${selected[@]}"; do
            sel=$(echo "$sel" | xargs)
            log_info "Running go test for: ${sel}"
            go test -v -run "Test_main/${sel}" 2>&1
        done
    fi

    if [ $? -ne 0 ]; then
        log_error "Go tests failed"
        exit 1
    fi

    cd - > /dev/null
    log_success "Go tests passed"
}

# Parse test cases
parse_test_cases_selection() {
    local selection="$1"

    log_step "Step 2/3: Parsing test cases..."

    parse_test_cases

    local selected_tests
    selected_tests=$(select_tests "$selection")

    local count=$(echo "$selected_tests" | grep -c '^' || echo "0")
    log_success "Selected ${count} test case(s)"
    echo "$selected_tests"
}

# Run single test case (async function)
run_test_case() {
    local test_name="$1"
    local kubeconfig="$2"

    log_test "Processing test: ${test_name}"

    local test_dir expected_yaml expected_secret expected_artifact_pvc eval_script
    test_dir=$(get_test_case_info "$test_name" "test-dir")
    expected_yaml=$(get_test_case_info "$test_name" "expected-yaml")
    expected_secret=$(get_test_case_info "$test_name" "expected-secret")
    expected_artifact_pvc=$(get_test_case_info "$test_name" "expected-artifact-pvc")
    eval_script=$(get_test_case_info "$test_name" "validation.script")

    local workflow_file="${PROJECT_ROOT}/${expected_yaml}"
    local secret_file=""
    if [ -n "$expected_secret" ] && [ "$expected_secret" != "null" ]; then
        secret_file="${PROJECT_ROOT}/${test_dir}/$(basename "$expected_secret")"
    fi

    # 1. Apply secret if exists
    if [ -n "$secret_file" ] && [ -f "$secret_file" ]; then
        apply_secret "$secret_file" "argo" "$kubeconfig" || return 1
    fi

    # 2. Submit workflow
    local workflow_name
    workflow_name=$(argo_submit "$workflow_file" "argo" "$kubeconfig") || return 1

    log_info "Workflow submitted: ${workflow_name}"

    # 3. Get UID
    local uid
    uid=$(argo_get_uid "$workflow_name" "argo" "$kubeconfig") || return 1

    # 4. Apply PVC if exists
    if [ -n "$expected_artifact_pvc" ] && [ "$expected_artifact_pvc" != "null" ]; then
        local pvc_file="${PROJECT_ROOT}/${test_dir}/${expected_artifact_pvc}"
        apply_pvc "$pvc_file" "$uid" "argo" "$kubeconfig" "$workflow_name" || return 1
    fi

    # 5. Wait for completion
    local status
    status=$(argo_wait "$workflow_name" "argo" "$kubeconfig") || {
        log_error "Workflow ${workflow_name} failed or timed out"
        argo_delete "$workflow_name" "argo" "$kubeconfig"
        return 1
    }

    log_info "Workflow ${workflow_name} completed with status: ${status}"

    # 6. Delete PVC if exists
    if [ -n "$expected_artifact_pvc" ] && [ "$expected_artifact_pvc" != "null" ]; then
        local pvc_file="${PROJECT_ROOT}/${test_dir}/${expected_artifact_pvc}"
        local pvc_name=$(grep "^  name:" "$pvc_file" | awk '{print $2}')
        delete_pvc "$pvc_name" "argo" "$kubeconfig"
    fi

    # 7. Delete workflow
    argo_delete "$workflow_name" "argo" "$kubeconfig"

    # 8. Run eval
    local eval_script_path="${PROJECT_ROOT}/${test_dir}/${eval_script}"
    run_eval "$eval_script_path" "$workflow_name" "argo" "$kubeconfig" || {
        log_error "Eval failed for ${test_name}"
        return 1
    }

    log_success "PASS: ${test_name}"
    return 0
}

# Run all selected tests asynchronously
run_all_tests() {
    local selected_tests="$1"
    local kubeconfig="$2"

    log_step "Step 3/3: Running tests (async)..."

    local pids=()
    local test_names=()
    local temp_dir=$(mktemp -d)
    local passed=0
    local failed=0

    while IFS= read -r test_name; do
        [ -z "$test_name" ] && continue

        run_test_case "$test_name" "$kubeconfig" > "${temp_dir}/${test_name}.log" 2>&1 &
        pids+=($!)
        test_names+=("$test_name")

        log_info "Started ${test_name} (PID: $!)"
    done <<< "$selected_tests"

    log_info "Waiting for ${#pids[@]} tests to complete..."

    for i in "${!pids[@]}"; do
        local pid="${pids[$i]}"
        local test_name="${test_names[$i]}"

        if wait $pid; then
            ((passed++))
            log_success "${test_name}: PASS"
        else
            ((failed++))
            log_error "${test_name}: FAIL"
            log_error "See log: ${temp_dir}/${test_name}.log"
        fi
    done

    echo ""
    echo "═══════════════════════════════════════════════════════════════"
    echo -e "${BOLD}Final Results${NC}"
    echo "═══════════════════════════════════════════════════════════════"
    echo -e "  ${GREEN}Passed: ${passed}${NC}"
    echo -e "  ${RED}Failed: ${failed}${NC}"
    echo -e "  Total:   $((passed + failed))"
    echo "═══════════════════════════════════════════════════════════════"

    rm -rf "$temp_dir"

    if [ $failed -gt 0 ]; then
        return 1
    fi
    return 0
}

# Main
main() {
    display_banner

    parse_args "$@"

    echo ""

    run_go_test "$TEST_SELECTION"

    local selected_tests
    selected_tests=$(parse_test_cases_selection "$TEST_SELECTION")

    run_all_tests "$selected_tests" "$KUBECONFIG_PATH"

    if [ $? -eq 0 ]; then
        log_success "All tests passed!"
        exit 0
    else
        log_error "Some tests failed!"
        exit 1
    fi
}

main "$@"
EOF
```

- [ ] **Step 2: Make main.sh executable**

```bash
chmod +x .opencode/skills/submit-test/scripts/main.sh
```

---

## Chunk 5: Remove Dead Code

**Files:**
- Remove: `.opencode/skills/submit-test/scripts/workflow-submitter.sh`
- Remove: `.opencode/skills/submit-test/scripts/result-validator.sh`

- [ ] **Step 1: Remove dead code files**

```bash
rm -f .opencode/skills/submit-test/scripts/workflow-submitter.sh
rm -f .opencode/skills/submit-test/scripts/result-validator.sh
```

- [ ] **Step 2: Verify removal**

```bash
ls -la .opencode/skills/submit-test/scripts/
```

---

## Chunk 6: Commit Changes

- [ ] **Step 1: Stage and commit**

```bash
git add -A .opencode/skills/submit-test/scripts/
git status
```

- [ ] **Step 2: Commit with descriptive message**

```bash
git commit -m "refactor: restructure submit-test into 4 clean files

- Create lib/utils.sh, lib/argo.sh, lib/test-cases.sh
- Rewrite main.sh with correct PVC ownerRef flow
- Remove workflow-submitter.sh and result-validator.sh (dead code)
- Flow: submit → UID → PVC → wait → delete → eval (async)
- CLI flags preserved: -k, -t, --list-tests, -h"
```

---

## Verification

After implementation, verify:

1. List tests: `./main.sh -k <kubeconfig> --list-tests`
2. Run single test: `./main.sh -k <kubeconfig> -t simple`
3. Run all tests: `./main.sh -k <kubeconfig> -t all`

Expected directory structure:
```
scripts/
├── main.sh
└── lib/
    ├── utils.sh
    ├── argo.sh
    └── test-cases.sh
```
