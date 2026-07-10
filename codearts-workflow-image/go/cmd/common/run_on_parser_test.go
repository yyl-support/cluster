package run_on_parser

import (
	"testing"
)

func TestParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *RunOnSpec
		wantErr bool
	}{
		{
			name:  "simple amd64",
			input: "amd64",
			want: &RunOnSpec{
				Arch: "amd64",
			},
			wantErr: false,
		},
		{
			name:  "simple arm64",
			input: "arm64",
			want: &RunOnSpec{
				Arch: "arm64",
			},
			wantErr: false,
		},
		{
			name:  "amd64 with custom cpu and memory",
			input: "amd64-cpu-16-mem-32G",
			want: &RunOnSpec{
				Arch:           "amd64",
				CPUCoreCount:   16,
				MemorySize:     32,
				MemorySizeUnit: "Gi",
				NPUCount:       0,
			},
			wantErr: false,
		},
		{
			name:  "amd64 with cpu, memory, and npu",
			input: "amd64-cpu-16-mem-32G-npu-1",
			want: &RunOnSpec{
				Arch:           "amd64",
				CPUCoreCount:   16,
				MemorySize:     32,
				MemorySizeUnit: "Gi",
				NPUCount:       1,
				NPUChipName:    "npu",
			},
			wantErr: false,
		},
		{
			name:  "amd64 with cpu, npu, and memory",
			input: "amd64-cpu-16-npu-1-mem-32G",
			want: &RunOnSpec{
				Arch:           "amd64",
				CPUCoreCount:   16,
				MemorySize:     32,
				MemorySizeUnit: "Gi",
				NPUCount:       1,
				NPUChipName:    "npu",
			},
			wantErr: false,
		},
		{
			name:  "arm64 with NPU 910b4",
			input: "arm64-910b4-2",
			want: &RunOnSpec{
				Arch:        "arm64",
				NPUChipName: "910B4",
				NPUCount:    2,
			},
			wantErr: false,
		},
		{
			name:  "arm64 with NPU 910B4 (uppercase)",
			input: "arm64-910B4-2",
			want: &RunOnSpec{
				Arch:        "arm64",
				NPUChipName: "910B4",
				NPUCount:    2,
			},
			wantErr: false,
		},
		{
			name:  "arm64 with npu (lowercase)",
			input: "arm64-npu-2",
			want: &RunOnSpec{
				Arch:        "arm64",
				NPUChipName: "npu",
				NPUCount:    2,
			},
			wantErr: false,
		},
		{
			name:  "arm64 with NPU (uppercase)",
			input: "arm64-NPU-2",
			want: &RunOnSpec{
				Arch:        "arm64",
				NPUChipName: "npu",
				NPUCount:    2,
			},
			wantErr: false,
		},
		{
			name:  "arm64 with NPU 910b4 and custom memory",
			input: "arm64-910b4-4-mem-128G",
			want: &RunOnSpec{
				Arch:           "arm64",
				MemorySize:     128,
				MemorySizeUnit: "Gi",
				NPUChipName:    "910B4",
				NPUCount:       4,
			},
			wantErr: false,
		},
		{
			name:  "arm64 with NPU 910b4 and custom memory (lowercase g)",
			input: "arm64-910b4-4-mem-128g",
			want: &RunOnSpec{
				Arch:           "arm64",
				MemorySize:     128,
				MemorySizeUnit: "Gi",
				NPUChipName:    "910B4",
				NPUCount:       4,
			},
			wantErr: false,
		},
		{
			name:  "arm64 with NPU 910a",
			input: "arm64-910a-1",
			want: &RunOnSpec{
				Arch:        "arm64",
				NPUChipName: "910A",
				NPUCount:    1,
			},
			wantErr: false,
		},
		{
			name:  "arm64 with NPU 910b1",
			input: "arm64-910b1-1",
			want: &RunOnSpec{
				Arch:        "arm64",
				NPUChipName: "910B1",
				NPUCount:    1,
			},
			wantErr: false,
		},
		{
			name:  "arm64 with NPU 910b2",
			input: "arm64-910b2-2",
			want: &RunOnSpec{
				Arch:        "arm64",
				NPUChipName: "910B2",
				NPUCount:    2,
			},
			wantErr: false,
		},
		{
			name:  "arm64 with NPU 910b3",
			input: "arm64-910b3-3",
			want: &RunOnSpec{
				Arch:        "arm64",
				NPUChipName: "910B3",
				NPUCount:    3,
			},
			wantErr: false,
		},
		{
			name:  "arm64 with NPU 310p3",
			input: "arm64-310p3-1",
			want: &RunOnSpec{
				Arch:        "arm64",
				NPUChipName: "310P3",
				NPUCount:    1,
			},
			wantErr: false,
		},
		{
			name:  "memory with Gi suffix",
			input: "amd64-mem-16Gi",
			want: &RunOnSpec{
				Arch:           "amd64",
				MemorySize:     16,
				MemorySizeUnit: "Gi",
			},
			wantErr: false,
		},
		{
			name:  "memory with g suffix (lowercase)",
			input: "amd64-mem-16g",
			want: &RunOnSpec{
				Arch:           "amd64",
				MemorySize:     16,
				MemorySizeUnit: "Gi",
			},
			wantErr: false,
		},
		{
			name:  "amd64 with NPU 910b4",
			input: "amd64-910b4-2",
			want: &RunOnSpec{
				Arch:        "amd64",
				NPUChipName: "910B4",
				NPUCount:    2,
			},
			wantErr: false,
		},
		{
			name:    "empty input",
			input:   "",
			wantErr: true,
		},
		{
			name:    "invalid arch",
			input:   "x86",
			wantErr: true,
		},
		{
			name:    "invalid cpu value",
			input:   "amd64-cpu-abc",
			wantErr: true,
		},
		{
			name:    "invalid memory value",
			input:   "amd64-mem-abcG",
			wantErr: true,
		},
		{
			name:    "invalid memory value (missing value)",
			input:   "amd64-mem",
			wantErr: true,
		},
		{
			name:  "memory with M suffix",
			input: "amd64-mem-16M",
			want: &RunOnSpec{
				Arch:           "amd64",
				MemorySize:     16,
				MemorySizeUnit: "Mi",
			},
			wantErr: false,
		},
		{
			name:  "memory with Mi suffix",
			input: "amd64-mem-16Mi",
			want: &RunOnSpec{
				Arch:           "amd64",
				MemorySize:     16,
				MemorySizeUnit: "Mi",
			},
			wantErr: false,
		},
		{
			name:    "memory with lowercase m (invalid for memory)",
			input:   "amd64-mem-16m",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != nil {
				if got.Arch != tt.want.Arch {
					t.Errorf("Parse() Arch = %v, want %v", got.Arch, tt.want.Arch)
				}
				if got.CPUCoreCount != tt.want.CPUCoreCount {
					t.Errorf("Parse() CPUCoreCount = %v, want %v", got.CPUCoreCount, tt.want.CPUCoreCount)
				}
				if got.MemorySize != tt.want.MemorySize {
					t.Errorf("Parse() MemorySize = %v, want %v", got.MemorySize, tt.want.MemorySize)
				}
				if got.MemorySizeUnit != tt.want.MemorySizeUnit {
					t.Errorf("Parse() MemorySizeUnit = %v, want %v", got.MemorySizeUnit, tt.want.MemorySizeUnit)
				}
				if got.NPUChipName != tt.want.NPUChipName {
					t.Errorf("Parse() NPUChipName = %v, want %v", got.NPUChipName, tt.want.NPUChipName)
				}
				if got.NPUCount != tt.want.NPUCount {
					t.Errorf("Parse() NPUCount = %v, want %v", got.NPUCount, tt.want.NPUCount)
				}
			}
		})
	}
}

func TestIsArm64(t *testing.T) {
	tests := []struct {
		name string
		spec *RunOnSpec
		want bool
	}{
		{
			name: "arm64 arch",
			spec: &RunOnSpec{
				Arch: "arm64",
			},
			want: true,
		},
		{
			name: "amd64 arch",
			spec: &RunOnSpec{
				Arch: "amd64",
			},
			want: false,
		},
		{
			name: "empty arch",
			spec: &RunOnSpec{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.spec.IsArm64()
			if got != tt.want {
				t.Errorf("IsArm64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsAmd64(t *testing.T) {
	tests := []struct {
		name string
		spec *RunOnSpec
		want bool
	}{
		{
			name: "amd64 arch",
			spec: &RunOnSpec{
				Arch: "amd64",
			},
			want: true,
		},
		{
			name: "arm64 arch",
			spec: &RunOnSpec{
				Arch: "arm64",
			},
			want: false,
		},
		{
			name: "empty arch",
			spec: &RunOnSpec{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.spec.IsAmd64()
			if got != tt.want {
				t.Errorf("IsAmd64() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsMEMEmpty(t *testing.T) {
	tests := []struct {
		name string
		spec *RunOnSpec
		want bool
	}{
		{
			name: "empty mem",
			spec: &RunOnSpec{},
			want: true,
		},
		{
			name: "with memory size",
			spec: &RunOnSpec{
				MemorySize: 8,
			},
			want: false,
		},
		{
			name: "with memory unit",
			spec: &RunOnSpec{
				MemorySizeUnit: "Gi",
			},
			want: false,
		},
		{
			name: "with both",
			spec: &RunOnSpec{
				MemorySize:     8,
				MemorySizeUnit: "Gi",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.spec.IsMEMEmpty()
			if got != tt.want {
				t.Errorf("IsMEMEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsGenericNPU(t *testing.T) {
	tests := []struct {
		name string
		spec *RunOnSpec
		want bool
	}{
		{
			name: "generic_npu",
			spec: &RunOnSpec{
				NPUChipName: "npu",
				NPUCount:    1,
			},
			want: true,
		},
		{
			name: "specific_chip_910b4",
			spec: &RunOnSpec{
				NPUChipName: "910B4",
				NPUCount:    1,
			},
			want: false,
		},
		{
			name: "specific_chip_310p3",
			spec: &RunOnSpec{
				NPUChipName: "310P3",
				NPUCount:    1,
			},
			want: false,
		},
		{
			name: "no_npu",
			spec: &RunOnSpec{},
			want: false,
		},
		{
			name: "empty_chip_name",
			spec: &RunOnSpec{
				NPUCount: 1,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.spec.IsGenericNPU()
			if got != tt.want {
				t.Errorf("IsGenericNPU() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIs310P3(t *testing.T) {
	tests := []struct {
		name string
		spec *RunOnSpec
		want bool
	}{
		{
			name: "310P3 chip",
			spec: &RunOnSpec{
				NPUChipName: "310P3",
				NPUCount:    1,
			},
			want: true,
		},
		{
			name: "910B4 chip",
			spec: &RunOnSpec{
				NPUChipName: "910B4",
				NPUCount:    1,
			},
			want: false,
		},
		{
			name: "generic npu",
			spec: &RunOnSpec{
				NPUChipName: "npu",
				NPUCount:    1,
			},
			want: false,
		},
		{
			name: "empty chip name",
			spec: &RunOnSpec{
				NPUCount: 1,
			},
			want: false,
		},
		{
			name: "910A chip",
			spec: &RunOnSpec{
				NPUChipName: "910A",
				NPUCount:    1,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.spec.Is310P3()
			if got != tt.want {
				t.Errorf("Is310P3() = %v, want %v", got, tt.want)
			}
		})
	}
}
