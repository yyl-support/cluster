#!/bin/sh
echo "Starting large repo download from gitcode.com..."
time git clone --progress https://gitcode.com/Ascend/pytorch.git
echo "Download complete"
du -sh pytorch/
ls -lh pytorch/ | head -20
