#!/usr/bin/env bash

set -Eeuo pipefail
BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${BASE_DIR}/lib/common.sh"

[[ "${1:-}" == "--execute" ]] || die "Usage: ./10-canary.sh --execute RUN_ID"
EXECUTE=1
load_run "${2:-}"
require_commands
require_execute

pre="${RUN_DIR}/reports/canary-pre.json"
collect_health_json "${pre}"
health_gate_ok "${pre}" || die "Current health gate blocks canary"

name="dispatch-canary-${RUN_ID//-/}"
namespace="default"
trap 'cleanup_canary "${namespace}" "${name}"' EXIT
set_stage canary-running

apply_canary "${namespace}" "${name}"
wait_for_rb_cluster "${namespace}" "${name}" "001" 120 || die "Canary RB validation failed"
pod="$(wait_for_canary_pod "${namespace}" "${name}" 180)" || die "Canary Pod was not created in 001"
node="$(cluster001 get pod -n "${namespace}" "${pod}" -o jsonpath='{.spec.nodeName}')"
[[ -n "${node}" ]] || die "Canary Pod has no node"
npu="$(cluster001 get node "${node}" -o jsonpath='{.metadata.labels.accelerator\/huawei-npu}')"
[[ -z "${npu}" ]] || die "Canary landed on NPU node: ${node}"

post="${RUN_DIR}/reports/canary-post.json"
collect_health_json "${post}"
health_gate_ok "${post}" || die "Health gate failed after canary"

touch "${RUN_DIR}/canary-passed"
set_stage canary-passed
log INFO "Canary passed: rb=001, pod=${pod}, node=${node}, npu=none"
