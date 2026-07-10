#!/bin/bash
echo "Testing generic NPU with custom CPU and memory"
echo "Running on arm64 with 2 CPUs, 1G memory, 1 NPU"
npu-smi info || echo "npu-smi not available"
echo "Listing /usr/local/Ascend/driver:"
ls -la /usr/local/Ascend/driver || echo "Directory not found or empty"