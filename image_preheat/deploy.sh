#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
NAMESPACE="${NAMESPACE:-preheat}"
IMAGE_NAME="${IMAGE_NAME:-harbor-preheat-webhook}"
IMAGE_TAG="${IMAGE_TAG:-v1}"
REGISTRY="${REGISTRY:-}"

echo "=== 部署 Karmada 镜像预热服务 ==="
echo ""
echo "前提: Vault 已配置，能注入 kubeconfig 到 Pod"
echo ""

cd "$SCRIPT_DIR"

if [[ -n "$REGISTRY" ]]; then
    FULL_IMAGE="${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"
else
    FULL_IMAGE="${IMAGE_NAME}:${IMAGE_TAG}"
fi

echo "镜像: $FULL_IMAGE"

echo ""
echo "步骤 1: 创建命名空间"
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f -

echo ""
echo "步骤 2: 更新镜像地址"
sed -i "s|image: harbor-preheat-webhook:v1|image: ${FULL_IMAGE}|g" deploy/deployment.yaml

echo ""
echo "步骤 3: 部署"
kubectl apply -f deploy/deployment.yaml
kubectl apply -f deploy/service.yaml

echo ""
echo "步骤 4: 等待就绪"
kubectl rollout status deployment/harbor-preheat-webhook -n "$NAMESPACE" --timeout=120s || true

echo ""
echo "=== 完成 ==="
kubectl get pods -n "$NAMESPACE" -l app=harbor-preheat-webhook
echo ""
echo "查看日志:"
echo "  kubectl logs -n $NAMESPACE -l app=harbor-preheat-webhook -f"