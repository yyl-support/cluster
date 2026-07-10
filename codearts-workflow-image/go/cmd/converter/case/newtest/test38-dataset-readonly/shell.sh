#!/bin/bash
echo "Dataset read only test"
ls -la /dataset
# Attempting to write should fail when readOnly is true
echo test > /dataset/test_write.txt 2>&1 || echo "Write failed (expected)"
