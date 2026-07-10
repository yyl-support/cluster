#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

get_namespace_from_workflow() {
    local workflow_file="$1"
    cd "$SCRIPT_DIR/go" && go run ./cmd/ns --from-workflow "$workflow_file"
}

TEST_CASE_DIR="$1"
KUBECONFIG_FILE="$2"

if [ -z "$TEST_CASE_DIR" ] || [ -z "$KUBECONFIG_FILE" ]; then
    echo "Usage: $0 <test-case-dir> <kubeconfig-file>"
    echo "   or: $0 all <kubeconfig-file> (to test all cases)"
    exit 1
fi

KUBECONFIG_FILE="$(cd "$(dirname "$KUBECONFIG_FILE")" && pwd)/$(basename "$KUBECONFIG_FILE")"

run_test() {
    local test_case_dir="$1"
    local kubeconfig="$2"
    local expected_exit_code="0"
    local pipeline_run_id=""
    local namespace="argo"
    
    echo "=========================================="
    echo "Testing: $test_case_dir"
    echo "=========================================="
    
    # Convert to absolute path
    test_case_dir="$(cd "$test_case_dir" && pwd)"
    
    # Get namespace from workflow
    local expected_yaml="${test_case_dir}/expected.yaml"
    if [[ -f "$expected_yaml" ]]; then
        namespace=$(get_namespace_from_workflow "$expected_yaml")
    fi
    
    # Delete old image if exists
    docker rmi "workflow-image:$(basename "$test_case_dir")" 2>/dev/null || true
    
    # Build image (no cache)
    docker build --no-cache -t "workflow-image:$(basename "$test_case_dir")" .
    
    # Create temp dir for workspace
    WORKSPACE_DIR=$(mktemp -d)
    cp "$test_case_dir/shell.sh" "$WORKSPACE_DIR/"
    cat "$kubeconfig" | base64 -w 0 > "$WORKSPACE_DIR/kubeconfig.key"

    # Extract WORKSPACE from env.sh, default to /CP_workspace
    WORKSPACE_VALUE=$(grep -E "^export WORKSPACE=" "$test_case_dir/env.sh" 2>/dev/null | sed 's/export WORKSPACE=//' | sed 's/"//g' | sed "s/'//g")
    if [ -z "$WORKSPACE_VALUE" ]; then
        WORKSPACE_VALUE="/CP_workspace"
    fi

    expected_exit_code=$(grep -E "^export EXPECTED_EXIT_CODE=" "$test_case_dir/env.sh" 2>/dev/null | sed 's/export EXPECTED_EXIT_CODE=//' | sed 's/"//g' | sed "s/'//g")
    if [ -z "$expected_exit_code" ]; then
        expected_exit_code="0"
    fi

    pipeline_run_id=$(grep -E "^export CP_pipeline_run_id=" "$test_case_dir/env.sh" 2>/dev/null | sed 's/export CP_pipeline_run_id=//' | sed 's/"//g' | sed "s/'//g")
    
    # Run container in background
    docker run -d \
        -e WORKSPACE="$WORKSPACE_VALUE" \
        -v "$WORKSPACE_DIR":"$WORKSPACE_VALUE" \
        -v "$test_case_dir/env.sh":"$WORKSPACE_VALUE/env.sh" \
        --name "workflow-test-$(basename "$test_case_dir")" \
        "workflow-image:$(basename "$test_case_dir")" \
        sleep infinity
    
    # Exec: cd to WORKSPACE, source env.sh, run entrypoint.sh
    set +e
    docker exec "workflow-test-$(basename "$test_case_dir")" bash -c 'cd $WORKSPACE && source ./env.sh && /workspace/workflowtool/entrypoint.sh'
    exit_code=$?
    set -e

    if [ "$exit_code" -ne "$expected_exit_code" ]; then
        echo "Test failed: $test_case_dir"
        echo "Expected exit code: $expected_exit_code, got: $exit_code"
        if [ -n "$pipeline_run_id" ]; then
            kubectl delete job.batch.volcano.sh -n "$namespace" -l "pipeline/run-id=$pipeline_run_id" --kubeconfig "$kubeconfig" --force --grace-period=0 2>/dev/null || true
        fi
        docker rm -f "workflow-test-$(basename "$test_case_dir")" >/dev/null 2>&1 || true
        rm -rf "$WORKSPACE_DIR"
        exit 1
    fi

    docker exec "workflow-test-$(basename "$test_case_dir")" bash -c 'ls -la "$WORKSPACE"'
    if [ -n "$pipeline_run_id" ]; then
        kubectl delete job.batch.volcano.sh -n "$namespace" -l "pipeline/run-id=$pipeline_run_id" --kubeconfig "$kubeconfig" --force --grace-period=0 2>/dev/null || true
    fi
    # Cleanup
    docker rm -f "workflow-test-$(basename "$test_case_dir")"
    rm -rf "$WORKSPACE_DIR"
    
    echo "Test passed: $test_case_dir (exit code $exit_code)"
    echo ""
}

if [ "$TEST_CASE_DIR" = "all" ]; then
    TEST_CASES_DIR="$SCRIPT_DIR/go/cmd/converter/case/newtest"
    
    if [ ! -d "$TEST_CASES_DIR" ]; then
        echo "Error: Test cases directory not found: $TEST_CASES_DIR"
        exit 1
    fi
    
    for test_dir in "$TEST_CASES_DIR"/test*; do
        if [ -d "$test_dir" ]; then
            run_test "$test_dir" "$KUBECONFIG_FILE"
        fi
    done
    
    echo "=========================================="
    echo "All tests passed!"
    echo "=========================================="
else
    run_test "$TEST_CASE_DIR" "$KUBECONFIG_FILE"
fi
