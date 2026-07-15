#!/usr/bin/env bash

set -Eeuo pipefail
BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${BASE_DIR}/lib/common.sh"

[[ "${1:-}" == "--execute" && $# -eq 2 ]] || die "Usage: ./40-rollback.sh --execute RUN_ID"
EXECUTE=1
load_run "${2:-}"
require_execute
set_stage rolling-back

[[ ! -f "${RUN_DIR}/changed-namespaces" ]] || run_with_retry 3 5 restore_stage namespaces || die "namespace rollback failed"
[[ ! -f "${RUN_DIR}/changed-p20" ]] || run_with_retry 3 5 restore_stage p20 || die "p20 rollback failed"
[[ ! -f "${RUN_DIR}/changed-cop" ]] || run_with_retry 3 5 restore_stage cop || die "COP rollback failed"

if [[ -f "${RUN_DIR}/changed-p20" ]]; then
  json_field_matches_backup cpp non-npu-vcjob-prefer-001 spec.placement.clusterAffinity "${RUN_DIR}/backups/p20-cluster-affinity.json" || die "p20 rollback verification failed"
fi
if [[ -f "${RUN_DIR}/changed-cop" ]]; then
  json_field_matches_backup cop non-npu-vcjob-node-affinity spec.overrideRules.0.overriders.plaintext.0.value.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution "${RUN_DIR}/backups/cop-preferred.json" || die "COP rollback verification failed"
fi
if [[ -f "${RUN_DIR}/changed-namespaces" ]]; then
  for policy in "${NS_POLICIES[@]}"; do
    json_field_matches_backup cpp "${policy}" spec.placement.clusterAffinity "${RUN_DIR}/backups/${policy}-cluster-affinity.json" || die "${policy} rollback verification failed"
  done
fi

post="${RUN_DIR}/reports/rollback-post.json"
collect_health_json "${post}"
health_gate_ok "${post}" || die "Health gate failed after rollback"
set_stage rolled-back
touch "${RUN_DIR}/rollback-completed"
log INFO "Rollback completed; only run-owned COP/p20/namespace changes were restored"
