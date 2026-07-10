#!/bin/bash
set -e

echo "=========================================="
echo "Git CDN Multi-Provider Clone Test"
echo "=========================================="

echo ""
echo "[1] Verifying git CDN config:"
git config --global --list | grep url.

echo ""
echo "[2] Cloning repos from different providers..."

mkdir -p /workspace/test-clones
cd /workspace/test-clones

echo ""
echo "--- GitCode ---"
if git clone --depth 1 https://gitcode.com/Ascend/RAGSDK.git gitcode-ragsdk 2>&1; then
    echo "✓ GitCode clone SUCCESS"
    ls -la gitcode-ragsdk | head -5
else
    echo "✗ GitCode clone FAILED"
fi

echo ""
echo "--- GitHub (via gh-proxy) ---"
if git clone --depth 1 https://gh-proxy.test.osinfra.cn/https://github.com/Ascend/fbgemm-ascend.git github-fbgemm 2>&1; then
    echo "✓ GitHub clone SUCCESS"
    ls -la github-fbgemm | head -5
else
    echo "✗ GitHub clone FAILED"
fi

echo ""
echo "--- Gitee ---"
if git clone --depth 1 https://gitee.com/ascend/samples.git gitee-samples 2>&1; then
    echo "✓ Gitee clone SUCCESS"
    ls -la gitee-samples | head -5
else
    echo "✗ Gitee clone FAILED"
fi

echo ""
echo "--- CodeHub ---"
if git clone --depth 1 https://codehub.devcloud.cn-north-4.huaweicloud.com/6a4fa7d010294da695be52110517c08b/CI_gitcode.git codehub-ci 2>&1; then
    echo "✓ CodeHub clone SUCCESS"
    ls -la codehub-ci | head -5
else
    echo "✗ CodeHub clone FAILED (expected - requires auth)"
fi

echo ""
echo "=========================================="
echo "Clone test completed"
echo "=========================================="
echo ""
echo "Final workspace structure:"
ls -la /workspace/test-clones