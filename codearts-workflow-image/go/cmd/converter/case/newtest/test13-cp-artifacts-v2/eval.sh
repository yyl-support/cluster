#!/bin/bash
# Eval script for test13-cp-artifacts-v2 (copy-script approach)
# Params: WORKFLOW_NAME NAMESPACE KUBECONFIG [WORKSPACE] [CP_ARTIFACTS_TEMP_FOLDER] [WORKFLOW_EXIT]

WORKFLOW_NAME="$1"
NAMESPACE="$2"
KUBECONFIG="$3"
WORKSPACE="${4:-.}"
CP_ARTIFACTS_TEMP_FOLDER="${5:-}"
WORKFLOW_EXIT="${6:-0}"

if [[ "$WORKFLOW_EXIT" -ne 0 ]]; then
    echo "FAIL: submit exited with code ${WORKFLOW_EXIT}"
    exit 1
fi

echo "=========================================="
echo "EVAL: test13-cp-artifacts-v2 (copy-script approach)"
echo "=========================================="

echo ""
echo "[1/3] Validating artifacts..."

artifact_dest="${WORKSPACE}"
if [ -f "${artifact_dest}/test.txt" ]; then
    content=$(cat "${artifact_dest}/test.txt")
    if [ "$content" = "xxxx" ]; then
        echo "PASS: Artifact file test.txt found with correct content"
    else
        echo "FAIL: Artifact file test.txt has incorrect content: $content"
        exit 1
    fi
else
    echo "FAIL: Artifact file test.txt NOT found at ${artifact_dest}/test.txt"
    exit 1
fi

if [ -f "${artifact_dest}/debug.log" ]; then
    content=$(cat "${artifact_dest}/debug.log")
    if [ "$content" = "yyyy" ]; then
        echo "PASS: Artifact file debug.log found with correct content"
    else
        echo "FAIL: Artifact file debug.log has incorrect content: $content"
        exit 1
    fi
else
    echo "FAIL: Artifact file debug.log NOT found at ${artifact_dest}/debug.log"
    exit 1
fi

echo "Test-specific validations passed"

echo ""
echo "[2/3] Verifying workflow CRD..."

workflow_crd=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml)

if echo "$workflow_crd" | grep -q "mountPath: /output/artifact"; then
    echo "PASS: mountPath: /output/artifact found in CRD"
else
    echo "FAIL: mountPath: /output/artifact NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "name: output"; then
    echo "PASS: volumeClaimTemplates name 'output' found in CRD"
else
    echo "FAIL: volumeClaimTemplates name 'output' NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "cp -r --parents"; then
    echo "PASS: cp -r --parents found in script"
else
    echo "FAIL: cp -r --parents NOT found in script"
    exit 1
fi

echo ""
echo "[3/3] Verifying artifact files exist in workspace..."
if [ ! -d "${artifact_dest}" ] || [ -z "$(ls -A "${artifact_dest}" 2>/dev/null)" ]; then
    echo "FAIL: Workspace is empty"
    exit 1
fi
echo "PASS: Artifact files exist"

echo ""
echo "=========================================="
echo "PASS: test13-cp-artifacts-v2 - All validations passed"
echo "=========================================="
exit 0