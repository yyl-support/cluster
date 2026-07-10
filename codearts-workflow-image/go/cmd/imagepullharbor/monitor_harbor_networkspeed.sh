#!/bin/bash

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KUBECONFIG_DIR="$HOME/.kube"

KUBECONFIGS=("gy-001.yaml" "wlcb-001.yaml")
HARBOR_NAMESPACE="harbor"
INTERVAL_SECONDS=2
SAMPLE_COUNT=10
THRESHOLD_MB=10

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${CYAN}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
log_data() { echo -e "${BLUE}[DATA]${NC} $1"; }

get_registry_pods() {
    local kubeconfig="$1"
    
    export KUBECONFIG="${KUBECONFIG_DIR}/${kubeconfig}"
    
    kubectl get pods -n "$HARBOR_NAMESPACE" -l component=registry -o jsonpath='{.items[*].metadata.name}' 2>/dev/null
}

get_network_stats() {
    local pod_name="$1"
    local kubeconfig="$2"
    
    export KUBECONFIG="${KUBECONFIG_DIR}/${kubeconfig}"
    
    kubectl exec -n "$HARBOR_NAMESPACE" "$pod_name" -- cat /proc/net/dev 2>/dev/null | grep -E "eth0|ens" | head -1
}

parse_network_bytes() {
    local stats_line="$1"
    
    echo "$stats_line" | awk '{print $2, $10}'
}

calculate_speed_mb() {
    local rx1="$1"
    local tx1="$2"
    local rx2="$3"
    local tx2="$4"
    local interval="$5"
    
    awk -v rx1="$rx1" -v tx1="$tx1" -v rx2="$rx2" -v tx2="$tx2" -v interval="$interval" 'BEGIN {
        rx_diff = rx2 - rx1
        tx_diff = tx2 - tx1
        rx_speed_mb = rx_diff / interval / 1024 / 1024
        tx_speed_mb = tx_diff / interval / 1024 / 1024
        printf "%.2f %.2f %.0f %.0f", rx_speed_mb, tx_speed_mb, rx_diff/interval/1024, tx_diff/interval/1024
    }'
}

monitor_registry() {
    local kubeconfig="$1"
    local cluster_name=$(basename "$kubeconfig" .yaml)
    
    export KUBECONFIG="${KUBECONFIG_DIR}/${kubeconfig}"
    
    echo ""
    echo "=========================================="
    log_info "Cluster: $cluster_name"
    echo "=========================================="
    
    local pods=$(get_registry_pods "$kubeconfig")
    
    if [[ -z "$pods" ]]; then
        log_error "Harbor registry pods not found in cluster: $cluster_name"
        return 1
    fi
    
    local pod_array=($pods)
    local pod_count=${#pod_array[@]}
    
    log_info "Found $pod_count registry pod(s): ${pod_array[*]}"
    echo ""
    
    declare -A pod_rx1
    declare -A pod_tx1
    declare -A pod_rx_samples
    declare -A pod_tx_samples
    
    for pod in "${pod_array[@]}"; do
        local stats=$(get_network_stats "$pod" "$kubeconfig")
        if [[ -z "$stats" ]]; then
            log_error "Cannot get network stats from pod: $pod"
            return 1
        fi
        
        local bytes=$(parse_network_bytes "$stats")
        pod_rx1[$pod]=$(echo "$bytes" | awk '{print $1}')
        pod_tx1[$pod]=$(echo "$bytes" | awk '{print $2}')
        
        local total_rx_mb=$(awk -v rx="${pod_rx1[$pod]}" 'BEGIN {printf "%.2f", rx/1024/1024}')
        local total_tx_mb=$(awk -v tx="${pod_tx1[$pod]}" 'BEGIN {printf "%.2f", tx/1024/1024}')
        log_data "Pod $pod initial: Total RX=${total_rx_mb}MB, Total TX=${total_tx_mb}MB"
        
        pod_rx_samples[$pod]=""
        pod_tx_samples[$pod]=""
    done
    
    echo ""
    echo "Time                        Pod                                RX (MB/s)      TX (MB/s)"
    echo "                           [Download from upstream]          [Send to client]"
    echo "--------------------------------------------------------------------------------------------"
    
    for i in $(seq 1 $SAMPLE_COUNT); do
        sleep "$INTERVAL_SECONDS"
        
        local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
        
        for pod in "${pod_array[@]}"; do
            local stats2=$(get_network_stats "$pod" "$kubeconfig")
            local bytes2=$(parse_network_bytes "$stats2")
            local rx2=$(echo "$bytes2" | awk '{print $1}')
            local tx2=$(echo "$bytes2" | awk '{print $2}')
            
            local speeds=$(calculate_speed_mb "${pod_rx1[$pod]}" "${pod_tx1[$pod]}" "$rx2" "$tx2" "$INTERVAL_SECONDS")
            local rx_mb=$(echo "$speeds" | awk '{print $1}')
            local tx_mb=$(echo "$speeds" | awk '{print $2}')
            
            printf "%-24s  %-32s    %-12s    %-12s\n" "$timestamp" "$pod" "$rx_mb" "$tx_mb"
            
            pod_rx_samples[$pod]="${pod_rx_samples[$pod]} $rx_mb"
            pod_tx_samples[$pod]="${pod_tx_samples[$pod]} $tx_mb"
            
            pod_rx1[$pod]=$rx2
            pod_tx1[$pod]=$tx2
        done
    done
    
    echo ""
    echo "=========================================="
    log_info "Summary (Average of all registry pods)"
    echo "=========================================="
    
    local all_rx_samples=""
    local all_tx_samples=""
    
    for pod in "${pod_array[@]}"; do
        local avg_rx=$(echo "${pod_rx_samples[$pod]}" | awk '{sum=0; for(i=1;i<=NF;i++) sum+=$i; printf "%.2f", sum/NF}')
        local avg_tx=$(echo "${pod_tx_samples[$pod]}" | awk '{sum=0; for(i=1;i<=NF;i++) sum+=$i; printf "%.2f", sum/NF}')
        local max_rx=$(echo "${pod_rx_samples[$pod]}" | awk '{max=0; for(i=1;i<=NF;i++) if($i>max) max=$i; print max}')
        local max_tx=$(echo "${pod_tx_samples[$pod]}" | awk '{max=0; for(i=1;i<=NF;i++) if($i>max) max=$i; print max}')
        
        log_data "$pod: RX Avg=${avg_rx} MB/s (Max=${max_rx}), TX Avg=${avg_tx} MB/s (Max=${max_tx})"
        
        all_rx_samples="$all_rx_samples ${pod_rx_samples[$pod]}"
        all_tx_samples="$all_tx_samples ${pod_tx_samples[$pod]}"
    done
    
    local combined_avg_rx=$(echo "$all_rx_samples" | awk '{sum=0; for(i=1;i<=NF;i++) sum+=$i; printf "%.2f", sum/NF}')
    local combined_avg_tx=$(echo "$all_tx_samples" | awk '{sum=0; for(i=1;i<=NF;i++) sum+=$i; printf "%.2f", sum/NF}')
    local combined_max_rx=$(echo "$all_rx_samples" | awk '{max=0; for(i=1;i<=NF;i++) if($i>max) max=$i; print max}')
    local combined_max_tx=$(echo "$all_tx_samples" | awk '{max=0; for(i=1;i<=NF;i++) if($i>max) max=$i; print max}')
    
    echo ""
    log_info "Combined Average (all pods):"
    log_data "  RX (from upstream): Avg=${combined_avg_rx} MB/s, Max=${combined_max_rx} MB/s"
    log_data "  TX (to clients):    Avg=${combined_avg_tx} MB/s, Max=${combined_max_tx} MB/s"
    
    echo ""
    
    if awk -v avg="$combined_avg_rx" -v threshold="$THRESHOLD_MB" 'BEGIN {exit !(avg < threshold)}'; then
        log_warn "⚠️  SLOW: Registry RX ${combined_avg_rx} MB/s < ${THRESHOLD_MB} MB/s threshold"
        log_warn "    Network to upstream (SWR/Docker Hub) is slow"
        log_warn "    Recommendation: Trigger concurrent image pull"
        echo ""
        log_warn "RESULT: SLOW - Need concurrent pull"
        return 0
    else
        log_success "✅ GOOD: Registry RX ${combined_avg_rx} MB/s >= ${THRESHOLD_MB} MB/s threshold"
        log_success "    Network to upstream is fast"
        echo ""
        log_success "RESULT: GOOD - Network is acceptable"
        return 1
    fi
}

monitor_continuous() {
    local kubeconfig="$1"
    local cluster_name=$(basename "$kubeconfig" .yaml)
    
    export KUBECONFIG="${KUBECONFIG_DIR}/${kubeconfig}"
    
    local pods=$(get_registry_pods "$kubeconfig")
    
    if [[ -z "$pods" ]]; then
        log_error "Registry pods not found"
        return 1
    fi
    
    local pod_array=($pods)
    
    echo ""
    log_info "Continuous monitoring: ${pod_array[*]} (Ctrl+C to stop)"
    echo ""
    echo "Time                        Pod                                RX (MB/s)      TX (MB/s)"
    echo "--------------------------------------------------------------------------------------------"
    
    declare -A pod_rx1
    declare -A pod_tx1
    
    for pod in "${pod_array[@]}"; do
        local stats=$(get_network_stats "$pod" "$kubeconfig")
        local bytes=$(parse_network_bytes "$stats")
        pod_rx1[$pod]=$(echo "$bytes" | awk '{print $1}')
        pod_tx1[$pod]=$(echo "$bytes" | awk '{print $2}')
    done
    
    while true; do
        sleep "$INTERVAL_SECONDS"
        
        local timestamp=$(date '+%Y-%m-%d %H:%M:%S')
        
        for pod in "${pod_array[@]}"; do
            local stats2=$(get_network_stats "$pod" "$kubeconfig")
            local bytes2=$(parse_network_bytes "$stats2")
            local rx2=$(echo "$bytes2" | awk '{print $1}')
            local tx2=$(echo "$bytes2" | awk '{print $2}')
            
            local speeds=$(calculate_speed_mb "${pod_rx1[$pod]}" "${pod_tx1[$pod]}" "$rx2" "$tx2" "$INTERVAL_SECONDS")
            local rx_mb=$(echo "$speeds" | awk '{print $1}')
            local tx_mb=$(echo "$speeds" | awk '{print $2}')
            
            printf "%-24s  %-32s    %-12s    %-12s\n" "$timestamp" "$pod" "$rx_mb" "$tx_mb"
            
            pod_rx1[$pod]=$rx2
            pod_tx1[$pod]=$tx2
        done
    done
}

usage() {
    echo "Usage: $0 [options]"
    echo ""
    echo "Monitor Harbor Registry network speed (RX/TX) - All registry pods"
    echo ""
    echo "Metrics:"
    echo "  RX - Download speed from upstream (SWR/Docker Hub)"
    echo "  TX - Send speed to clients (kubectl/docker)"
    echo ""
    echo "Threshold: ${THRESHOLD_MB} MB/s"
    echo ""
    echo "Options:"
    echo "  -k, --kubeconfig <file>   Specific kubeconfig (default: all clusters)"
    echo "  -i, --interval <sec>      Sampling interval (default: 2)"
    echo "  -n, --samples <count>     Number of samples (default: 10)"
    echo "  --continuous              Continuous monitoring mode"
    echo "  -h, --help                Show help"
    echo ""
    echo "Examples:"
    echo "  $0                              Monitor all clusters"
    echo "  $0 -k gy-001.yaml               Monitor gy-001 only"
    echo "  $0 -k gy-001.yaml -n 5          5 samples on gy-001"
    echo "  $0 -k gy-001.yaml --continuous  Continuous monitor"
}

parse_args() {
    local selected_kubeconfig=""
    local continuous=false
    
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -k|--kubeconfig)
                selected_kubeconfig="$2"
                shift 2
                ;;
            -i|--interval)
                INTERVAL_SECONDS="$2"
                shift 2
                ;;
            -n|--samples)
                SAMPLE_COUNT="$2"
                shift 2
                ;;
            --continuous)
                continuous=true
                shift
                ;;
            -h|--help)
                usage
                exit 0
                ;;
            *)
                log_error "Unknown option: $1"
                usage
                exit 1
                ;;
        esac
    done
    
    if [[ "$continuous" == "true" ]]; then
        if [[ -n "$selected_kubeconfig" ]]; then
            monitor_continuous "$selected_kubeconfig"
            exit 0
        else
            log_error "Continuous mode requires -k option"
            usage
            exit 1
        fi
    fi
    
    if [[ -n "$selected_kubeconfig" ]]; then
        KUBECONFIGS=("$selected_kubeconfig")
    fi
}

main() {
    echo ""
    echo "=========================================="
    echo "  Harbor Registry Network Monitor"
    echo "=========================================="
    
    parse_args "$@"
    
    local slow_detected=false
    
    for kc in "${KUBECONFIGS[@]}"; do
        if monitor_registry "$kc"; then
            slow_detected=true
        fi
    done
    
    echo ""
    echo "=========================================="
    if [[ "$slow_detected" == "true" ]]; then
        log_warn "⚠️  FINAL RESULT: Network to upstream is SLOW"
        log_warn "    Action: Run image_pull_harbor.sh to pull images concurrently"
    else
        log_success "✅ FINAL RESULT: Network to upstream is GOOD"
    fi
    echo "=========================================="
}

main "$@"