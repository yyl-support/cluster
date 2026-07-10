#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "Running gosec with golang:1.24-alpine..."

docker run --rm \
    -v "$PROJECT_ROOT:/app" \
    -w /app \
    -e GOPROXY=https://goproxy.cn,direct \
    -e GOTOOLCHAIN=auto \
    golang:1.24-alpine \
    sh -c "
        apk add --no-cache git gcc musl-dev && \
        CGO_ENABLED=0 GOOS=linux go install github.com/securego/gosec/v2/cmd/gosec@latest && \
        cd go && \
        /go/bin/gosec -no-fail ./...
    "

echo "Gosec completed."
