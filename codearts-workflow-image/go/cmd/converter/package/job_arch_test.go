package converter

import (
	"reflect"
	"testing"
)

func Test_convertJobArch(t *testing.T) {
	tests := []struct {
		name string
		spec string
		want map[string]string
	}{
		{
			name: "arm64 default (no chip)",
			spec: "arm64",
			want: map[string]string{
				"kubernetes.io/arch": "arm64",
			},
		},
		{
			name: "amd64 default (no chip)",
			spec: "amd64",
			want: map[string]string{
				"kubernetes.io/arch": "amd64",
			},
		},
		{
			name: "arm64-npu-1 (npu pattern - no chip selector)",
			spec: "arm64-npu-1",
			want: map[string]string{
				"kubernetes.io/arch": "arm64",
			},
		},
		{
			name: "arm64-910B4-1 (chip pattern)",
			spec: "arm64-910B4-1",
			want: map[string]string{
				"kubernetes.io/arch":               "arm64",
				"node.kubernetes.io/npu.chip.name": "910B4",
			},
		},
		{
			name: "arm64-910b4-1 (chip pattern, case insensitive)",
			spec: "arm64-910b4-1",
			want: map[string]string{
				"kubernetes.io/arch":               "arm64",
				"node.kubernetes.io/npu.chip.name": "910B4",
			},
		},
		{
			name: "amd64-910B4-2 (chip pattern)",
			spec: "amd64-910B4-2",
			want: map[string]string{
				"kubernetes.io/arch":               "amd64",
				"node.kubernetes.io/npu.chip.name": "910B4",
			},
		},
		{
			name: "arm64-unknownChip-1 (invalid chip - no selector)",
			spec: "arm64-unknownChip-1",
			want: map[string]string{
				"kubernetes.io/arch": "arm64",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertJobArch(tt.spec)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("convertJobArch() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
