#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KUBECONFIG_DIR="$HOME/.kube"

KUBECONFIGS=("gy-001.yaml" "wlcb-001.yaml")
CONCURRENT_LIMIT=5
NAMESPACE="argo"
DRY_RUN=false
AUTO_PROCEED=false

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

log_info() { echo -e "${CYAN}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }

check_kubeconfig() {
    for kc in "${KUBECONFIGS[@]}"; do
        local path="${KUBECONFIG_DIR}/${kc}"
        if [[ ! -f "$path" ]]; then
            log_error "Kubeconfig not found: $path"
            return 1
        fi
        log_info "Kubeconfig found: $kc"
    done
}

monitor_harbor_pods() {
    local kubeconfig="$1"
    local cluster_name=$(basename "$kubeconfig" .yaml)
    
    export KUBECONFIG="${KUBECONFIG_DIR}/${kubeconfig}"
    
    log_info "Monitoring Harbor pods in cluster: $cluster_name"
    
    local harbor_namespace="harbor"
    local harbor_components=("harbor-core" "harbor-jobservice" "harbor-nginx" "harbor-portal" "harbor-registry")
    local unhealthy_count=0
    local slow_count=0
    
    for component in "${harbor_components[@]}"; do
        local pod_status=$(kubectl get pods -n "$harbor_namespace" -l app="$component" -o jsonpath='{.items[0].status.phase}' 2>/dev/null)
        local ready=$(kubectl get pods -n "$harbor_namespace" -l app="$component" -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}' 2>/dev/null)
        
        if [[ "$pod_status" != "Running" ]] || [[ "$ready" != "True" ]]; then
            log_warn "Harbor component $component is not healthy: Status=$pod_status, Ready=$ready"
            ((unhealthy_count++))
        else
            log_success "Harbor $component is Running and Ready"
        fi
        
        local restarts=$(kubectl get pods -n "$harbor_namespace" -l app="$component" -o jsonpath='{.items[0].status.containerStatuses[0].restartCount}' 2>/dev/null)
        if [[ -n "$restarts" ]] && [[ "$restarts" -gt 0 ]]; then
            log_warn "Harbor $component has restarted $restarts times"
            ((slow_count++))
        fi
    done
    
    log_info "Checking Harbor network metrics..."
    local nginx_pod=$(kubectl get pods -n "$harbor_namespace" -l app=harbor-nginx -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    
    if [[ -n "$nginx_pod" ]]; then
        local network_rx=$(kubectl exec -n "$harbor_namespace" "$nginx_pod" -- cat /proc/net/dev 2>/dev/null | grep -E "eth0|ens" | awk '{print $2}')
        local network_tx=$(kubectl exec -n "$harbor_namespace" "$nginx_pod" -- cat /proc/net/dev 2>/dev/null | grep -E "eth0|ens" | awk '{print $10}')
        
        if [[ -n "$network_rx" ]] && [[ -n "$network_tx" ]]; then
            local rx_mb=$((network_rx / 1024 / 1024))
            local tx_mb=$((network_tx / 1024 / 1024))
            log_info "Harbor nginx network: RX=${rx_mb}MB, TX=${tx_mb}MB"
        fi
    fi
    
    local registry_pod=$(kubectl get pods -n "$harbor_namespace" -l app=harbor-registry -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [[ -n "$registry_pod" ]]; then
        local registry_logs=$(kubectl logs -n "$harbor_namespace" "$registry_pod" --tail=50 2>/dev/null | grep -E "error|slow|timeout|failed" | wc -l)
        if [[ "$registry_logs" -gt 0 ]]; then
            log_warn "Harbor registry has $registry_logs error/slow indicators in recent logs"
            ((slow_count++))
        fi
    fi
    
    if [[ $unhealthy_count -gt 0 ]] || [[ $slow_count -gt 0 ]]; then
        log_warn "Harbor may have performance issues (unhealthy: $unhealthy_count, slow indicators: $slow_count)"
        return 0
    fi
    
    log_success "Harbor pods are healthy"
    return 1
}

get_pending_pods() {
    local kubeconfig="$1"
    local cluster_name=$(basename "$kubeconfig" .yaml)
    
    export KUBECONFIG="${KUBECONFIG_DIR}/${kubeconfig}"
    
    log_info "Checking pending pods in cluster: $cluster_name"
    
    local pending_pods=$(kubectl get pods -A --field-selector=status.phase=Pending -o json 2>/dev/null)
    
    if [[ -z "$pending_pods" ]] || echo "$pending_pods" | jq -e '.items | length == 0' >/dev/null 2>&1; then
        log_info "No pending pods in cluster: $cluster_name"
        return
    fi
    
    echo "$pending_pods" | jq -r '.items[] | select(.spec.containers != null) | .spec.containers[].image' 2>/dev/null | sort -u
}

extract_images_from_pending_pods() {
    local all_images=()
    
    for kc in "${KUBECONFIGS[@]}"; do
        local images=$(get_pending_pods "$kc")
        if [[ -n "$images" ]]; then
            while IFS= read -r img; do
                if [[ -n "$img" ]] && [[ "$img" == *"harbor-portal"* ]]; then
                    all_images+=("$img")
                fi
            done <<< "$images"
        fi
    done
    
    printf '%s\n' "${all_images[@]}" | sort -u
}

generate_volcano_job() {
    local image="$1"
    local job_name="imagepull-$(echo "$image" | sed 's/[:/]/-/g' | cut -c1-50)"
    local arch=$(echo "$image" | grep -q "aarch64" && echo "arm64" || echo "amd64")
    
    cat <<EOF
apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
    generateName: ${job_name}-
    labels:
        jobRepositoryName: imagepull-harbor
        kubernetes.io/arch: ${arch}
spec:
    policies:
        - event: PodFailed
          action: AbortJob
    queue: shared-flexible-queue
    maxRetry: 0
    ttlSecondsAfterFinished: 1800
    tasks:
        - name: main-script
          replicas: 1
          template:
            spec:
                containers:
                    - name: ascend
                      image: ${image}
                      command:
                        - bash
                        - -c
                      args:
                        - |
                          echo "Pulling image: ${image}"
                          echo "Image pull completed successfully"
                          date
                      workingDir: /workspace
                      resources:
                        limits:
                            memory: 1Gi
                        requests:
                            cpu: "1"
                            memory: 1Gi
                      env:
                        - name: WORKSPACE
                          value: /workspace
                nodeSelector:
                    kubernetes.io/arch: ${arch}
                imagePullSecrets:
                    - name: huawei-swr-image-pull-secret-model-gy
                activeDeadlineSeconds: 600
                securityContext:
                    runAsUser: 0
                restartPolicy: Never
EOF
}

submit_image_pull_job() {
    local image="$1"
    local kubeconfig="$2"
    local cluster_name=$(basename "$kubeconfig" .yaml)
    
    export KUBECONFIG="${KUBECONFIG_DIR}/${kubeconfig}"
    
    local job_yaml=$(generate_volcano_job "$image")
    local temp_file="/tmp/${cluster_name}-imagepull-$RANDOM.yaml"
    
    echo "$job_yaml" > "$temp_file"
    
    log_info "Submitting image pull job for: $image in cluster: $cluster_name"
    
    if kubectl apply -f "$temp_file" -n "$NAMESPACE" 2>/dev/null; then
        log_success "Job submitted successfully for: $image"
    else
        log_error "Failed to submit job for: $image"
    fi
    
    rm -f "$temp_file"
}

pull_images_concurrent() {
    local images="$1"
    local count=0
    local pids=()
    
    while IFS= read -r image; do
        [[ -z "$image" ]] && continue
        
        for kc in "${KUBECONFIGS[@]}"; do
            if [[ $count -ge $CONCURRENT_LIMIT ]]; then
                wait -n 2>/dev/null || wait "${pids[0]}"
                pids=("${pids[@]:1}")
                ((count--))
            fi
            
            submit_image_pull_job "$image" "$kc" &
            pids+=($!)
            ((count++))
            
            log_info "Started pulling: $image (concurrent: $count)"
        done
    done <<< "$images"
    
    for pid in "${pids[@]}"; do
        wait "$pid" 2>/dev/null || true
    done
    
    log_success "All image pull jobs submitted"
}

show_status() {
    log_info "Current image pull job status:"
    
    for kc in "${KUBECONFIGS[@]}"; do
        export KUBECONFIG="${KUBECONFIG_DIR}/${kc}"
        local cluster_name=$(basename "$kc" .yaml)
        
        echo ""
        log_info "Cluster: $cluster_name"
        kubectl get pods -n "$NAMESPACE" -l jobRepositoryName=imagepull-harbor 2>/dev/null || echo "No pods found"
    done
}

main() {
    echo ""
    echo "=========================================="
    echo "  Harbor Image Pull Helper"
    echo "=========================================="
    echo ""
    
    check_kubeconfig || exit 1
    
    echo ""
    
    local harbor_issues=false
    for kc in "${KUBECONFIGS[@]}"; do
        if monitor_harbor_pods "$kc"; then
            harbor_issues=true
        fi
    done
    
    echo ""
    
    if [[ "$harbor_issues" == "true" ]]; then
        log_warn "Harbor performance issues detected, proceeding with concurrent image pull..."
    else
        log_info "Harbor pods are healthy, but will still check for pending pods..."
    fi
    
    echo ""
    
    log_info "Extracting images from pending pods..."
    local images=$(extract_images_from_pending_pods)
    
    if [[ -z "$images" ]]; then
        log_info "No harbor images found in pending pods"
        
        read -p "Do you want to pull specific images manually? (y/n): " answer
        if [[ "$answer" == "y" ]]; then
            read -p "Enter images (comma-separated): " manual_images
            images=$(echo "$manual_images" | tr ',' '\n' | sort -u)
        fi
    fi
    
    if [[ -n "$images" ]]; then
        echo ""
        log_info "Images to pull:"
        echo "$images"
        
        echo ""
        read -p "Proceed with concurrent image pull? (y/n): " proceed
        
        if [[ "$proceed" == "y" ]]; then
            pull_images_concurrent "$images"
            
            echo ""
            show_status
        fi
    fi
    
    echo ""
    log_success "Script completed"
}

main "$@"