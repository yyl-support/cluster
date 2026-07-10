#!/bin/bash

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
IMAGE_NAME="${IMAGE_NAME:-harbor-preheat-webhook}"
IMAGE_TAG="${IMAGE_TAG:-v1}"
REGISTRY="${REGISTRY:-}"

echo "=== 构建镜像预热服务 ==="
echo "镜像名称: $IMAGE_NAME"
echo "镜像标签: $IMAGE_TAG"
echo "仓库地址: ${REGISTRY:-local}"

cd "$SCRIPT_DIR"

docker build -t "${IMAGE_NAME}:${IMAGE_TAG}" .

if [[ -n "$REGISTRY" ]]; then
    full_image="${REGISTRY}/${IMAGE_NAME}:${IMAGE_TAG}"
    docker tag "${IMAGE_NAME}:${IMAGE_TAG}" "$full_image"
    echo "推送镜像到: $full_image"
    docker push "$full_image"
fi

echo "=== 构建完成 ==="
echo "镜像: ${IMAGE_NAME}:${IMAGE_TAG}"