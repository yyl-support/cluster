#!/bin/bash

# Test script for the rewritten submit-test skill
# Verifies basic functionality and error handling

# Get script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "${SCRIPT_DIR}")/../../.."
SCRIPTS_DIR="${SCRIPT_DIR}/scripts"

# Colors for output
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

log_test() {
    echo -e "${YELLOW}[TEST]${NC} $1"
}

# Test function
run_test() {
    local test_name="$1"
    local command="$2"
    local expected_exit_code="${3:-0}"
    
    log_test "Running test: ${test_name}"
    echo "  Command: ${command}"
    
    # Execute command and capture output and exit code
    local output
    output=$($command 2>&1)
    local exit_code=$?
    
    echo "  Exit code: ${exit_code} (expected: ${expected_exit_code})"
    
    if [ $exit_code -eq $expected_exit_code ]; then
        log_success "Test ${test_name} passed"
        return 0
    else
        log_error "Test ${test_name} failed"
        echo "  Output:"
        echo "$output" | sed 's/^/    /'
        return 1
    fi
}

# Main test execution
main() {
    echo -e "${MAGENTA}"
    echo "╔═══════════════════════════════════════════════════════════════╗"
    echo "║          Argo Workflow Test Skill - Test Suite             ║"
    echo "╚═══════════════════════════════════════════════════════════════╝"
    echo -e "${NC}"
    
    local total_tests=0
    local passed_tests=0
    local failed_tests=0
    
    # Test 1: Check if main script exists
    ((total_tests++))
    if [ -f "${SCRIPTS_DIR}/main.sh" ]; then
        ((passed_tests++))
        log_success "Test 1: main.sh exists"
    else
        ((failed_tests++))
        log_error "Test 1: main.sh not found"
    fi
    
    # Test 2: Check if test-cases.sh exists
    ((total_tests++))
    if [ -f "${SCRIPTS_DIR}/test-cases.sh" ]; then
        ((passed_tests++))
        log_success "Test 2: test-cases.sh exists"
    else
        ((failed_tests++))
        log_error "Test 2: test-cases.sh not found"
    fi
    
    # Test 3: Check if workflow-submitter.sh exists
    ((total_tests++))
    if [ -f "${SCRIPTS_DIR}/workflow-submitter.sh" ]; then
        ((passed_tests++))
        log_success "Test 3: workflow-submitter.sh exists"
    else
        ((failed_tests++))
        log_error "Test 3: workflow-submitter.sh not found"
    fi
    
    # Test 4: Check if result-validator.sh exists
    ((total_tests++))
    if [ -f "${SCRIPTS_DIR}/result-validator.sh" ]; then
        ((passed_tests++))
        log_success "Test 4: result-validator.sh exists"
    else
        ((failed_tests++))
        log_error "Test 4: result-validator.sh not found"
    fi
    
    # Test 5: Check if test-cases.json exists
    ((total_tests++))
    if [ -f "${SCRIPT_DIR}/test-cases.json" ]; then
        ((passed_tests++))
        log_success "Test 5: test-cases.json exists"
    else
        ((failed_tests++))
        log_error "Test 5: test-cases.json not found"
    fi
    
    # Test 6: Check if scripts have execute permissions
    ((total_tests++))
    if [ -x "${SCRIPTS_DIR}/main.sh" ] && [ -x "${SCRIPTS_DIR}/test-cases.sh" ] && \
       [ -x "${SCRIPTS_DIR}/workflow-submitter.sh" ] && [ -x "${SCRIPTS_DIR}/result-validator.sh" ]; then
        ((passed_tests++))
        log_success "Test 6: All scripts have execute permissions"
    else
        ((failed_tests++))
        log_error "Test 6: Some scripts missing execute permissions"
    fi
    
    # Test 7: Test basic help output
    ((total_tests++))
    if run_test "Help output" "bash ${SCRIPTS_DIR}/main.sh --help" 1; then
        ((passed_tests++))
    else
        ((failed_tests++))
    fi
    
    # Test 8: Test list-tests option
    ((total_tests++))
    if run_test "List tests" "bash ${SCRIPTS_DIR}/main.sh --list-tests" 1; then
        ((passed_tests++))
    else
        ((failed_tests++))
    fi
    
    # Test 9: Test test-cases.sh parsing
    ((total_tests++))
    if run_test "Test case parsing" "bash ${SCRIPTS_DIR}/test-cases.sh" 0; then
        ((passed_tests++))
    else
        ((failed_tests++))
    fi
    
    # Test 10: Test workflow-submitter.sh usage
    ((total_tests++))
    if run_test "Workflow submitter usage" "bash ${SCRIPTS_DIR}/workflow-submitter.sh --help" 1; then
        ((passed_tests++))
    else
        ((failed_tests++))
    fi
    
    # Test 11: Test result-validator.sh usage
    ((total_tests++))
    if run_test "Result validator usage" "bash ${SCRIPTS_DIR}/result-validator.sh --help" 1; then
        ((passed_tests++))
    else
        ((failed_tests++))
    fi
    
    echo ""
    echo "═══════════════════════════════════════════════════════════════"
    echo -e "${BOLD}Test Suite Summary${NC}"
    echo "═══════════════════════════════════════════════════════════════"
    echo "  Total tests: ${total_tests}"
    echo -e "  ${GREEN}Passed: ${passed_tests}${NC}"
    if [ $failed_tests -gt 0 ]; then
        echo -e "  ${RED}Failed: ${failed_tests}${NC}"
    fi
    echo "═══════════════════════════════════════════════════════════════"
    echo ""
    
    if [ $failed_tests -eq 0 ]; then
        log_success "All tests passed! The submit-test skill is ready to use."
        return 0
    else
        log_error "Some tests failed. Please check the implementation."
        return 1
    fi
}

# Execute main function
main "$@"