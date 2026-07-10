#!/bin/bash

set -euo pipefail

IMAGE_FILE=""
NAMESPACE="${NAMESPACE:-preheat}"
TIMEOUT="${TIMEOUT:-600}"
KUBECONFIG="${KUBECONFIG:-}"
CONCURRENCY="${CONCURRENCY:-3}"
INCLUDE_NODES=""
EXCLUDE_NODES=""
LABEL_PREFIX="image-preheat"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info()  { echo -e "${GREEN}[INFO]${NC}  $1" >&2; }
log_warn()  { echo -e "${YELLOW}[WARN]${NC}  $1" >&2; }
log_error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }
log_debug() { [[ "${DEBUG:-0}" == "1" ]] && echo -e "${BLUE}[DEBUG]${NC} $1" >&2; }

usage() {
    cat <<'USAGE'
用法: preheat.sh -f <镜像列表文件> [-k <kubeconfig>] [-n <命名空间>] [-t <超时>] [-c <并发数>] [-N <指定节点>] [-E <排除节点>]

在集群节点上预先拉取指定镜像。
- 每个镜像在所有兼容架构节点上预热
- 每个 Pod 只拉一个镜像
- 支持并发控制、错误检测、进度显示

选项:
    -f, --file       镜像列表文件路径（每行一个镜像）
    -k, --kubeconfig kubeconfig 文件路径（默认集群内 ServiceAccount）
    -n, --namespace  创建 Pod 的命名空间（默认: preheat）
    -t, --timeout    每个 Pod 超时秒数（默认: 600，0=不限）
    -c, --concurrency 并发 Pod 数（默认: 3）
    -N, --nodes      只在指定节点预热（逗号分隔）
    -E, --exclude    排除指定节点（逗号分隔）
    -h, --help       显示帮助

示例:
    preheat.sh -f images.txt
    preheat.sh -f images.txt -k /path/to/kubeconfig -c 5
    preheat.sh -f images.txt -N node1,node2
    preheat.sh -f images.txt -E master,control-plane

镜像列表文件格式（每行一个镜像，# 开头为注释）:
    nginx:alpine              # multi-arch → 所有节点
    myapp:v1-amd64            # 只在 x86 节点
    myapp:v1-arm64            # 只在 arm 节点
    # 注释行会被忽略
USAGE
    exit 1
}

parse_args() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -f|--file)       IMAGE_FILE="$2"; shift 2 ;;
            -k|--kubeconfig) KUBECONFIG="$2"; shift 2 ;;
            -n|--namespace)  NAMESPACE="$2"; shift 2 ;;
            -t|--timeout)    TIMEOUT="$2"; shift 2 ;;
            -c|--concurrency) CONCURRENCY="$2"; shift 2 ;;
            -N|--nodes)      INCLUDE_NODES="$2"; shift 2 ;;
            -E|--exclude)    EXCLUDE_NODES="$2"; shift 2 ;;
            -h|--help)       usage ;;
            *)               log_error "Unknown option: $1"; usage ;;
        esac
    done

    if [[ -z "$IMAGE_FILE" ]]; then
        log_error "Must specify image file with -f"; usage
    fi
    if [[ ! -f "$IMAGE_FILE" ]]; then
        log_error "Image file not found: $IMAGE_FILE"; exit 1
    fi
    if ! [[ "$CONCURRENCY" =~ ^[1-9][0-9]*$ ]]; then
        log_error "Concurrency must be positive integer"; exit 1
    fi
    if ! [[ "$TIMEOUT" =~ ^[0-9]+$ ]]; then
        log_error "Timeout must be non-negative integer"; exit 1
    fi
    if [[ -n "$INCLUDE_NODES" ]] && [[ -n "$EXCLUDE_NODES" ]]; then
        log_error "Cannot use both -N and -E"; exit 1
    fi
}

kc() {
    if [[ -n "$KUBECONFIG" ]]; then
        kubectl --kubeconfig "$KUBECONFIG" "$@"
    else
        kubectl "$@"
    fi
}

cleanup() {
    local exit_code=$?
    log_info "Cleaning up remaining preheat pods..."
    kc delete pods -n "$NAMESPACE" -l "$LABEL_PREFIX=preheat" --grace-period=0 --force --ignore-not-found=true &>/dev/null || true
    exit $exit_code
}
trap cleanup EXIT

read_images() {
    local images=()
    while IFS= read -r line || [[ -n "$line" ]]; do
        line="${line#"${line%%[![:space:]]*}"}"
        line="${line%"${line##*[![:space:]]}"}"
        [[ -z "$line" ]] && continue
        [[ "$line" =~ ^# ]] && continue
        images+=("$line")
    done < "$IMAGE_FILE"
    printf '%s\n' "${images[@]}"
}

detect_image_arch() {
    local image="$1"
    local lower=$(echo "$image" | tr '[:upper:]' '[:lower:]')
    if echo "$lower" | grep -qE 'amd64|x86_64|x64'; then
        echo "amd64"
    elif echo "$lower" | grep -qE 'arm64|aarch64|armv[789]'; then
        echo "arm64"
    elif echo "$lower" | grep -qE 'x86'; then
        echo "amd64"
    elif echo "$lower" | grep -qE 'arm'; then
        echo "arm64"
    else
        echo "multi"
    fi
}

get_node_arch() {
    local node="$1"
    local arch=$(kc get node "$node" -o jsonpath='{.status.nodeInfo.architecture}' 2>/dev/null || echo "Unknown")
    case "$arch" in
        amd64)  echo "amd64" ;;
        arm64)  echo "arm64" ;;
        *)      echo "$arch" ;;
    esac
}

sanitize_pod_name() {
    echo "$1" | tr '[:upper:]' '[:lower:]' | sed 's/[^a-z0-9.-]/-/g' | sed 's/^-//;s/-$//' | cut -c1-63
}

create_pod() {
    local node="$1"
    local image="$2"
    local ts="$(date +%s)"
    local pod_name="$LABEL_PREFIX-$(sanitize_pod_name "$node")-$(sanitize_pod_name "$image")-$ts"
    
    cat <<EOF | kc apply -f - &>/dev/null
apiVersion: v1
kind: Pod
metadata:
  name: ${pod_name}
  namespace: ${NAMESPACE}
  labels:
    ${LABEL_PREFIX}: preheat
    preheat-node: ${node}
spec:
  nodeName: ${node}
  restartPolicy: Never
  tolerations:
    - operator: Exists
  containers:
    - name: preheat
      image: ${image}
      imagePullPolicy: IfNotPresent
      command: ["true"]
      resources:
        requests:
          cpu: 50m
          memory: 32Mi
        limits:
          cpu: 200m
          memory: 128Mi
EOF

    echo "$pod_name"
}

is_pod_failed() {
    local pod_name="$1"
    local phase=$(kc get pod "$pod_name" -n "$NAMESPACE" -o jsonpath='{.status.phase}' 2>/dev/null || echo "Unknown")

    case "$phase" in
        Succeeded) echo "succeeded"; return 0 ;;
        Failed)    echo "failed"; return 0 ;;
        Unknown|"") echo "stuck"; return 0 ;;
    esac

    local reasons=$(kc get pod "$pod_name" -n "$NAMESPACE" -o jsonpath='{.status.containerStatuses[*].state.waiting.reason}' 2>/dev/null || echo "")
    for r in $reasons; do
        case "$r" in
            ImagePullBackOff|ErrImagePull|InvalidImageName|CrashLoopBackOff|CreateContainerConfigError|CreateContainerError)
                echo "failed"; return 0 ;;
        esac
    done

    local term_reasons=$(kc get pod "$pod_name" -n "$NAMESPACE" -o jsonpath='{.status.containerStatuses[*].state.terminated.reason}' 2>/dev/null || echo "")
    for r in $term_reasons; do
        case "$r" in
            OOMKilled|Error|ContainerCannotRun)
                echo "failed"; return 0 ;;
        esac
    done

    echo "pending"; return 0
}

wait_for_pods() {
    local pod_names="$1"
    local start=$SECONDS

    while true; do
        local success_count=0
        local fail_count=0
        local stuck_count=0
        local pending_count=0

        for pn in $pod_names; do
            local result=$(is_pod_failed "$pn")
            case "$result" in
                succeeded) success_count=$((success_count+1)) ;;
                failed)    fail_count=$((fail_count+1)) ;;
                stuck)     stuck_count=$((stuck_count+1)) ;;
                pending)   pending_count=$((pending_count+1)) ;;
            esac
        done

        if [[ $pending_count -eq 0 ]] && [[ $stuck_count -eq 0 ]]; then
            local duration=$(( SECONDS - start ))
            log_info "Batch done: ${success_count} ok, ${fail_count} failed (took ${duration}s)"
            return 0
        fi

        if [[ $stuck_count -gt 0 ]] && [[ $(( SECONDS - start )) -ge 300 ]]; then
            log_warn "${stuck_count} pods stuck (Unknown) for 300s, treating as failed"
            fail_count=$((fail_count + stuck_count))
            local duration=$(( SECONDS - start ))
            log_info "Batch done: ${success_count} ok, ${fail_count} failed (took ${duration}s)"
            return 1
        fi

        if [[ $TIMEOUT -gt 0 ]] && [[ $(( SECONDS - start )) -ge $TIMEOUT ]]; then
            log_warn "Batch timeout (${TIMEOUT}s)"
            return 1
        fi

        log_debug "Progress: ${success_count} ok, ${fail_count} fail, ${pending_count} pending, ${stuck_count} stuck"
        sleep 5
    done
}

delete_pods() {
    local pod_names="$1"
    for pn in $pod_names; do
        kc delete pod "$pn" -n "$NAMESPACE" --grace-period=5 --ignore-not-found=true &>/dev/null &
    done
    wait || true
}

main() {
    parse_args "$@"

    log_info "=== Image Preheat Starting ==="
    log_info "Image file:    $IMAGE_FILE"
    log_info "Namespace:     $NAMESPACE"
    log_info "Kubeconfig:    ${KUBECONFIG:-集群内 ServiceAccount}"
    log_info "Concurrency:   $CONCURRENCY"
    log_info "Timeout:       ${TIMEOUT}s (0=unlimited)"
    [[ -n "$INCLUDE_NODES" ]] && log_info "Include nodes: $INCLUDE_NODES"
    [[ -n "$EXCLUDE_NODES" ]] && log_info "Exclude nodes: $EXCLUDE_NODES"

    kc create namespace "$NAMESPACE" --dry-run=client -o yaml | kc apply -f - &>/dev/null || true

    local old_pods=$(kc get pods -n "$NAMESPACE" -l "$LABEL_PREFIX=preheat" -o jsonpath='{.items[*].metadata.name}' 2>/dev/null || true)
    if [[ -n "$old_pods" ]]; then
        log_warn "Found leftover pods, cleaning up..."
        kc delete pods -n "$NAMESPACE" -l "$LABEL_PREFIX=preheat" --grace-period=0 --force --ignore-not-found=true &>/dev/null || true
        sleep 3
    fi

    local images_str=$(read_images)
    local image_count=$(echo "$images_str" | grep -c '.' || echo 0)

    if [[ $image_count -eq 0 ]]; then
        log_error "No images found"; exit 1
    fi

    local -a images_arr=()
    while IFS= read -r line; do
        [[ -n "$line" ]] && images_arr+=("$line")
    done <<< "$images_str"

    log_info "Images ($image_count):"
    for image in "${images_arr[@]}"; do
        printf "  - %s (%s)\n" "$image" "$(detect_image_arch "$image")"
    done

    local -a nodes_arr=()
    local -a node_archs=()
    local -a exclude_arr=()

    if [[ -n "$EXCLUDE_NODES" ]]; then
        IFS=',' read -ra exclude_arr <<< "$EXCLUDE_NODES"
    fi

    if [[ -n "$INCLUDE_NODES" ]]; then
        IFS=',' read -ra include_list <<< "$INCLUDE_NODES"
        for node in "${include_list[@]}"; do
            node=$(echo "$node" | xargs)
            if kc get node "$node" &>/dev/null; then
                nodes_arr+=("$node")
                node_archs+=("$(get_node_arch "$node")")
            else
                log_warn "Node '$node' not found, skipping"
            fi
        done
    else
        for node in $(kc get nodes -o jsonpath='{.items[*].metadata.name}'); do
            local excluded=false
            for ex in "${exclude_arr[@]}"; do
                ex=$(echo "$ex" | xargs)
                [[ "$node" == "$ex" ]] && excluded=true && break
            done
            $excluded && continue
            nodes_arr+=("$node")
            node_archs+=("$(get_node_arch "$node")")
        done
    fi

    local node_count=${#nodes_arr[@]}
    if [[ $node_count -eq 0 ]]; then
        log_error "No nodes found"; exit 1
    fi

    log_info "Nodes ($node_count):"
    for i in "${!nodes_arr[@]}"; do
        printf "  - %s (%s)\n" "${nodes_arr[$i]}" "${node_archs[$i]}"
    done

    local total_ok=0
    local total_fail=0

    for image in "${images_arr[@]}"; do
        local img_arch=$(detect_image_arch "$image")

        local -a target_nodes=()
        for i in "${!nodes_arr[@]}"; do
            local narch="${node_archs[$i]}"
            [[ "$img_arch" == "multi" ]] || [[ "$img_arch" == "$narch" ]] && target_nodes+=("${nodes_arr[$i]}")
        done

        if [[ ${#target_nodes[@]} -eq 0 ]]; then
            log_info "Image $image ($img_arch): no compatible nodes"
            continue
        fi

        log_info "Image $image ($img_arch): ${#target_nodes[@]} nodes"

        local batch_start=0
        while [[ $batch_start -lt ${#target_nodes[@]} ]]; do
            local batch_end=$((batch_start + CONCURRENCY))
            [[ $batch_end -gt ${#target_nodes[@]} ]] && batch_end=${#target_nodes[@]}

            local pod_names=""
            for j in $(seq $batch_start $((batch_end-1))); do
                local pod_name
                pod_name=$(create_pod "${target_nodes[$j]}" "$image")
                pod_names="${pod_names:+$pod_names }$pod_name"
            done

            wait_for_pods "$pod_names" || true

            for pn in $pod_names; do
                local result=$(is_pod_failed "$pn")
                case "$result" in
                    succeeded) total_ok=$((total_ok+1)) ;;
                    *)         total_fail=$((total_fail+1)) ;;
                esac
            done

            delete_pods "$pod_names"

            batch_start=$batch_end
        done
    done

    log_info "=== Completed ==="
    log_info "Pods: $total_ok succeeded, $total_fail failed"
}

main "$@"