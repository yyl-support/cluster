package run_on_parser

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var KnownChipNames = map[string]bool{
	"910a":  true,
	"910b1": true,
	"910b2": true,
	"910b3": true,
	"910b4": true,
	"310p3": true,
}

type RunOnSpec struct {
	Arch           string
	CPUCoreCount   int
	MemorySize     int
	MemorySizeUnit string
	NPUChipName    string
	NPUCount       int
}

func (r *RunOnSpec) GetMem() string {
	return fmt.Sprintf("%d%s", r.MemorySize, r.MemorySizeUnit)
}

func (r *RunOnSpec) IsEmpty() bool {
	return r.Arch == "" && r.CPUCoreCount == 0 && r.MemorySize == 0 &&
		r.MemorySizeUnit == "" && r.NPUChipName == "" && r.NPUCount == 0
}

func (r *RunOnSpec) IsCPUEmpty() bool {
	return r.CPUCoreCount == 0
}

func (r *RunOnSpec) IsNPUEmpty() bool {
	return r.NPUChipName == "" && r.NPUCount == 0
}

func (r *RunOnSpec) IsGenericNPU() bool {
	return r.NPUChipName == "npu"
}

func (r *RunOnSpec) Is310P3() bool {
	return r.NPUChipName == "310P3"
}

func (r *RunOnSpec) IsMEMEmpty() bool {
	return r.MemorySize == 0 && r.MemorySizeUnit == ""
}

func (r *RunOnSpec) IsArm64() bool {
	return r.Arch == "arm64"
}

func (r *RunOnSpec) IsAmd64() bool {
	return r.Arch == "amd64"
}

func Parse(runOn string) (*RunOnSpec, error) {
	if runOn == "" {
		return nil, errors.New("run_on spec cannot be empty")
	}

	tokens := strings.Split(runOn, "-")
	if len(tokens) == 0 {
		return nil, errors.New("invalid run_on spec format")
	}

	spec := &RunOnSpec{}

	spec.Arch = tokens[0]
	if spec.Arch != "amd64" && spec.Arch != "arm64" {
		return nil, fmt.Errorf("unsupported architecture: %s (expected amd64 or arm64)", spec.Arch)
	}

	for i := 1; i < len(tokens); i += 2 {
		key := tokens[i]
		if i+1 >= len(tokens) {
			return nil, fmt.Errorf("incomplete key-value pair for key: %s", key)
		}
		value := tokens[i+1]

		if KnownChipNames[strings.ToLower(key)] {
			if c, err := strconv.Atoi(value); err == nil && c > 0 {
				spec.NPUCount = c
				spec.NPUChipName = strings.ToUpper(key)
			}
			continue
		}

		switch strings.ToLower(key) {
		case "cpu":
			cores, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("invalid cpu value: %s", value)
			}
			if cores <= 0 {
				return nil, fmt.Errorf("cpu must be positive: %d", cores)
			}
			spec.CPUCoreCount = cores
			continue
		case "mem":
			memSize, unit, err := parseMemory(value)
			if err != nil {
				return nil, err
			}
			spec.MemorySize = memSize
			spec.MemorySizeUnit = unit
			continue
		case "npu":
			c, err := strconv.Atoi(value)
			if err != nil || c < 0 {
				return nil, fmt.Errorf("invalid npu count: %s", value)
			}
			spec.NPUCount = c
			spec.NPUChipName = "npu"
		}
	}

	return spec, nil
}

func parseMemory(value string) (int, string, error) {
	originalValue := value

	if strings.HasSuffix(value, "Mi") {
		sizeStr := strings.TrimSuffix(value, "Mi")
		size, err := strconv.Atoi(sizeStr)
		if err != nil {
			return 0, "", fmt.Errorf("invalid memory value: %s", originalValue)
		}
		return size, "Mi", nil
	}

	if strings.HasSuffix(value, "M") {
		sizeStr := strings.TrimSuffix(value, "M")
		size, err := strconv.Atoi(sizeStr)
		if err != nil {
			return 0, "", fmt.Errorf("invalid memory value: %s", originalValue)
		}
		return size, "Mi", nil
	}

	if strings.HasSuffix(value, "Gi") {
		sizeStr := strings.TrimSuffix(value, "Gi")
		size, err := strconv.Atoi(sizeStr)
		if err != nil {
			return 0, "", fmt.Errorf("invalid memory value: %s", originalValue)
		}
		return size, "Gi", nil
	}

	if strings.HasSuffix(value, "G") {
		sizeStr := strings.TrimSuffix(value, "G")
		size, err := strconv.Atoi(sizeStr)
		if err != nil {
			return 0, "", fmt.Errorf("invalid memory value: %s", originalValue)
		}
		return size, "Gi", nil
	}

	if strings.HasSuffix(value, "g") {
		sizeStr := strings.TrimSuffix(value, "g")
		size, err := strconv.Atoi(sizeStr)
		if err != nil {
			return 0, "", fmt.Errorf("invalid memory value: %s", originalValue)
		}
		return size, "Gi", nil
	}

	return 0, "", fmt.Errorf("memory must use M/Mi or G/Gi units (e.g., 16M, 16Mi, 16G, or 16Gi): %s", originalValue)
}
