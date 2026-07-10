#!/bin/bash
# Clean script for test10-cp-artifacts
# Cleans up local artifact files downloaded during eval
# Params: WORKFLOW_NAME NAMESPACE KUBECONFIG WORKSPACE CP_ARTIFACTS_TEMP_FOLDER

WORKFLOW_NAME="$1"
NAMESPACE="$2"
KUBECONFIG="$3"
WORKSPACE="$4"
CP_ARTIFACTS_TEMP_FOLDER="$5"

echo "=========================================="
echo "CLEAN: test10-cp-artifacts"
echo "=========================================="

artifact_dest="${WORKSPACE}"

echo ""
echo "[1/2] Removing downloaded artifacts..."

if [ -d "$artifact_dest" ]; then
    rm -rf "$artifact_dest"
    echo "Removed: $artifact_dest"
else
    echo "No artifact directory found: $artifact_dest"
fi

echo ""
echo "[2/2] Verifying cleanup..."

if [ ! -d "$artifact_dest" ] || [ -z "$(ls -A "$artifact_dest" 2>/dev/null)" ]; then
    echo "PASS: Local artifacts cleaned up successfully"
else
    echo "WARN: Artifact directory may not be fully cleaned"
fi

echo ""
echo "=========================================="
echo "CLEAN: test10-cp-artifacts completed"
echo "=========================================="
exit 0
