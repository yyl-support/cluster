#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
source "${SCRIPT_DIR}/utils.sh"

PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../../.." && pwd)"

get_namespace_from_workflow() {
    local workflow_file="$1"
    cd "$PROJECT_ROOT/go" && go run ./cmd/ns --from-workflow "$workflow_file"
}

TEST_CASES=()
TEST_CASES_JSON="${SCRIPT_DIR}/../../test-cases.json"

parse_test_cases() {
    if [[ ! -f "$TEST_CASES_JSON" ]]; then
        log_error "Test cases configuration file not found: ${TEST_CASES_JSON}"
        return 1
    fi
    
    TEST_CASES=()
    while IFS= read -r test_name; do
        TEST_CASES+=("$test_name")
    done < <(jq -r 'keys[]' "$TEST_CASES_JSON" 2>/dev/null)
    
    if [[ ${#TEST_CASES[@]} -eq 0 ]]; then
        log_error "No test cases found in configuration"
        return 1
    fi
    
    log_success "Found ${#TEST_CASES[@]} test cases"
}

get_test_case_info() {
    local test_name="$1"
    local field="$2"
    
    if [[ -z "$test_name" ]] || [[ -z "$field" ]]; then
        log_error "Test name and field required"
        return 1
    fi
    
    if [[ ! -f "$TEST_CASES_JSON" ]]; then
        log_error "Test cases configuration file not loaded"
        return 1
    fi
    
    local jq_query=".[\"${test_name}\"]"
    
    if [[ "$field" == *"."* ]]; then
        IFS='.' read -ra path_parts <<< "$field"
        for part in "${path_parts[@]}"; do
            jq_query="${jq_query}[\"${part}\"]"
        done
    else
        jq_query="${jq_query}[\"${field}\"]"
    fi
    
    jq_query="${jq_query} // empty"
    
    local value
    value=$(jq -r "$jq_query" "$TEST_CASES_JSON" 2>/dev/null)
    
    if [[ -z "$value" ]] || [[ "$value" == "null" ]]; then
        return 1
    fi
    
    echo "$value"
}

select_tests() {
    local selection="$1"
    
    if [[ "$selection" == "all" ]]; then
        printf '%s\n' "${TEST_CASES[@]}"
        return 0
    fi
    
    local IFS=','
    for test_name in $selection; do
        for available in "${TEST_CASES[@]}"; do
            if [[ "$available" == "$test_name" ]]; then
                echo "$available"
                break
            fi
        done
    done
}

list_tests() {
    if [[ ${#TEST_CASES[@]} -eq 0 ]]; then
        log_error "No test cases available"
        return 1
    fi
    
    echo ""
    echo -e "${BOLD}Available Test Cases:${NC}"
    echo "═══════════════════════════════════════════════════════════════"
    echo ""
    
    local count=1
    for test_name in "${TEST_CASES[@]}"; do
        printf "  %2d. %s\n" "$count" "$test_name"
        ((count++))
    done
    
    echo ""
    echo "═══════════════════════════════════════════════════════════════"
    echo ""
}

get_namespace_for_test() {
    local test_name="$1"
    
    local expected_yaml
    expected_yaml=$(get_test_case_info "$test_name" "expected-yaml")
    
    if [[ -z "$expected_yaml" ]]; then
        echo "argo"
        return 0
    fi
    
    local workflow_file="${PROJECT_ROOT}/${expected_yaml}"
    
    if [[ ! -f "$workflow_file" ]]; then
        echo "argo"
        return 0
    fi
    
    get_namespace_from_workflow "$workflow_file"
}

get_cp_artifacts_temp_folder() {
    local test_dir="$1"
    local env_file="${test_dir}/env.sh"
    
    if [[ ! -f "$env_file" ]]; then
        echo ""
        return 0
    fi
    
    local cp_artifacts_temp_folder
    cp_artifacts_temp_folder=$(grep "^export CP_artifacts_temp_folder=" "$env_file" 2>/dev/null | sed 's/export CP_artifacts_temp_folder=//' | sed 's/"//g' | sed 's/'"'"'//g')
    
    if [[ -z "$cp_artifacts_temp_folder" ]]; then
        local cp_artifacts
        cp_artifacts=$(grep "^export CP_artifacts=" "$env_file" 2>/dev/null | sed 's/export CP_artifacts=//' | sed 's/"//g' | sed 's/'"'"'//g')
        if [[ -n "$cp_artifacts" ]]; then
            cp_artifacts_temp_folder="/output"
        fi
    fi
    
    echo "$cp_artifacts_temp_folder"
}

get_workspace() {
    local test_dir="$1"
    local env_file="${test_dir}/env.sh"
    
    if [[ ! -f "$env_file" ]]; then
        echo ""
        return 0
    fi
    
    local workspace
    workspace=$(grep "^export WORKSPACE=" "$env_file" 2>/dev/null | sed 's/export WORKSPACE=//' | sed 's/"//g' | sed 's/'"'"'//g')
    
    echo "$workspace"
}
