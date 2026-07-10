#!/bin/bash
# Eval script for test30-cp-pull-failure
# Tests: Image pull failure with copy-artifact container
# Expected behavior: Pod DELETED (imagePullError), NO artifact extraction

WORKFLOW_NAME="$1"
NAMESPACE="$2"
KUBECONFIG="$3"
WORKSPACE="${4:-.}"
CP_ARTIFACTS_TEMP_FOLDER="${5:-}"
WORKFLOW_EXIT="${6:-0}"

echo "=========================================="
echo "EVAL: test30-cp-pull-failure"
echo "=========================================="

# Expected: submit exits with code 1 (image pull failure)
if [[ "$WORKFLOW_EXIT" -eq 1 ]]; then
    echo "PASS: Submit exited with code 1 (expected for image pull failure)"
else
    echo "FAIL: Expected submit exit code 1, got ${WORKFLOW_EXIT}"
    exit 1
fi

echo ""
echo "[1/4] Verifying pod was DELETED..."

# Wait for pod deletion (async delete takes time)
pod_name=""
max_wait=30
elapsed=0

while [ $elapsed -lt $max_wait ]; do
    pod_name=$(kubectl get pods -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -l volcano.sh/job-name=${WORKFLOW_NAME} -o jsonpath='{.items[0].metadata.name}' 2>&1)
    
    # Pod deleted if: empty result, NotFound error, OR array index error (empty list)
    if [[ -z "$pod_name" ]] || \
       echo "$pod_name" | grep -q "NotFound" || \
       echo "$pod_name" | grep -q "array index out of bounds" || \
       echo "$pod_name" | grep -q "length 0"; then
        echo "PASS: Pod was deleted (expected behavior for imagePullError)"
        break
    fi
    
    echo "  Pod still exists: ${pod_name} (${elapsed}s elapsed)"
    sleep 3
    ((elapsed += 3))
done

# Pod deleted if any deletion indicator found
if [[ -n "$pod_name" ]] && \
   ! echo "$pod_name" | grep -q "NotFound" && \
   ! echo "$pod_name" | grep -q "array index out of bounds" && \
   ! echo "$pod_name" | grep -q "length 0"; then
    echo "FAIL: Pod ${pod_name} still exists after ${max_wait}s (should be deleted on image pull failure)"
    exit 1
fi

echo ""
echo "[2/4] Checking workflow CRD exists..."

workflow_yaml=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml 2>&1)

if echo "$workflow_yaml" | grep -q "NotFound"; then
    echo "[INFO] Job was deleted (image pull failure detected and handled)"
    echo "PASS: Image pull failure handled - job was stopped and deleted"
    echo ""
    echo "=========================================="
    echo "PASS: test30-cp-pull-failure - All validations passed"
    echo "=========================================="
    exit 0
fi

echo "PASS: Workflow CRD exists (submitted before deletion)"

echo ""
echo "[3/4] Verifying invalid image in CRD..."

if echo "$workflow_yaml" | grep -q "swr.cn-southwest-2.myhuaweicloud.com/nonexistent/invalid-image:does-not-exist"; then
    echo "PASS: Non-existent image found in main container CRD"
else
    echo "FAIL: Expected non-existent image NOT found in CRD"
    exit 1
fi

echo ""
echo "[4/4] Verifying NO artifact extraction..."

artifact_dest="${WORKSPACE}"

if [ -f "${artifact_dest}/test.txt" ]; then
    echo "FAIL: Artifact file test.txt found at ${artifact_dest}/test.txt (should NOT be extracted on image pull failure)"
    exit 1
else
    echo "PASS: Artifact file test.txt NOT found (expected: NO extraction on image pull failure)"
fi

if [ -f "${artifact_dest}/debug.log" ]; then
    echo "FAIL: Artifact file debug.log found at ${artifact_dest}/debug.log (should NOT be extracted on image pull failure)"
    exit 1
else
    echo "PASS: Artifact file debug.log NOT found (expected: NO extraction on image pull failure)"
fi

echo ""
echo "=========================================="
echo "PASS: test30-cp-pull-failure - All validations passed"
echo "=========================================="
exit 0