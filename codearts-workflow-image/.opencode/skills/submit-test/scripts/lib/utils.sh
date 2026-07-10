#!/bin/bash

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
MAGENTA='\033[0;35m'
BOLD='\033[1m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1" >&2
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

log_step() {
    echo -e "${CYAN}[STEP]${NC} $1"
}

log_test() {
    echo -e "${MAGENTA}[TEST]${NC} $1"
}

log_eval() {
    echo -e "${YELLOW}[EVAL]${NC} $1"
}

get_timestamp() {
    date -u +"%Y-%m-%dT%H:%M:%SZ"
}

validate_kubeconfig() {
    if [[ -z "$KUBECONFIG" ]]; then
        log_error "KUBECONFIG environment variable is not set"
        return 1
    fi
    if [[ ! -f "$KUBECONFIG" ]]; then
        log_error "Kubeconfig file not found: $KUBECONFIG"
        return 1
    fi
    return 0
}

usage() {
    cat <<EOF
Usage: $0 [OPTIONS]

Options:
    -k, --kubeconfig PATH    Path to kubeconfig file (or set KUBECONFIG env var)
    -t, --test TYPE          Test type: all, submit, validate
    -w, --workflow NAME     Specific workflow name to test
    -h, --help              Show this help message

Examples:
    $0 -k ~/.kube/config -t all
    $0 --test submit --workflow my-workflow

EOF
}
