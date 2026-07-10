#!/bin/bash
echo "Running on 910B4 NPU"
npu-smi info || echo "npu-smi not available"
