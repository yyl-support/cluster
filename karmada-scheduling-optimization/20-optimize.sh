#!/usr/bin/env bash

set -Eeuo pipefail
BASE_DIR="$(cd "$(dirname "$0")" && pwd)"
source "${BASE_DIR}/lib/common.sh"

[[ "${1:-}" == "--execute" && $# -eq 3 ]] || die "Usage: ./20-optimize.sh --execute RUN_ID {cop|p20|namespaces}"
EXECUTE=1
load_run "$2"
stage="$3"
require_commands
require_execute
[[ -f "${RUN_DIR}/canary-passed" ]] || die "Canary has not passed"

case "${stage}" in
  cop) ;;
  p20) [[ -f "${RUN_DIR}/cop-passed" ]] || die "COP stage has not passed" ;;
  namespaces) [[ -f "${RUN_DIR}/p20-passed" ]] || die "p20 stage has not passed" ;;
  *) die "Unsupported stage: ${stage}" ;;
esac

changed=0
validation_name=""
on_exit() {
  local status=$?
  trap - EXIT
  if [[ "${status}" -ne 0 && "${changed}" == "1" ]]; then
    log ERROR "Stage ${stage} failed after mutation; restoring stage backup"
    run_with_retry 3 5 restore_stage "${stage}" || log ERROR "Automatic stage restore failed"
  fi
  [[ -z "${validation_name}" ]] || cleanup_canary default "${validation_name}"
  exit "${status}"
}
trap on_exit EXIT

pre="${RUN_DIR}/reports/${stage}-pre.json"
collect_health_json "${pre}"
health_gate_ok "${pre}" || die "Current health gate blocks optimization"

case "${stage}" in
  cop)
    backup_object cop non-npu-vcjob-node-affinity
    backup_json_field cop non-npu-vcjob-node-affinity \
      'spec.overrideRules.0.overriders.plaintext.0.value.nodeAffinity.preferredDuringSchedulingIgnoredDuringExecution' \
      "${RUN_DIR}/backups/cop-preferred.json"
    current="$(karmada get cop non-npu-vcjob-node-affinity -o jsonpath='{.spec.overrideRules[0].overriders.plaintext[0].value}')"
    patch="$(python3 - "${current}" <<'PY'
import json,sys
d=json.loads(sys.argv[1])
affinity=d["nodeAffinity"]
expressions=[expression for term in affinity["requiredDuringSchedulingIgnoredDuringExecution"]["nodeSelectorTerms"] for expression in term.get("matchExpressions", [])]
assert {"key":"accelerator/huawei-npu","operator":"DoesNotExist"} in expressions
preferred=affinity.setdefault("preferredDuringSchedulingIgnoredDuringExecution", [])
expected={"weight":100,"preference":{"matchExpressions":[{"key":"node.cce.io/billing-mode","operator":"In","values":["pre-paid"]}]}}
preferred[:]=[item for item in preferred if not any(expression.get("key") == "node.cce.io/billing-mode" for expression in item.get("preference", {}).get("matchExpressions", []))]
preferred.append(expected)
print(json.dumps([{"op":"replace","path":"/spec/overrideRules/0/overriders/plaintext/0/value","value":d}], separators=(",", ":")))
PY
)" || die "COP required NPU exclusion invariant missing"
    karmada patch cop non-npu-vcjob-node-affinity --type json -p "${patch}"
    changed=1
    mark_changed cop
    value="$(karmada get cop non-npu-vcjob-node-affinity -o jsonpath='{.spec.overrideRules[0].overriders.plaintext[0].value}')"
    python3 - "${value}" <<'PY' || die "COP post-validation failed"
import json,sys
d=json.loads(sys.argv[1])["nodeAffinity"]
required=[e for t in d["requiredDuringSchedulingIgnoredDuringExecution"]["nodeSelectorTerms"] for e in t.get("matchExpressions", [])]
preferred=d["preferredDuringSchedulingIgnoredDuringExecution"]
assert {"key":"accelerator/huawei-npu","operator":"DoesNotExist"} in required
assert any(item == {"weight":100,"preference":{"matchExpressions":[{"key":"node.cce.io/billing-mode","operator":"In","values":["pre-paid"]}]}} for item in preferred)
PY
    validation_name="cop-canary-${RUN_ID//-/}"
    cleanup_canary default "${validation_name}"
    apply_canary default "${validation_name}"
    wait_for_rb_cluster default "${validation_name}" "001" 120 || die "COP canary RB validation failed"
    validation_pod="$(wait_for_canary_pod default "${validation_name}" 180)" || die "COP canary Pod missing"
    affinity="$(cluster001 get pod -n default "${validation_pod}" -o jsonpath='{.spec.affinity}')"
    python3 - "${affinity}" <<'PY' || die "COP end-to-end canary affinity validation failed"
import json,sys
d=json.loads(sys.argv[1])["nodeAffinity"]
required=[e for t in d["requiredDuringSchedulingIgnoredDuringExecution"]["nodeSelectorTerms"] for e in t.get("matchExpressions", [])]
preferred=d["preferredDuringSchedulingIgnoredDuringExecution"]
assert {"key":"accelerator/huawei-npu","operator":"DoesNotExist"} in required
assert any(item == {"weight":100,"preference":{"matchExpressions":[{"key":"node.cce.io/billing-mode","operator":"In","values":["pre-paid"]}]}} for item in preferred)
PY
    validation_node="$(cluster001 get pod -n default "${validation_pod}" -o jsonpath='{.spec.nodeName}')"
    [[ -z "$(cluster001 get node "${validation_node}" -o jsonpath='{.metadata.labels.accelerator\/huawei-npu}')" ]] || die "COP canary landed on NPU node"
    cleanup_canary default "${validation_name}"
    validation_name=""
    ;;
  p20)
    backup_object cpp non-npu-vcjob-prefer-001
    backup_json_field cpp non-npu-vcjob-prefer-001 'spec.placement.clusterAffinity' "${RUN_DIR}/backups/p20-cluster-affinity.json"
    [[ "$(karmada get cpp non-npu-vcjob-prefer-001 -o jsonpath='{.spec.placement.clusterAffinity.clusterNames}')" == '["001"]' ]] || die "Unexpected p20 source placement"
    [[ "$(karmada get cluster 001 -o jsonpath='{.metadata.labels.has-cpu}')" == "true" ]] || die "001 lacks has-cpu=true"
    [[ -z "$(karmada get cluster wlcb -o jsonpath='{.metadata.labels.has-cpu}')" ]] || die "wlcb unexpectedly has has-cpu"
    karmada patch cpp non-npu-vcjob-prefer-001 --type json -p '[{"op":"replace","path":"/spec/placement/clusterAffinity","value":{"labelSelector":{"matchLabels":{"has-cpu":"true"}}}}]'
    changed=1
    mark_changed p20
    [[ "$(karmada get cpp non-npu-vcjob-prefer-001 -o jsonpath='{.spec.placement.clusterAffinity.labelSelector.matchLabels.has-cpu}')" == "true" ]] || die "p20 validation failed"
    [[ "$(label_targets has-cpu=true)" == "001" ]] || die "p20 selector does not resolve exactly to 001"
    validation_name="p20-canary-${RUN_ID//-/}"
    cleanup_canary default "${validation_name}"
    apply_canary default "${validation_name}"
    wait_for_rb_cluster default "${validation_name}" "001" 120 || die "p20 end-to-end RB validation failed"
    wait_for_canary_pod default "${validation_name}" 180 >/dev/null || die "p20 canary Pod missing"
    cleanup_canary default "${validation_name}"
    validation_name=""
    ;;
  namespaces)
    [[ "$(label_targets dispatch/auto=true)" == "001 wlcb" ]] || die "dispatch/auto=true does not resolve exactly to 001 and wlcb"
    for policy in "${NS_POLICIES[@]}"; do
      backup_object cpp "${policy}"
      backup_json_field cpp "${policy}" 'spec.placement.clusterAffinity' "${RUN_DIR}/backups/${policy}-cluster-affinity.json"
      [[ "$(karmada get cpp "${policy}" -o jsonpath='{.spec.placement.clusterAffinity.clusterNames}')" == '["001","wlcb"]' ]] || die "Unexpected ${policy} source placement"
    done
    changed=1
    mark_changed namespaces
    for policy in "${NS_POLICIES[@]}"; do
      karmada patch cpp "${policy}" --type json -p '[{"op":"replace","path":"/spec/placement/clusterAffinity","value":{"labelSelector":{"matchLabels":{"dispatch/auto":"true"}}}}]'
    done
    for policy in "${NS_POLICIES[@]}"; do
      [[ "$(karmada get cpp "${policy}" -o jsonpath='{.spec.placement.clusterAffinity.labelSelector.matchLabels.dispatch\/auto}')" == "true" ]] || die "${policy} validation failed"
    done
    [[ "$(label_targets dispatch/auto=true)" == "001 wlcb" ]] || die "namespace selector target set changed during optimization"
    for binding in argo pytorch indexsdk recsdk ragsdk op-plugin fbgemm-ascend multimodalsdk mindie-llm mindie-motor mindie-sd; do
      actual="$(karmada get crb "${binding}-namespace" -o jsonpath='{.spec.clusters[*].name}')"
      [[ "$(tr ' ' '\n' <<<"${actual}" | sort | tr '\n' ' ' | sed 's/ $//')" == "001 wlcb" ]] || die "CRB ${binding}-namespace mismatch: ${actual}"
    done
    ;;
esac

post="${RUN_DIR}/reports/${stage}-post.json"
collect_health_json "${post}"
health_gate_ok "${post}" || die "Health gate failed after ${stage}; inspect and rollback"
wait_for_stable_health "${stage}" "${STAGE_STABLE_SECONDS}"
touch "${RUN_DIR}/${stage}-passed"
set_stage "${stage}-passed"
log INFO "Optimization stage passed: ${stage}"
trap - EXIT
