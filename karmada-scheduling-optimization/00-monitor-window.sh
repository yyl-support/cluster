#!/usr/bin/env bash

set -Eeuo pipefail
BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${BASE_DIR}/lib/common.sh"

once=0
new=0
for arg in "$@"; do
  [[ "${arg}" == "--once" ]] && once=1
  [[ "${arg}" == "--new-run" ]] && new=1
done
require_commands

if [[ "${new}" == "1" || ! -f "${RUNTIME_DIR}/active-run" ]]; then
  new_run
  set_stage monitoring-window
else
  load_run
fi

stable_since=0

while true; do
  report="${RUN_DIR}/reports/window-$(date '+%Y%m%d-%H%M%S').json"
  if ! in_night_window; then
    log INFO "Outside Beijing-time window ${NIGHT_START_HOUR}:00-${NIGHT_END_HOUR}:00"
    stable_since=0
  elif collect_health_json "${report}" && health_gate_ok "${report}"; then
    bound_pending="$(health_value "${report}" cluster001.bound_pending)"
    if [[ "${stable_since}" -eq 0 ]]; then
      stable_since="$(date +%s)"
      log INFO "Stable candidate started; bound_pending=${bound_pending}"
    fi
    elapsed=$(( $(date +%s) - stable_since ))
    log INFO "Stable for ${elapsed}/${STABLE_SECONDS}s"
    if (( elapsed >= STABLE_SECONDS )); then
      set_stage canary-ready
      log INFO "Window accepted. Workflow will execute canary for run ${RUN_ID}."
      exit 0
    fi
  else
    log WARN "Health gate failed; reset stability timer"
    stable_since=0
  fi

  (( once == 1 )) && exit 1
  sleep "${POLL_SECONDS}"
done
