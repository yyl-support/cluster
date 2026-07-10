#!/bin/bash
echo "Testing Ascend driver mount"
npu-smi info || echo "npu-smi not available"
echo "Listing /usr/local/Ascend/driver:"
ls -la /usr/local/Ascend/driver || echo "Directory not found or empty"