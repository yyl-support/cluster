#!/bin/bash
echo "Building artifacts..."
mkdir -p /output/artifact
echo "artifact content 1" > /output/artifact/test.txt
echo "artifact content 2" > /output/artifact/test2.txt
ls -la /output/artifact/