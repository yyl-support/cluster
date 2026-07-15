#!/usr/bin/env bash

set -Eeuo pipefail
LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BASE_DIR="$(cd "${LIB_DIR}/.." && pwd)"
source "${BASE_DIR}/config.sh"
mkdir -p "${RUNTIME_DIR}"

log() {
  local destination="${RUN_DIR:-${RUNTIME_DIR}}/automation.log"
  printf '%s [%s] %s\n' "$(date '+%F %T %Z')" "$1" "$2" | tee -a "${destination}"
}

die() { log ERROR "$1"; exit 1; }
karmada() { KUBECONFIG="${KARMADA_KUBECONFIG}" "${KUBECTL_BIN}" --request-timeout=30s "$@"; }
cluster001() { KUBECONFIG="${CLUSTER_001_KUBECONFIG}" "${KUBECTL_BIN}" --request-timeout=30s "$@"; }
clusterwlcb() { KUBECONFIG="${CLUSTER_WLCB_KUBECONFIG}" "${KUBECTL_BIN}" --request-timeout=30s "$@"; }

require_commands() {
  command -v "${KUBECTL_BIN}" >/dev/null || die "kubectl not found: ${KUBECTL_BIN}"
  command -v python3 >/dev/null || die "python3 not found"
  [[ -r "${KARMADA_KUBECONFIG}" ]] || die "Karmada kubeconfig is not readable"
  [[ -r "${CLUSTER_001_KUBECONFIG}" ]] || die "001 kubeconfig is not readable"
  [[ -r "${CLUSTER_WLCB_KUBECONFIG}" ]] || die "wlcb kubeconfig is not readable"
}

new_run() {
  RUN_ID="$(date '+%Y%m%d-%H%M%S')"
  RUN_DIR="${RUNTIME_DIR}/runs/${RUN_ID}"
  mkdir -p "${RUN_DIR}/backups" "${RUN_DIR}/reports"
  printf '%s\n' "${RUN_ID}" > "${RUNTIME_DIR}/active-run"
  export RUN_ID RUN_DIR
}

load_run() {
  RUN_ID="${1:-}"
  if [[ -z "${RUN_ID}" && -f "${RUNTIME_DIR}/active-run" ]]; then RUN_ID="$(<"${RUNTIME_DIR}/active-run")"; fi
  [[ -n "${RUN_ID}" ]] || die "No active run"
  RUN_DIR="${RUNTIME_DIR}/runs/${RUN_ID}"
  [[ -d "${RUN_DIR}" ]] || die "Run not found: ${RUN_ID}"
  export RUN_ID RUN_DIR
}

set_stage() { printf '%s\n' "$1" > "${RUN_DIR}/stage"; log INFO "stage=$1"; }
require_execute() { [[ "${EXECUTE:-0}" == "1" ]] || die "Write operation blocked; run through workflow.sh or pass --execute"; }

run_with_retry() {
  local attempts="$1" delay="$2" current=1
  shift 2
  while ! "$@"; do
    (( current >= attempts )) && return 1
    log WARN "Command failed; retrying ${current}/${attempts}: $*"
    current=$((current + 1))
    sleep "${delay}"
  done
}

in_night_window() {
  local hour
  hour=$((10#$(date '+%H')))
  [[ "${hour}" -ge "${NIGHT_START_HOUR}" && "${hour}" -lt "${NIGHT_END_HOUR}" ]]
}

collect_health_json() {
  local output="$1" tmp="${RUN_DIR:-${RUNTIME_DIR}}/.health-tmp"
  mkdir -p "${tmp}"
  karmada get clusters -o json > "${tmp}/clusters.json"
  cluster001 get pods -A -o json > "${tmp}/pods-001.json"
  clusterwlcb get pods -A -o json > "${tmp}/pods-wlcb.json"
  cluster001 get nodes -o json > "${tmp}/nodes-001.json"
  clusterwlcb get nodes -o json > "${tmp}/nodes-wlcb.json"
  cluster001 get queue "${TEST_QUEUE}" -o json > "${tmp}/queue-001.json"
  clusterwlcb get queue "${TEST_QUEUE}" -o json > "${tmp}/queue-wlcb.json"
  python3 "${BASE_DIR}/lib/health.py" "${tmp}" > "${output}"
}

health_value() {
  python3 - "$1" "$2" <<'PY'
import json, sys
value=json.load(open(sys.argv[1]))
for part in sys.argv[2].split('.'):
    value=value[part]
print(value)
PY
}

health_gate_ok() {
  [[ "$(health_value "$1" clusters.all_ready)" == "True" ]] || return 1
  [[ "$(health_value "$1" cluster001.unbound_pending)" == "0" ]] || return 1
  [[ "$(health_value "$1" clusterwlcb.unbound_pending)" == "0" ]] || return 1
  if [[ "${REQUIRE_ZERO_PENDING}" == "1" ]]; then
    [[ "$(health_value "$1" cluster001.bound_pending)" == "0" ]] || return 1
    [[ "$(health_value "$1" clusterwlcb.bound_pending)" == "0" ]] || return 1
  fi
  [[ "$(health_value "$1" cluster001.queue_open)" == "True" ]] || return 1
  [[ "$(health_value "$1" clusterwlcb.queue_open)" == "True" ]] || return 1
}

wait_for_stable_health() {
  local prefix="$1" required_seconds="${2:-${STAGE_STABLE_SECONDS}}" stable_since=0 report
  while true; do
    report="${RUN_DIR}/reports/${prefix}-stable-$(date '+%Y%m%d-%H%M%S').json"
    if collect_health_json "${report}" && health_gate_ok "${report}"; then
      [[ "${stable_since}" -ne 0 ]] || stable_since="$(date +%s)"
      (( $(date +%s) - stable_since >= required_seconds )) && return 0
    else
      stable_since=0
    fi
    sleep "${POLL_SECONDS}"
  done
}

restore_stage() {
  local stage="$1"
  case "${stage}" in
    cop) restore_json_field cop non-npu-vcjob-node-affinity /spec/overrideRules/0/overriders/plaintext/0/value/nodeAffinity/preferredDuringSchedulingIgnoredDuringExecution "${RUN_DIR}/backups/cop-preferred.json" ;;
    p20) restore_json_field cpp non-npu-vcjob-prefer-001 /spec/placement/clusterAffinity "${RUN_DIR}/backups/p20-cluster-affinity.json" ;;
    namespaces) for policy in "${NS_POLICIES[@]}"; do restore_json_field cpp "${policy}" /spec/placement/clusterAffinity "${RUN_DIR}/backups/${policy}-cluster-affinity.json"; done ;;
    *) die "Unknown restore stage: ${stage}" ;;
  esac
}

backup_json_field() {
  local type="$1" name="$2" path="$3" output="$4" document="${RUN_DIR}/.field-backup-source.json"
  karmada get "${type}" "${name}" -o json > "${document}"
  python3 - "${path}" "${document}" > "${output}" <<'PY'
import json, sys
path, source = sys.argv[1:]
with open(source, encoding="utf-8") as handle: value = json.load(handle)
exists = True
for part in path.split("."):
    if isinstance(value, dict) and part in value: value = value[part]
    elif isinstance(value, list) and part.isdigit() and int(part) < len(value): value = value[int(part)]
    else: exists, value = False, None; break
json.dump({"exists": exists, "value": value}, sys.stdout, separators=(",", ":"))
PY
  [[ -s "${output}" ]] || die "Field backup failed: ${type}/${name} ${path}"
}

restore_json_field() {
  local type="$1" name="$2" path="$3" input="$4" patch document="${RUN_DIR}/.field-restore-current.json"
  [[ -s "${input}" ]] || die "Field backup missing: ${input}"
  karmada get "${type}" "${name}" -o json > "${document}"
  patch="$(python3 - "${path}" "${input}" "${document}" <<'PY'
import json, sys
path, source, current_source = sys.argv[1:]
with open(source, encoding="utf-8") as handle: backup = json.load(handle)
with open(current_source, encoding="utf-8") as handle: value = json.load(handle)
current_exists = True
for part in path.strip("/").split("/"):
    if isinstance(value, dict) and part in value: value = value[part]
    elif isinstance(value, list) and part.isdigit() and int(part) < len(value): value = value[int(part)]
    else: current_exists = False; break
if not backup["exists"] and not current_exists: print("[]"); raise SystemExit
operation={"op":"replace" if backup["exists"] and current_exists else "add" if backup["exists"] else "remove","path":path}
if backup["exists"]: operation["value"]=backup["value"]
print(json.dumps([operation], separators=(",", ":")))
PY
)"
  [[ "${patch}" == "[]" ]] || karmada patch "${type}" "${name}" --type json -p "${patch}"
}

json_field_matches_backup() {
  local type="$1" name="$2" path="$3" input="$4" document="${RUN_DIR}/.field-verify-current.json"
  karmada get "${type}" "${name}" -o json > "${document}"
  python3 - "${path}" "${input}" "${document}" <<'PY'
import json, sys
path, source, current_source = sys.argv[1:]
with open(current_source, encoding="utf-8") as handle: value = json.load(handle)
with open(source, encoding="utf-8") as handle: backup = json.load(handle)
exists=True
for part in path.split("."):
    if isinstance(value, dict) and part in value: value=value[part]
    elif isinstance(value, list) and part.isdigit() and int(part) < len(value): value=value[int(part)]
    else: exists, value=False, None; break
raise SystemExit(0 if exists == backup["exists"] and value == backup["value"] else 1)
PY
}

label_targets() { karmada get clusters -l "$1" -o jsonpath='{.items[*].metadata.name}' | tr ' ' '\n' | sort | tr '\n' ' ' | sed 's/ $//'; }
backup_object() { local type="$1" name="$2"; karmada get "${type}" "${name}" -o yaml > "${RUN_DIR}/backups/${type}-${name}.yaml"; [[ -s "${RUN_DIR}/backups/${type}-${name}.yaml" ]] || die "Backup failed: ${type}/${name}"; }
mark_changed() { touch "${RUN_DIR}/changed-$1"; date -u '+%Y-%m-%dT%H:%M:%SZ' > "${RUN_DIR}/changed-$1-at"; }

wait_for_rb_cluster() {
  local namespace="$1" resource_name="$2" expected="$3" timeout_seconds="${4:-120}" deadline=$((SECONDS + timeout_seconds)) actual=""
  while (( SECONDS < deadline )); do
    actual="$(karmada get rb -n "${namespace}" -o json | python3 -c 'import json,sys; name=sys.argv[1]; print(next((" ".join(sorted(c.get("name","") for c in rb.get("spec",{}).get("clusters",[]))) for rb in json.load(sys.stdin).get("items",[]) if rb.get("spec",{}).get("resource",{}).get("name")==name), ""))' "${resource_name}")"
    [[ "${actual}" == "${expected}" ]] && return 0
    sleep 5
  done
  log ERROR "RB mismatch: ${namespace}/${resource_name}; expected=${expected}, actual=${actual:-none}"
  return 1
}

cleanup_canary() { karmada delete vcjob -n "$1" "$2" --ignore-not-found >/dev/null 2>&1 || true; }

apply_canary() {
  local namespace="$1" name="$2"
  cat <<EOF | karmada apply -f -
apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
  name: ${name}
  namespace: ${namespace}
  labels:
    automation.karmada.io/run-id: "${RUN_ID}"
spec:
  schedulerName: volcano
  queue: ${TEST_QUEUE}
  ttlSecondsAfterFinished: 30
  minAvailable: 1
  tasks:
  - replicas: 1
    name: test
    template:
      spec:
        containers:
        - name: test
          image: ${TEST_IMAGE}
          command: ["sh", "-c", "sleep 30"]
          resources:
            requests: {cpu: 10m, memory: 16Mi}
            limits: {cpu: 50m, memory: 32Mi}
        restartPolicy: Never
EOF
}

wait_for_canary_pod() {
  local namespace="$1" name="$2" timeout_seconds="${3:-180}" deadline=$((SECONDS + timeout_seconds)) pod="" phase=""
  while (( SECONDS < deadline )); do
    pod="$(cluster001 get pods -n "${namespace}" -l "volcano.sh/job-name=${name}" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || true)"
    phase="$(cluster001 get pods -n "${namespace}" -l "volcano.sh/job-name=${name}" -o jsonpath='{.items[0].status.phase}' 2>/dev/null || true)"
    if [[ -n "${pod}" && ( "${phase}" == "Running" || "${phase}" == "Succeeded" ) ]]; then printf '%s\n' "${pod}"; return 0; fi
    sleep 5
  done
  return 1
}
