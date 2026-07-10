#!/bin/bash

namespace="argo"

cd /workspace

workflow_template=${workflow_template:-"./workflowtool/workflow_templatev2.yaml"}
workflow_output=${workflow_output:-"./workflowtool/workflow.yaml"}

if [ -z "${WORKSPACE}" ]; then
    echo "错误：环境变量 WORKSPACE 未设置"
    exit 1
fi

./workflowtool/convert_to_yaml -o "${workflow_output}" -t "${workflow_template}"

secret_file="${workflow_output%.yaml}-secret.yaml"
if [ -f "$secret_file" ]; then
    echo "Applying secret: $secret_file"
    kubectl apply -f "$secret_file" -n "$namespace"
fi

workflow_name=$(argo submit "${workflow_output}" -n "$namespace" -o name) || {
  echo "ERROR: Workflow submission failed"
  exit 1
}
workflow_name=$(basename "$workflow_name")
echo "Workflow submitted: $workflow_name"

echo "Waiting for main-script pod to complete..."
main_pod=""
while true; do
    main_pod=$(kubectl get pods -n "$namespace" \
        -l "workflows.argoproj.io/workflow=${workflow_name}" \
        --sort-by='.metadata.creationTimestamp' \
        -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)

    if [[ -n "$main_pod" ]] && [[ "$main_pod" == *"main-script"* || "$main_pod" == "$workflow_name" ]]; then
        break
    fi
    echo "  Waiting for main-script pod to appear..."
    sleep 2
done

argo logs "${workflow_name}" -n "$namespace" --follow --no-color 2>&1 &
LOGS_PID=$!


phase=$(kubectl get pod "${main_pod}" -n "$namespace"  -o jsonpath='{.status.phase}' 2>/dev/null)

while [[ "$phase" == "Pending" || "$phase" == "Running" ]]; do
    phase=$(kubectl get pod "${main_pod}" -n "$namespace"  -o jsonpath='{.status.phase}' 2>/dev/null)
    sleep 10
done


if [[ "$phase" != "Succeeded" ]]; then
  echo "Workflow ${workflow_name} failed with status: ${phase}"
  exit 1
fi

echo "main-script pod completed"

if [[ -n "${CP_artifacts:-}" && -z "${CP_artifacts_temp_folder:-}" ]]; then
    CP_artifacts_temp_folder="/output"
fi

if [[ -n "${CP_artifacts_temp_folder:-}" ]]; then
    echo "Artifact workflow detected, using copy-script approach..."

    copy_pod_name=""
    while true; do
        copy_pod_name=$(kubectl get pods -n "$namespace" \
            -l "workflows.argoproj.io/workflow=${workflow_name}" \
            --sort-by='.metadata.creationTimestamp' \
            -o jsonpath='{.items[-1].metadata.name}' 2>/dev/null)

        if [[ -n "$copy_pod_name" ]] && [[ "$copy_pod_name" == *"copy-script"* ]]; then
            break
        fi
        sleep 2
    done
    echo "Copy pod: $copy_pod_name"

    kubectl wait --for=condition=Ready "pod/${copy_pod_name}" -n "$namespace" --timeout=20m 2>&1 || {
        echo "ERROR: copy-script pod did not become ready"
        exit 1
    }
    
    echo "Copying artifacts from copy pod..."
    mkdir -p "${WORKSPACE}"
    kubectl cp "$namespace/$copy_pod_name:${CP_artifacts_temp_folder}/." "${WORKSPACE}/" 2>&1

    echo "Killing copy pod..."
    kubectl exec "$copy_pod_name" -n "$namespace" -- sh -c "kill -TERM 1; exit" 2>/dev/null || true
fi
