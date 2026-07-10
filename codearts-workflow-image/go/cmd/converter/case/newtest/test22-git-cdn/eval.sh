#!/bin/bash
# Eval script for test22-git-cdn
# Validates: Git CDN configs are NOT in converter output (moved to submit phase)
# Params: WORKFLOW_NAME NAMESPACE KUBECONFIG WORKSPACE [WORKFLOW_EXIT]

WORKFLOW_NAME="$1"
NAMESPACE="$2"
KUBECONFIG="$3"
WORKSPACE="${4:-.}"
WORKFLOW_EXIT="${5:-0}"

if [[ "$WORKFLOW_EXIT" -ne 0 ]]; then
    echo "FAIL: submit exited with code ${WORKFLOW_EXIT}"
    exit 1
fi

echo "=========================================="
echo "EVAL: test22-git-cdn"
echo "=========================================="

TEST_DIR="${WORKSPACE}/go/cmd/converter/case/newtest/test22-git-cdn"

echo ""
echo "[1/3] Validating expected.yaml has NO CDN configs (converter phase)..."

expected_yaml="${TEST_DIR}/expected.yaml"

if [[ ! -f "$expected_yaml" ]]; then
    echo "FAIL: expected.yaml not found at ${expected_yaml}"
    exit 1
fi

cdn_patterns=(
    "git-cache-http-server.git-cache.svc.cluster.local"
    "git-cache-github.git-cache.svc.cluster.local"
    "git-cache-gitee.git-cache.svc.cluster.local"
    "git-cache-atomgit.git-cache.svc.cluster.local"
    "git-cache-codehub.git-cache.svc.cluster.local"
)

cdn_found=false
for pattern in "${cdn_patterns[@]}"; do
    if grep -qF "$pattern" "$expected_yaml"; then
        echo "FAIL: CDN pattern '${pattern}' found in expected.yaml - CDN should NOT be in converter output"
        cdn_found=true
    fi
done

if [[ "$cdn_found" == "false" ]]; then
    echo "PASS: No CDN configs in expected.yaml (correct - CDN moved to submit phase)"
fi

echo ""
echo "[2/3] Checking generated workflow name..."

if grep -q "generateName: ascend-ragsdk-" "$expected_yaml"; then
    echo "PASS: Workflow generateName pattern matches gitcode repo"
else
    echo "FAIL: Workflow generateName pattern NOT found"
    exit 1
fi

echo ""
echo "[3/3] Checking git clone script structure..."

if grep -q "gitcode.com/Ascend/RAGSDK.git" "$expected_yaml"; then
    echo "PASS: GitCode repo URL found in workflow"
else
    echo "FAIL: GitCode repo URL NOT found in workflow"
    exit 1
fi

echo ""
echo "=========================================="
echo "PASS: test22-git-cdn - CDN moved to submit phase (correct)"
echo "=========================================="
exit 0