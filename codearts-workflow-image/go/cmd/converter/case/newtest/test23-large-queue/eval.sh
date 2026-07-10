#!/bin/bash

set -e

NAMESPACE="${NAMESPACE:-argo}"
KUBECONFIG="${KUBECONFIG:-$HOME/.kube/config}"
WORKFLOW_NAME="${WORKFLOW_NAME:-}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
expected_yaml="$SCRIPT_DIR/expected.yaml"

echo "=========================================="
echo "EVAL: test23-large-queue"
echo "=========================================="

echo ""
echo "[1/2] Validating expected.yaml has large-task-shared-queue..."

if grep -q "queue: large-task-shared-queue" "$expected_yaml"; then
    echo "PASS: queue 'large-task-shared-queue' found in expected.yaml"
else
    echo "FAIL: queue 'large-task-shared-queue' NOT found in expected.yaml"
    exit 1
fi

echo ""
echo "[2/3] Validating memory=8Gi..."

if grep -q "memory: 8Gi" "$expected_yaml"; then
    echo "PASS: memory=8Gi found"
else
    echo "FAIL: memory=8Gi NOT found"
    exit 1
fi

echo ""
echo "[3/3] Validating arm64 architecture..."

if grep -q "kubernetes.io/arch: arm64" "$expected_yaml"; then
    echo "PASS: arm64 architecture found"
else
    echo "FAIL: arm64 architecture NOT found"
    exit 1
fi

echo ""
echo "=========================================="
echo "PASS: test23-large-queue - All validations passed"
echo "=========================================="
exit 0