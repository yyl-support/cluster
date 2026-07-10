#!/bin/bash

echo "Testing /dev/shm volume mount"
df -h | grep /dev/shm || echo "No /dev/shm found"
ls -la /dev/shm 2>/dev/null || echo "Cannot access /dev/shm"
echo "Creating test file in /dev/shm"
echo "test content" > /dev/shm/test_file && echo "SUCCESS: Can write to /dev/shm" || echo "FAIL: Cannot write to /dev/shm"
cat /dev/shm/test_file && echo "SUCCESS: Can read from /dev/shm" || echo "FAIL: Cannot read from /dev/shm"
rm /dev/shm/test_file