#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILL_DIR="${SCRIPT_DIR}/.."
LIB_DIR="${SCRIPT_DIR}/lib"
source "${LIB_DIR}/utils.sh"
source "${LIB_DIR}/argo.sh"
source "${LIB_DIR}/test-cases.sh"

PROJECT_ROOT="$(cd "${SKILL_DIR}/../../.." && pwd)"
TEST_CASES_JSON="${SKILL_DIR}/test-cases.json"

KUBECONFIG_PATH=""
TEST_SELECTION=""
SKIP_SUBMIT=false
SKIP_VALIDATE=false
SKIP_GO_TESTS=false
DRY_RUN=false

declare -A TEST_RESULTS
PASSED_COUNT=0
FAILED_COUNT=0
TOTAL_COUNT=0

display_banner() {
    echo -e "${MAGENTA}"
    echo "╔═══════════════════════════════════════════════════════════════╗"
    echo "║            Argo Workflow Test Orchestrator                    ║"
    echo "║                                                               ║"
    echo "║  1. Parse Test Cases                                          ║"
    echo "║  2. [Go Tests] (optional)                                     ║"
    echo "║  3. Submit Workflows (Async)                                  ║"
    echo "║  4. Wait & Validate                                           ║"
    echo "║  5. Cleanup                                                    ║"
    echo "╚═══════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
}

parse_args() {
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
                parse_test_cases > /dev/null 2>&1
                list_tests
                exit 0
                ;;
            --skip-submit)
                SKIP_SUBMIT=true
                shift
                ;;
            --skip-validate)
                SKIP_VALIDATE=true
                shift
                ;;
            --skip-go-tests)
                SKIP_GO_TESTS=true
                shift
                ;;
            --dry-run)
                DRY_RUN=true
                SKIP_SUBMIT=true
                SKIP_VALIDATE=true
                shift
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

    if [[ -z "$KUBECONFIG_PATH" ]]; then
        echo -e "${CYAN}[PROMPT]${NC} Please provide the kubeconfig path:"
        read -p "Enter kubeconfig path (or press Enter for default ~/.kube/config): " KUBECONFIG_PATH

        if [[ -z "$KUBECONFIG_PATH" ]]; then
            KUBECONFIG_PATH="$HOME/.kube/config"
        fi
    fi

    if [[ ! -f "$KUBECONFIG_PATH" ]]; then
        log_error "Kubeconfig file not found: ${KUBECONFIG_PATH}"
        exit 1
    fi

    export KUBECONFIG="$KUBECONFIG_PATH"
}

run_go_test() {
    local selection="$1"
    local test_output
    local temp_file

    cd "${PROJECT_ROOT}/go"

    if [[ "$selection" == "all" ]] || [[ -z "$selection" ]]; then
        test_output=$(go test -v -run Test_main ./cmd/converter 2>&1)
    else
        parse_test_cases > /dev/null 2>&1

        local regex_parts=()
        local IFS=','
        for test_name in $selection; do
            local test_dir
            test_dir=$(get_test_case_info "$test_name" "test-dir")
            if [[ -n "$test_dir" ]]; then
                local dir_name
                dir_name=$(basename "$test_dir")
                regex_parts+=("Test_main/${dir_name}")
            fi
        done

        local go_regex
        go_regex=$(IFS='|'; echo "${regex_parts[*]}")

        test_output=$(go test -v -run "$go_regex" ./cmd/converter 2>&1)
    fi

    temp_file=$(mktemp)
    echo "$test_output" > "$temp_file"
    echo "$temp_file"
}

parse_test_cases_selection() {
    local selection="$1"

    if [[ "$selection" == "all" ]] || [[ -z "$selection" ]]; then
        select_tests "all"
    else
        select_tests "$selection"
    fi
}

wait_for_pod() {
    local pattern="$1"
    local pod_var="$2"
    local timeout="$3"
    local start_time=$(date +%s)

    while true; do
        local elapsed=$(($(date +%s) - start_time))
        if [[ $elapsed -ge $timeout ]]; then
            log_error "Timeout waiting for pod matching '${pattern}' (${timeout}s)"
            return 1
        fi

        local pod_name
        pod_name=$(kubectl get pods -n "$namespace" --kubeconfig "$kubeconfig" \
            -l "workflows.argoproj.io/workflow=${workflow_name}" \
            --sort-by='.metadata.creationTimestamp' \
            -o jsonpath='{.items[-1].metadata.name}' 2>/dev/null)

        if [[ -n "$pod_name" ]] && [[ "$pod_name" == *"$pattern"* ]]; then
            eval "$pod_var='$pod_name'"
            return 0
        fi

        echo "  Waiting for pod matching '${pattern}' (${elapsed}s)..."
        sleep 2
    done
}

run_test_case() {
    local test_name="$1"
    local kubeconfig="$2"
    local result="FAIL"
    local namespace
    namespace=$(get_namespace_for_test "$test_name")

    {
        echo "═══════════════════════════════════════════════════════════════"
        log_test "Processing test: ${test_name}"
        echo "═══════════════════════════════════════════════════════════════"

        local test_dir expected_yaml expected_secret workspace cp_artifacts_temp_folder
        test_dir=$(get_test_case_info "$test_name" "test-dir")
        expected_yaml=$(get_test_case_info "$test_name" "expected-yaml")
        workspace=$(get_workspace "${PROJECT_ROOT}/${test_dir}")
        cp_artifacts_temp_folder=$(get_cp_artifacts_temp_folder "${PROJECT_ROOT}/${test_dir}")
        expected_secret=$(get_test_case_info "$test_name" "expected-secret")

        if [[ -z "$test_dir" ]] || [[ -z "$expected_yaml" ]]; then
            log_error "Missing test-dir or expected-yaml for: ${test_name}"
            return 1
        fi

        if [[ "$DRY_RUN" == "true" ]]; then
            log_info "[DRY-RUN] Would process test: ${test_name}"
            log_info "[DRY-RUN]   test_dir: ${PROJECT_ROOT}/${test_dir}"
            log_info "[DRY-RUN]   expected_yaml: ${PROJECT_ROOT}/${expected_yaml}"
            [[ -n "$expected_secret" ]] && log_info "[DRY-RUN]   expected_secret: ${PROJECT_ROOT}/${expected_secret}"
            return 0
        fi

        local workflow_file="${PROJECT_ROOT}/${expected_yaml}"
        if [[ ! -f "$workflow_file" ]]; then
            log_error "Workflow file not found: ${workflow_file}"
            return 1
        fi

        if [[ "$SKIP_SUBMIT" == "true" ]]; then
            log_info "Skipping workflow submission (--skip-submit)"
            return 0
        fi

        local workflow_exit=0
        local workflow_name
        local secret_file=""
        if [[ -n "$expected_secret" ]] && [[ -f "${PROJECT_ROOT}/${expected_secret}" ]]; then
            secret_file="${PROJECT_ROOT}/${expected_secret}"
        fi

        if workflow_name=$(cd "$PROJECT_ROOT" && argo_submit "$workflow_file" "$namespace" "$kubeconfig" "$workspace" "$cp_artifacts_temp_folder" "$secret_file"); then
            workflow_exit=0
        else
            workflow_exit=$?
        fi
        log_info "Workflow submitted: ${workflow_name}"

        if [[ $workflow_exit -ne 0 ]]; then
            log_info "Workflow execution failed (exit code ${workflow_exit}); continuing to validation"
        fi

        if [[ "$SKIP_VALIDATE" == "false" ]]; then
            local eval_script
            eval_script=$(get_test_case_info "$test_name" "validation.script")

            if [[ -n "$eval_script" ]] && [[ -f "${PROJECT_ROOT}/${test_dir}/${eval_script}" ]]; then
                log_eval "Running eval: ${eval_script}"
                if run_eval "${PROJECT_ROOT}/${test_dir}/${eval_script}" "$workflow_name" "$namespace" "$kubeconfig" "$workspace" "$cp_artifacts_temp_folder" "$workflow_exit"; then
                    result="PASS"
                else
                    result="FAIL"
                fi
            else
                result="PASS"
            fi
        else
            result="PASS"
        fi

        if [[ "$result" == "PASS" ]]; then
            log_success "PASS: ${test_name}"
            [[ -f "${PROJECT_ROOT}/${test_dir}/clean.sh" ]] && \
                bash "${PROJECT_ROOT}/${test_dir}/clean.sh" "$workflow_name" "$namespace" "$kubeconfig" "$workspace" "$cp_artifacts_temp_folder"
            log_info "Cleaning up workflow: ${workflow_name}"
            cleanup_workflow "$workflow_name" "$namespace" "$kubeconfig"
        else
            log_error "FAIL: ${test_name}"
        fi

        return $([[ "$result" == "PASS" ]] && echo 0 || echo 1)
    }
}

run_all_tests() {
    local selected_tests="$1"
    local kubeconfig="$2"
    local pids=()

    log_step "Running all tests asynchronously..."

    while IFS= read -r test_name; do
        test_name=$(echo "$test_name" | xargs)
        [[ -z "$test_name" ]] && continue

        ((TOTAL_COUNT++))

        run_test_case "$test_name" "$kubeconfig" &
        pids+=($!)

        log_info "Started test: ${test_name} (PID: $!)"

    done <<< "$selected_tests"

    echo ""
    log_step "Waiting for all tests to complete..."

    for i in "${!pids[@]}"; do
        wait ${pids[$i]}
        local exit_code=$?
        local test_name
        test_name=$(echo "$selected_tests" | sed -n "$((i+1))p" | xargs)

        if [[ $exit_code -eq 0 ]]; then
            TEST_RESULTS["$test_name"]="PASS"
            ((PASSED_COUNT++))
        else
            TEST_RESULTS["$test_name"]="FAIL"
            ((FAILED_COUNT++))
        fi
    done
}

print_summary() {
    echo ""
    echo "═══════════════════════════════════════════════════════════════"
    echo -e "${BOLD}Test Results Summary${NC}"
    echo "═══════════════════════════════════════════════════════════════"
    printf "  ${GREEN}Passed: %d${NC}\n" "$PASSED_COUNT"
    printf "  ${RED}Failed: %d${NC}\n" "$FAILED_COUNT"
    printf "  Total:   %d\n" "$TOTAL_COUNT"
    echo "═══════════════════════════════════════════════════════════════"
    echo ""
}

main() {
    display_banner

    parse_args "$@"

    cd "${PROJECT_ROOT}"
    log_info "Working directory: $(pwd)"
    log_info "Kubeconfig: ${KUBECONFIG_PATH}"

    echo ""

    log_step "Step 1: Parsing test cases..."
    if ! parse_test_cases; then
        log_error "Failed to parse test cases"
        exit 1
    fi
    log_success "Found ${#TEST_CASES[@]} test cases"

    echo ""

    if [[ "$SKIP_GO_TESTS" == "true" ]]; then
        log_info "Skipping Go tests (--skip-go-tests)"
    else
        log_step "Step 2: Running Go tests..."
        local go_test_output_file
        go_test_output_file=$(run_go_test "$TEST_SELECTION")
        if grep -q "FAIL" "$go_test_output_file" 2>/dev/null; then
            log_error "Go tests failed, aborting workflow tests"
            cat "$go_test_output_file" 2>/dev/null
            rm -f "$go_test_output_file"
            exit 1
        fi
        log_success "Go tests passed"
        rm -f "$go_test_output_file"
    fi

    echo ""

    log_step "Step 3: Selecting test cases..."
    local selected_tests
    selected_tests=$(parse_test_cases_selection "$TEST_SELECTION")
    local selected_count
    selected_count=$(echo "$selected_tests" | grep -c '^' || echo "0")
    log_success "Selected ${selected_count} test case(s)"

    echo ""

    log_step "Step 4: Running workflow tests..."
    run_all_tests "$selected_tests" "$KUBECONFIG_PATH"

    echo ""

    print_summary

    if [[ $FAILED_COUNT -gt 0 ]]; then
        log_error "Some tests failed!"
        exit 1
    else
        log_success "All tests passed!"
        exit 0
    fi
}

main "$@"
