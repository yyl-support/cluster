#!/usr/bin/env bash

set -Eeuo pipefail
BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${BASE_DIR}/lib/common.sh"

require_commands
lock_dir="${RUNTIME_DIR}/workflow.lock"
mkdir "${lock_dir}" 2>/dev/null || die "Another workflow is already running"
trap 'rmdir "${lock_dir}" 2>/dev/null || true' EXIT
"${BASE_DIR}/00-monitor-window.sh" --new-run
load_run

"${BASE_DIR}/10-canary.sh" --execute "${RUN_ID}"
"${BASE_DIR}/20-optimize.sh" --execute "${RUN_ID}" cop
"${BASE_DIR}/20-optimize.sh" --execute "${RUN_ID}" p20
"${BASE_DIR}/20-optimize.sh" --execute "${RUN_ID}" namespaces

set_stage optimization-completed
log INFO "Scheduling optimization completed; continue with manual monitoring"
