#!/bin/bash
set -e

KUBECONFIG=~/.kube/karmada-proxy.config
NAMESPACE=argo
YAML_FILE=go/cmd/test-log.yaml

echo "=== Step 1: Submit job ==="
kubectl create -f "$YAML_FILE" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG"

echo ""
echo "=== Step 2: Wait for job creation (5s) ==="
sleep 5

echo ""
echo "=== Step 3: Get job name ==="
JOB_NAME=$(kubectl get job.batch.volcano.sh -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -l test=log-test -o jsonpath='{.items[0].metadata.name}')
echo "Job name: $JOB_NAME"

if [ -z "$JOB_NAME" ]; then
    echo "ERROR: Job not found"
    exit 1
fi

echo ""
echo "=== Step 4: Wait for pod to appear ==="
echo "Waiting for pod with label volcano.sh/job-name=$JOB_NAME..."
while true; do
    POD_NAME=$(kubectl get pods -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -l volcano.sh/job-name="$JOB_NAME" -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [ -n "$POD_NAME" ]; then
        echo "Found pod: $POD_NAME"
        break
    fi
    echo "  Pod not found, waiting 2s..."
    sleep 2
done

echo ""
echo "=== Step 5: Wait for pod Running ==="
while true; do
    PHASE=$(kubectl get pod "$POD_NAME" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o jsonpath='{.status.phase}' 2>/dev/null)
    echo "  Pod phase: $PHASE"
    if [ "$PHASE" == "Running" ] || [ "$PHASE" == "Succeeded" ] || [ "$PHASE" == "Failed" ]; then
        break
    fi
    sleep 2
done

echo ""
echo "=== Step 6: Stream logs ==="
echo "kubectl logs $POD_NAME -n $NAMESPACE --kubeconfig $KUBECONFIG"
kubectl logs "$POD_NAME" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" --follow --all-containers=true || echo "Logs finished"

echo ""
echo "=== Step 7: Check final job status ==="
kubectl get job.batch.volcano.sh "$JOB_NAME" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o wide

echo ""
echo "=== Step 8: Cleanup ==="
kubectl delete job.batch.volcano.sh "$JOB_NAME" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" --ignore-not-found=true
echo "Job deleted"