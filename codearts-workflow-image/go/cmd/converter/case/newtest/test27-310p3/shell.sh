#!/bin/bash

echo "Running on 310P3 NPU"
echo "Testing specific NPU chip type"
npu-smi info || echo "npu-smi not available"