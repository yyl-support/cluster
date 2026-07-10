#!/bin/bash

set -euo pipefail

source /workspace/workflowtool/select_kubeconfig.sh

namespace="argo"
work_dir="/workspace"
workflow_template="${workflow_template:-./workflowtool/workflow_templatev2.yaml}"
workflow_output="${workflow_output:-./workflowtool/workflow.yaml}"
cp_artifacts="${CP_artifacts:-}"
cp_artifacts_temp_folder="${CP_artifacts_temp_folder:-}"
cp_workspace="${WORKSPACE}"
kubeconfig_path="/workspace/workflowtool/k8s-cluster-kubeconfig.yaml"

export CP_bandwidth="150M"

cd "$work_dir"

/workspace/workflowtool/convert_to_yaml -o "$workflow_output" -t "$workflow_template"

secret_file="${workflow_output%.yaml}-secret.yaml"

submit_args=(
  --namespace "$namespace"
  --work-dir "$work_dir"
  --workflow-output "$workflow_output"
  --kubeconfig-path "$kubeconfig_path"
  --cp-workspace "$cp_workspace"
)

if [[ -f "$secret_file" ]]; then
  submit_args+=(--secret-file "$secret_file")
fi

if [[ -n "$cp_artifacts" ]]; then
  submit_args+=(--cp-artifacts "$cp_artifacts")
fi

if [[ -n "$cp_artifacts_temp_folder" ]]; then
  submit_args+=(--cp-artifacts-temp-folder "$cp_artifacts_temp_folder")
fi

if [[ -n "${workflow_name_file:-}" ]]; then
  submit_args+=(--workflow-name-file "$workflow_name_file")
fi

exec /workspace/workflowtool/submit "${submit_args[@]}"  2>&1
