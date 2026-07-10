package converter

import (
	"fmt"
	"strconv"

	runonparser "github.com/opensourceways/codearts-workflow-image-go/cmd/common"
	"github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"
)

const (
	ResourceMemory = "memory"
	ResourceCPU    = "cpu"

	MaxCPUPer1980NPU = 23
	MaxCPUPer310P3   = 11
)

var arm310P3ResourceMap = map[int]struct {
	cpu   int
	memGi int
}{
	0: {8, 16},
	1: {8, 32},
	2: {16, 64},
	3: {24, 96},
	4: {32, 128},
	5: {40, 160},
	6: {48, 192},
	7: {56, 224},
	8: {64, 256},
}

var arm1980ResourceMap = map[int]struct {
	cpu   int
	memGi int
}{
	0: {8, 16},
	1: {12, 48},
	2: {20, 80},
	3: {28, 112},
	4: {36, 144},
	5: {44, 176},
	6: {52, 208},
	7: {60, 240},
	8: {64, 512},
}

func getNPUResourceName(parsedSpec *runonparser.RunOnSpec) string {
	if parsedSpec.Is310P3() {
		return "huawei.com/ascend-310"
	}
	return "huawei.com/ascend-1980"
}

func convertJobResource(spec string) (volcano.Resources, error) {
	parsedSpec, err := runonparser.Parse(spec)
	if err != nil {
		return volcano.Resources{}, err
	}

	resources := volcano.Resources{
		Requests: volcano.ResourceList{
			ResourceCPU:    "8",
			ResourceMemory: "8Gi",
		},
		Limits: volcano.ResourceList{
			ResourceCPU:    "8",
			ResourceMemory: "8Gi",
		},
	}

	if !parsedSpec.IsNPUEmpty() {
		if parsedSpec.Arch == "amd64" {
			return volcano.Resources{}, fmt.Errorf("npu scaling is not supported for amd64 architecture")
		}
		if parsedSpec.NPUCount > 8 {
			return volcano.Resources{}, fmt.Errorf("npu count %d exceeds maximum supported value of 8", parsedSpec.NPUCount)
		}

		var res struct {
			cpu   int
			memGi int
		}
		var ok bool

		if parsedSpec.Is310P3() {
			res, ok = arm310P3ResourceMap[parsedSpec.NPUCount]
		} else {
			res, ok = arm1980ResourceMap[parsedSpec.NPUCount]
		}

		if !ok {
			return volcano.Resources{}, fmt.Errorf("internal error: no resource mapping for npu count %d", parsedSpec.NPUCount)
		}

		resources.Requests[ResourceCPU] = strconv.Itoa(res.cpu)
		resources.Requests[ResourceMemory] = strconv.Itoa(res.memGi) + "Gi"
		resources.Limits[ResourceMemory] = strconv.Itoa(res.memGi) + "Gi"

		npuResource := getNPUResourceName(parsedSpec)
		if parsedSpec.NPUCount > 0 {
			resources.Requests[npuResource] = strconv.Itoa(parsedSpec.NPUCount)
			resources.Limits[npuResource] = strconv.Itoa(parsedSpec.NPUCount)
		}

		delete(resources.Limits, ResourceCPU)
	}

	if !parsedSpec.IsCPUEmpty() {
		cpuRequest := parsedSpec.CPUCoreCount
		if cpuRequest < 1 {
			cpuRequest = 1
		}
		resources.Requests[ResourceCPU] = strconv.Itoa(cpuRequest)
		if parsedSpec.Arch == "amd64" || parsedSpec.IsNPUEmpty() {
			resources.Limits[ResourceCPU] = strconv.Itoa(cpuRequest)
		}
	}

	if !parsedSpec.IsMEMEmpty() {
		memValue := parsedSpec.GetMem()
		resources.Requests[ResourceMemory] = memValue
		resources.Limits[ResourceMemory] = memValue
	}

	if !parsedSpec.IsNPUEmpty() && parsedSpec.NPUCount > 0 {
		maxCPU := parsedSpec.NPUCount * MaxCPUPer1980NPU
		if parsedSpec.Is310P3() {
			maxCPU = parsedSpec.NPUCount * MaxCPUPer310P3
		}
		cpuRequest, err := strconv.Atoi(resources.Requests[ResourceCPU])
		if err == nil && cpuRequest > maxCPU {
			resources.Requests[ResourceCPU] = strconv.Itoa(maxCPU)
		}
	}

	return resources, nil
}
