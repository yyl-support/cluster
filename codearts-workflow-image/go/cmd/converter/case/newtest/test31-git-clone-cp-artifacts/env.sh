#!/bin/bash
export CP_runs_on="arm64"
export CP_docker_image="swr.cn-southwest-2.myhuaweicloud.com/base_image/ascend-ci/cann:8.2.rc1-910b-ubuntu22.04-py3.11"
export CP_pipeline_run_id="test-git-clone-cp-123"
export CP_repo_url="https://gitcode.com/Ascend/AscendNPU-IR.git"
export CP_artifacts="*.txt;*.log"
export WORKSPACE="/home/chenqi252/code/agentpk/opencode/codearts-workflow-image/go/cmd/converter/case/newtest/test31-git-clone-cp-artifacts/artifacts"
export JOB_ID="job-git-cp"
export BUILDNUMBER="313233"
