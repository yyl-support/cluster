#!/bin/sh
echo "=== GOPROXY Test ==="
go version
go env GOPROXY
echo "GOPROXY_ENV=$GOPROXY"
git clone https://gitcode.com/Ascend/mind-cluster.git
cd /workspace/mind-cluster/component/ascend-operator
cat go.mod | head -3
START=$(date +%s%N)
go clean -modcache
go mod tidy
END=$(date +%s%N)
DURATION=$(( ($END - $START) / 1000000 ))
echo "go mod tidy took ${DURATION}ms"
echo "=== GOPROXY Test Completed ==="
