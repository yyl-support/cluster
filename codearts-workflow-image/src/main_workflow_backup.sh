#!/bin/bash

set -euo pipefail

workflow_name=""
namespace="argo"

cd /workspace

workflow_template=${workflow_template:-"./workflowtool/workflow_templatev2.yaml"}
workflow_output=${workflow_output:-"./workflowtool/workflow.yaml"}

if [ -z "${WORKSPACE}" ]; then
    echo "错误：环境变量 WORKSPACE 未设置"
    exit 1
fi

kubeconfig_key_file="${WORKSPACE}/kubeconfig.key"
if [ ! -f "${kubeconfig_key_file}" ]; then
    echo "警告：${kubeconfig_key_file} 不存在，尝试查找 kubeconfig_*.key 文件..."
    kubeconfig_key_file=$(find "${WORKSPACE}" -maxdepth 1 -name "kubeconfig_*.key" -type f | head -n 1)
    if [ -z "${kubeconfig_key_file}" ]; then
        echo "错误：未找到任何 kubeconfig_*.key 文件"
        exit 1
    fi
    echo "找到文件: ${kubeconfig_key_file}"
fi

echo "读取 kubeconfig.key 文件..."
cat "${kubeconfig_key_file}" | base64 -d > /workspace/workflowtool/k8s-cluster-kubeconfig.yaml

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

workflow_uid=$(kubectl get workflow ${workflow_name} -n "$namespace" -o jsonpath='{.metadata.uid}')

artifact_pvc_file="${workflow_output%.yaml}-artifact-pvc.yaml"
if [[ -f "$artifact_pvc_file" ]]; then
    sed "s/\${WORKFLOW_NAME}/${workflow_name}/g; s/\${WORKFLOW_UID}/${workflow_uid}/g" \
        "$artifact_pvc_file" | kubectl apply -f - -n "$namespace"
    echo "PVC applied, waiting for binding..."
    pvc_name=$(sed "s/\${WORKFLOW_NAME}/${workflow_name}/g; s/\${WORKFLOW_UID}/${workflow_uid}/g" \
        "$artifact_pvc_file" | awk '/^metadata:/ { found=1 } found && /^  name:/ { print $2; exit }')
    for i in $(seq 1 60); do
        pvc_status=$(kubectl get pvc/"$pvc_name" --namespace="$namespace" -o jsonpath='{.status.phase}' 2>/dev/null)
        if [[ "$pvc_status" == "Bound" ]]; then
            echo "PVC is Bound"
            break
        fi
        echo "Waiting for PVC... ($i/60)"
        sleep 5
    done
    if [[ "$pvc_status" != "Bound" ]]; then
        echo "PVC binding timeout"
        exit 1
    fi
fi

argo logs ${workflow_name} -n "$namespace" --follow 2>&1 | sed "s/\x1b\[[0-9;]*m//g" &
LOGS_PID=$!

wait $LOGS_PID || true

workflow_status=$(argo get ${workflow_name} -n "$namespace" | grep Status | awk '{print $2}')

while [[ "$workflow_status" == "Running" || "$workflow_status" == "Pending" ]]; do
  workflow_status=$(argo get ${workflow_name} -n "$namespace" | grep Status | awk '{print $2}')
  sleep 10
done

if [[ "${workflow_status}" != "Succeeded" ]]; then
  echo "Workflow ${workflow_name} failed with status: ${workflow_status}"
  exit 1
fi

copy_pod_file="${workflow_output%.yaml}-copy-pod.yaml"
if [[ -n "${CP_artifacts_temp_folder:-}" ]] && [[ -f "$copy_pod_file" ]]; then
    echo "Starting artifact copy..."

    copy_pod_name=$(grep "^  name:" "$copy_pod_file" | awk '{print $2}')
    kubectl apply -f "$copy_pod_file" -n "$namespace"

    kubectl wait pod/"$copy_pod_name" -n "$namespace" --for=condition=Ready --timeout=1800s || {
        echo "Copy pod wait timeout"; exit 1; }

    mkdir -p "${WORKSPACE}"
    kubectl cp "$namespace/$copy_pod_name:${CP_artifacts_temp_folder}/." "${WORKSPACE}/" || {
        echo "Artifact copy failed"; exit 1; }

    kubectl delete pod "$copy_pod_name" -n "$namespace"
    echo "Artifacts copied successfully"
fi
