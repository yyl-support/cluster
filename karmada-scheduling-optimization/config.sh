#!/usr/bin/env bash

export TZ="Asia/Shanghai"

BASE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RUNTIME_DIR="${RUNTIME_DIR:-${BASE_DIR}/runtime}"
KUBECTL_BIN="${KUBECTL_BIN:-kubectl}"

: "${KARMADA_KUBECONFIG:?Set KARMADA_KUBECONFIG to the Karmada kubeconfig path}"
: "${CLUSTER_001_KUBECONFIG:?Set CLUSTER_001_KUBECONFIG to the 001 kubeconfig path}"
: "${CLUSTER_WLCB_KUBECONFIG:?Set CLUSTER_WLCB_KUBECONFIG to the wlcb kubeconfig path}"

TEST_QUEUE="${TEST_QUEUE:-shared-flexible-queue}"
TEST_IMAGE="${TEST_IMAGE:-busybox:1.36}"

# Beijing time: 00:00 through 05:59.
NIGHT_START_HOUR="${NIGHT_START_HOUR:-0}"
NIGHT_END_HOUR="${NIGHT_END_HOUR:-6}"
STABLE_SECONDS="${STABLE_SECONDS:-1800}"
POLL_SECONDS="${POLL_SECONDS:-60}"
REQUIRE_ZERO_PENDING="${REQUIRE_ZERO_PENDING:-1}"
STAGE_STABLE_SECONDS="${STAGE_STABLE_SECONDS:-300}"

NS_POLICIES=(
  argo-namespace-propagation
  pytorch-namespace-propagation
  indexsdk-namespace-propagation
  recsdk-namespace-propagation
  ragsdk-namespace-propagation
  op-plugin-namespace-propagation
  fbgemm-ascend-namespace-propagation
  multimodalsdk-namespace-propagation
  mindie-llm-namespace-propagation
  mindie-motor-namespace-propagation
  mindie-sd-namespace-propagation
)
