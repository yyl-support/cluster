#!/bin/bash

kubeconfig_key_file=$(/workspace/workflowtool/kubeconfig)
if [ $? -eq 0 ]; then
    echo "kebuconfig file: $kubeconfig_key_file"
else
    echo "kubeconfig file not found"
fi
echo "读取 kubeconfig.key 文件..."
cat "${kubeconfig_key_file}" | base64 -d > /workspace/workflowtool/k8s-cluster-kubeconfig.yaml