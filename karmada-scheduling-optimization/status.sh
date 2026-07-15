#!/usr/bin/env bash

set -Eeuo pipefail
BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${BASE_DIR}/lib/common.sh"

load_run "${1:-}"
printf 'run_id=%s\n' "${RUN_ID}"
printf 'stage=%s\n' "$(<"${RUN_DIR}/stage")"
for marker in canary-passed cop-passed p20-passed namespaces-passed rollback-completed; do
  [[ -f "${RUN_DIR}/${marker}" ]] && printf '%s=true\n' "${marker}" || printf '%s=false\n' "${marker}"
done
