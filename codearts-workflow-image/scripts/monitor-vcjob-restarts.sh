#!/bin/bash
# Monitor pods created by Volcano Jobs that have restarted many times
# Usage: ./monitor-vcjob-restarts.sh [-k kubeconfig] [-n namespace] [-t threshold] [-w watch_interval]

set -euo pipefail

KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}"
NAMESPACE=""
THRESHOLD=1
WATCH_INTERVAL=10
ONESHOT=false

usage() {
    echo "Usage: $0 [-k kubeconfig] [-n namespace] [-t threshold] [-w interval] [--oneshot]"
    echo ""
    echo "  -k, --kubeconfig   Path to kubeconfig (default: ~/.kube/config)"
    echo "  -n, --namespace    Namespace to monitor (default: all namespaces)"
    echo "  -t, --threshold    Min restart count to report (default: 3)"
    echo "  -w, --watch        Watch interval in seconds (default: 10, 0 = oneshot)"
    echo "  --oneshot          Run once and exit"
    exit 1
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        -k|--kubeconfig) KUBECONFIG="$2"; shift 2 ;;
        -n|--namespace)  NAMESPACE="$2"; shift 2 ;;
        -t|--threshold)  THRESHOLD="$2"; shift 2 ;;
        -w|--watch)      WATCH_INTERVAL="$2"; shift 2 ;;
        --oneshot)       ONESHOT=true; shift ;;
        -h|--help)       usage ;;
        *) echo "Unknown option: $1"; usage ;;
    esac
done

if [[ "$ONESHOT" == "true" ]]; then
    WATCH_INTERVAL=0
fi

NS_FLAG=""
if [[ -n "$NAMESPACE" ]]; then
    NS_FLAG="-n $NAMESPACE"
fi

check_restarts() {
    local timestamp
    timestamp=$(date '+%Y-%m-%d %H:%M:%S')

    # Get all pods owned by Volcano Jobs (label: volcano.sh/job-name)
    local pods
    pods=$(kubectl get pods $NS_FLAG --kubeconfig "$KUBECONFIG" \
        -l 'volcano.sh/job-name' \
        -o json 2>/dev/null)

    if [[ -z "$pods" ]] || [[ "$pods" == "{}" ]]; then
        return
    fi

    local has_issue=false

    echo "$pods" | jq -r --argjson threshold "$THRESHOLD" '
        .items[] |
        select(
            (.status.containerStatuses != null) and
            ([.status.containerStatuses[].restartCount] | max >= $threshold)
        ) |
        {
            namespace: .metadata.namespace,
            pod: .metadata.name,
            vcjob: .metadata.labels["volcano.sh/job-name"],
            phase: .status.phase,
            containers: [.status.containerStatuses[] | {name: .name, restarts: .restartCount, ready: .ready, state: (.state | keys[0])}]
        }
    ' | while IFS= read -r line; do
        if [[ -n "$line" ]]; then
            has_issue=true
            echo "$line"
        fi
    done

    # Also check init containers
    echo "$pods" | jq -r --argjson threshold "$THRESHOLD" '
        .items[] |
        select(
            (.status.initContainerStatuses != null) and
            ([.status.initContainerStatuses[].restartCount] | max >= $threshold)
        ) |
        {
            namespace: .metadata.namespace,
            pod: .metadata.name,
            vcjob: .metadata.labels["volcano.sh/job-name"],
            phase: .status.phase,
            init_containers: [.status.initContainerStatuses[] | {name: .name, restarts: .restartCount, state: (.state | keys[0])}]
        }
    ' | while IFS= read -r line; do
        if [[ -n "$line" ]]; then
            has_issue=true
            echo "$line"
        fi
    done
}

if [[ "$WATCH_INTERVAL" -eq 0 ]]; then
    echo "=== Volcano Job Pod Restart Monitor (oneshot, threshold >= ${THRESHOLD}) ==="
    echo ""
    check_restarts
    echo ""
    echo "Done."
else
    echo "=== Volcano Job Pod Restart Monitor ==="
    echo "Threshold: >= ${THRESHOLD} restarts | Interval: ${WATCH_INTERVAL}s"
    echo "Press Ctrl+C to stop"
    echo ""

    while true; do
        clear 2>/dev/null || true
        echo "=== Volcano Job Pod Restart Monitor ==="
        echo "Time: $(date '+%Y-%m-%d %H:%M:%S') | Threshold: >= ${THRESHOLD} restarts"
        echo ""

        check_restarts

        echo ""
        echo "Refreshing every ${WATCH_INTERVAL}s..."
        sleep "$WATCH_INTERVAL"
    done
fi
