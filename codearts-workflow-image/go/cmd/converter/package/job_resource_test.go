package converter

import (
	"reflect"
	"testing"

	runonparser "github.com/opensourceways/codearts-workflow-image-go/cmd/common"
	"github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"
)

func requestList(cpu, memory string) volcano.ResourceList {
	return volcano.ResourceList{
		"cpu":    cpu,
		"memory": memory,
	}
}

func requestListWithNPU(cpu, memory, npu string) volcano.ResourceList {
	return volcano.ResourceList{
		"cpu":                    cpu,
		"memory":                 memory,
		"huawei.com/ascend-1980": npu,
	}
}

func requestListWithNPU310(cpu, memory, npu string) volcano.ResourceList {
	return volcano.ResourceList{
		"cpu":                   cpu,
		"memory":                memory,
		"huawei.com/ascend-310": npu,
	}
}

func limitList(memory string) volcano.ResourceList {
	return volcano.ResourceList{
		"memory": memory,
	}
}

func limitListWithCPU(cpu, memory string) volcano.ResourceList {
	return volcano.ResourceList{
		"cpu":    cpu,
		"memory": memory,
	}
}

func limitListWithNPU(memory, npu string) volcano.ResourceList {
	return volcano.ResourceList{
		"memory":                 memory,
		"huawei.com/ascend-1980": npu,
	}
}

func limitListWithNPU310(memory, npu string) volcano.ResourceList {
	return volcano.ResourceList{
		"memory":                memory,
		"huawei.com/ascend-310": npu,
	}
}

func Test_convertJobResource(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		want    volcano.Resources
		wantErr bool
	}{
		{
			name: "arm64 default",
			spec: "arm64",
			want: volcano.Resources{
				Requests: requestList("8", "8Gi"),
				Limits:   limitListWithCPU("8", "8Gi"),
			},
			wantErr: false,
		},
		{
			name: "amd64 default",
			spec: "amd64",
			want: volcano.Resources{
				Requests: requestList("8", "8Gi"),
				Limits:   limitListWithCPU("8", "8Gi"),
			},
			wantErr: false,
		},

		{
			name: "arm64-npu-0",
			spec: "arm64-npu-0",
			want: volcano.Resources{
				Requests: requestList("8", "16Gi"),
				Limits:   limitList("16Gi"),
			},
			wantErr: false,
		},
		{
			name: "arm64-npu-1",
			spec: "arm64-npu-1",
			want: volcano.Resources{
				Requests: requestListWithNPU("12", "48Gi", "1"),
				Limits:   limitListWithNPU("48Gi", "1"),
			},
			wantErr: false,
		},

		{
			name: "arm64-NPU-1",
			spec: "arm64-NPU-1",
			want: volcano.Resources{
				Requests: requestListWithNPU("12", "48Gi", "1"),
				Limits:   limitListWithNPU("48Gi", "1"),
			},
			wantErr: false,
		},
		{
			name: "arm64-npu-2",
			spec: "arm64-npu-2",
			want: volcano.Resources{
				Requests: requestListWithNPU("20", "80Gi", "2"),
				Limits:   limitListWithNPU("80Gi", "2"),
			},
			wantErr: false,
		},
		{
			name:    "amd64-npu-1",
			spec:    "amd64-npu-1",
			wantErr: true,
		},

		{
			name: "amd64-cpu-16",
			spec: "amd64-cpu-16",
			want: volcano.Resources{
				Requests: requestList("16", "8Gi"),
				Limits:   limitListWithCPU("16", "8Gi"),
			},
			wantErr: false,
		},
		{
			name: "amd64-cpu-1 (minimum request)",
			spec: "amd64-cpu-1",
			want: volcano.Resources{
				Requests: requestList("1", "8Gi"),
				Limits:   limitListWithCPU("1", "8Gi"),
			},
			wantErr: false,
		},
		{
			name: "arm64-cpu-1 (minimum request)",
			spec: "arm64-cpu-1",
			want: volcano.Resources{
				Requests: requestList("1", "8Gi"),
				Limits:   limitListWithCPU("1", "8Gi"),
			},
			wantErr: false,
		},
		{
			name: "amd64-cpu-16-mem-16G",
			spec: "amd64-cpu-16-mem-16G",
			want: volcano.Resources{
				Requests: requestList("16", "16Gi"),
				Limits:   limitListWithCPU("16", "16Gi"),
			},
			wantErr: false,
		},

		{
			name: "arm64-cpu-16 ",
			spec: "arm64-cpu-16",
			want: volcano.Resources{
				Requests: requestList("16", "8Gi"),
				Limits:   limitListWithCPU("16", "8Gi"),
			},
			wantErr: false,
		},
		{
			name: "arm64-mem-16G (ignored - not implemented)",
			spec: "arm64-mem-16G",
			want: volcano.Resources{
				Requests: requestList("8", "16Gi"),
				Limits:   limitListWithCPU("8", "16Gi"),
			},
			wantErr: false,
		},
		{
			name: "arm64-cpu-16-mem-16G ",
			spec: "arm64-cpu-16-mem-16G",
			want: volcano.Resources{
				Requests: requestList("16", "16Gi"),
				Limits:   limitListWithCPU("16", "16Gi"),
			},
			wantErr: false,
		},
		{
			name: "arm64-npu-2-cpu-16-mem-16g ",
			spec: "arm64-npu-2-cpu-16-mem-16g",
			want: volcano.Resources{
				Requests: requestListWithNPU("16", "16Gi", "2"),
				Limits:   limitListWithNPU("16Gi", "2"),
			},
			wantErr: false,
		},

		{
			name: "arm64-cpu-16-mem-16g-npu-1",
			spec: "arm64-cpu-16-mem-16g-npu-1",
			want: volcano.Resources{
				Requests: requestListWithNPU("16", "16Gi", "1"),
				Limits:   limitListWithNPU("16Gi", "1"),
			},
			wantErr: false,
		},
		{
			name: "arm64-cpu-16-npu-2-mem-16g",
			spec: "arm64-cpu-16-npu-2-mem-16g",
			want: volcano.Resources{
				Requests: requestListWithNPU("16", "16Gi", "2"),
				Limits:   limitListWithNPU("16Gi", "2"),
			},
			wantErr: false,
		},

		{
			name: "arm64-910B4-1",
			spec: "arm64-910B4-1",
			want: volcano.Resources{
				Requests: requestListWithNPU("12", "48Gi", "1"),
				Limits:   limitListWithNPU("48Gi", "1"),
			},
			wantErr: false,
		},
		{
			name: "arm64-910B4-2",
			spec: "arm64-910B4-2",
			want: volcano.Resources{
				Requests: requestListWithNPU("20", "80Gi", "2"),
				Limits:   limitListWithNPU("80Gi", "2"),
			},
			wantErr: false,
		},
		{
			name:    "amd64-910B4-1",
			spec:    "amd64-910B4-1",
			wantErr: true,
		},
		{
			name: "arm64-910b4-1 (case insensitive)",
			spec: "arm64-910b4-1",
			want: volcano.Resources{
				Requests: requestListWithNPU("12", "48Gi", "1"),
				Limits:   limitListWithNPU("48Gi", "1"),
			},
			wantErr: false,
		},
		{
			name: "arm64-310P3-1 (310 chip)",
			spec: "arm64-310P3-1",
			want: volcano.Resources{
				Requests: requestListWithNPU310("8", "32Gi", "1"),
				Limits:   limitListWithNPU310("32Gi", "1"),
			},
			wantErr: false,
		},
		{
			name: "arm64-310p3-1 (case insensitive 310)",
			spec: "arm64-310p3-1",
			want: volcano.Resources{
				Requests: requestListWithNPU310("8", "32Gi", "1"),
				Limits:   limitListWithNPU310("32Gi", "1"),
			},
			wantErr: false,
		},
		{
			name: "arm64-310P3-2",
			spec: "arm64-310P3-2",
			want: volcano.Resources{
				Requests: requestListWithNPU310("16", "64Gi", "2"),
				Limits:   limitListWithNPU310("64Gi", "2"),
			},
			wantErr: false,
		},
		{
			name: "arm64-cpu-16-310P3-1",
			spec: "arm64-cpu-16-310P3-1",
			want: volcano.Resources{
				Requests: requestListWithNPU310("11", "32Gi", "1"),
				Limits:   limitListWithNPU310("32Gi", "1"),
			},
			wantErr: false,
		},
		{
			name: "arm64-cpu-16-mem-16g-310P3-1",
			spec: "arm64-cpu-16-mem-16g-310P3-1",
			want: volcano.Resources{
				Requests: requestListWithNPU310("11", "16Gi", "1"),
				Limits:   limitListWithNPU310("16Gi", "1"),
			},
			wantErr: false,
		},
		{
			name: "arm64-cpu-16-910B4-1",
			spec: "arm64-cpu-16-910B4-1",
			want: volcano.Resources{
				Requests: requestListWithNPU("16", "48Gi", "1"),
				Limits:   limitListWithNPU("48Gi", "1"),
			},
			wantErr: false,
		},
		{
			name: "arm64-cpu-16-mem-16g-910B4-1",
			spec: "arm64-cpu-16-mem-16g-910B4-1",
			want: volcano.Resources{
				Requests: requestListWithNPU("16", "16Gi", "1"),
				Limits:   limitListWithNPU("16Gi", "1"),
			},
			wantErr: false,
		},
		{
			name: "arm64-unknownChip-1 (invalid chip - no npu resource)",
			spec: "arm64-unknownChip-1",
			want: volcano.Resources{
				Requests: requestList("8", "8Gi"),
				Limits:   limitListWithCPU("8", "8Gi"),
			},
			wantErr: false,
		},

		{
			name: "arm64-cpu-50-910B4-1 (capped to 23 per NPU)",
			spec: "arm64-cpu-50-910B4-1",
			want: volcano.Resources{
				Requests: requestListWithNPU("23", "48Gi", "1"),
				Limits:   limitListWithNPU("48Gi", "1"),
			},
			wantErr: false,
		},
		{
			name: "arm64-cpu-50-910B4-2 (capped to 46 for 2 NPU)",
			spec: "arm64-cpu-50-910B4-2",
			want: volcano.Resources{
				Requests: requestListWithNPU("46", "80Gi", "2"),
				Limits:   limitListWithNPU("80Gi", "2"),
			},
			wantErr: false,
		},
		{
			name: "arm64-cpu-40-310P3-2 (capped to 22 for 2 NPU)",
			spec: "arm64-cpu-40-310P3-2",
			want: volcano.Resources{
				Requests: requestListWithNPU310("22", "64Gi", "2"),
				Limits:   limitListWithNPU310("64Gi", "2"),
			},
			wantErr: false,
		},
		{
			name: "arm64-cpu-40-310P3-1 (capped to 11 for 1 NPU)",
			spec: "arm64-cpu-40-310P3-1",
			want: volcano.Resources{
				Requests: requestListWithNPU310("11", "32Gi", "1"),
				Limits:   limitListWithNPU310("32Gi", "1"),
			},
			wantErr: false,
		},
		{
			name: "arm64-cpu-20-910B4-1 (under cap, no change)",
			spec: "arm64-cpu-20-910B4-1",
			want: volcano.Resources{
				Requests: requestListWithNPU("20", "48Gi", "1"),
				Limits:   limitListWithNPU("48Gi", "1"),
			},
			wantErr: false,
		},
		{
			name: "arm64-cpu-10-310P3-1 (under cap, no change)",
			spec: "arm64-cpu-10-310P3-1",
			want: volcano.Resources{
				Requests: requestListWithNPU310("10", "32Gi", "1"),
				Limits:   limitListWithNPU310("32Gi", "1"),
			},
			wantErr: false,
		},
		{
			name: "arm64-cpu-50-mem-16g-910B4-1 (capped with custom mem)",
			spec: "arm64-cpu-50-mem-16g-910B4-1",
			want: volcano.Resources{
				Requests: requestListWithNPU("23", "16Gi", "1"),
				Limits:   limitListWithNPU("16Gi", "1"),
			},
			wantErr: false,
		},
		{
			name: "arm64-cpu-50-npu-1 (generic NPU capped to 23)",
			spec: "arm64-cpu-50-npu-1",
			want: volcano.Resources{
				Requests: requestListWithNPU("23", "48Gi", "1"),
				Limits:   limitListWithNPU("48Gi", "1"),
			},
			wantErr: false,
		},

		{
			name:    "empty string",
			spec:    "",
			wantErr: true,
		},
		{
			name:    "invalid arch",
			spec:    "x86_64",
			wantErr: true,
		},
		{
			name:    "invalid npu count",
			spec:    "arm64-npu-invalid",
			wantErr: true,
		},
		{
			name:    "npu without count",
			spec:    "arm64-npu",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertJobResource(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertJobResource() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertJobResource() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestGetNPUResourceName(t *testing.T) {
	tests := []struct {
		name    string
		spec    string
		want    string
		wantErr bool
	}{
		{
			name:    "310P3 chip returns ascend-310",
			spec:    "arm64-310P3-1",
			want:    "huawei.com/ascend-310",
			wantErr: false,
		},
		{
			name:    "910B4 chip returns ascend-1980",
			spec:    "arm64-910B4-1",
			want:    "huawei.com/ascend-1980",
			wantErr: false,
		},
		{
			name:    "910A chip returns ascend-1980",
			spec:    "arm64-910A-1",
			want:    "huawei.com/ascend-1980",
			wantErr: false,
		},
		{
			name:    "generic npu returns ascend-1980 default",
			spec:    "arm64-npu-1",
			want:    "huawei.com/ascend-1980",
			wantErr: false,
		},
		{
			name:    "910B1 returns ascend-1980",
			spec:    "arm64-910B1-1",
			want:    "huawei.com/ascend-1980",
			wantErr: false,
		},
		{
			name:    "no NPU returns default ascend-1980",
			spec:    "arm64",
			want:    "huawei.com/ascend-1980",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedSpec, err := runonparser.Parse(tt.spec)
			if (err != nil) != tt.wantErr {
				t.Errorf("Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && parsedSpec != nil {
				got := getNPUResourceName(parsedSpec)
				if got != tt.want {
					t.Errorf("getNPUResourceName() = %v, want %v", got, tt.want)
				}
			}
		})
	}
}
