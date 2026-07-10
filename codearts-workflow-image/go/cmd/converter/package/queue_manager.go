package converter

import (
	runonparser "github.com/opensourceways/codearts-workflow-image-go/cmd/common"
)

const (
	QueueLargeTaskShared      = "large-task-shared-queue"
	QueueSharedFlexible       = "shared-flexible-queue"
	CPUThresholdForLargeQueue = 64
)

func DetermineQueue(runsOn string) string {
	parsedSpec, err := runonparser.Parse(runsOn)
	if err != nil {
		return QueueSharedFlexible
	}

	cpuCount := getCPUCount(parsedSpec)
	if cpuCount >= CPUThresholdForLargeQueue {
		return QueueLargeTaskShared
	}
	return QueueSharedFlexible
}

func getCPUCount(spec *runonparser.RunOnSpec) int {
	if !spec.IsCPUEmpty() {
		return spec.CPUCoreCount
	}

	if !spec.IsNPUEmpty() && spec.IsArm64() {
		if res, ok := arm1980ResourceMap[spec.NPUCount]; ok {
			return res.cpu
		}
	}

	return 8
}
