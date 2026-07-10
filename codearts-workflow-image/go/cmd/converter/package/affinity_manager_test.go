package converter

import (
	"testing"

	"github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"
)

func TestAddNPUAffinity(t *testing.T) {
	tests := []struct {
		name     string
		podSpec  *volcano.PodSpec
		runsOn   string
		wantAff  bool
		wantExpr []struct {
			key      string
			operator string
			values   []string
		}
	}{
		{
			name:    "nil_podSpec",
			podSpec: nil,
			runsOn:  "arm64-910b4-1",
			wantAff: false,
		},
		{
			name:    "no_npu",
			podSpec: &volcano.PodSpec{},
			runsOn:  "arm64-cpu-8-mem-16G",
			wantAff: false,
		},
		{
			name:    "npu_is_310p3",
			podSpec: &volcano.PodSpec{},
			runsOn:  "arm64-310p3-1",
			wantAff: false,
		},
		{
			name:    "npu_is_910b4",
			podSpec: &volcano.PodSpec{},
			runsOn:  "arm64-910b4-1",
			wantAff: false,
		},
		{
			name:    "npu_is_910a",
			podSpec: &volcano.PodSpec{},
			runsOn:  "arm64-910a-2",
			wantAff: false,
		},
		{
			name:    "npu_generic",
			podSpec: &volcano.PodSpec{},
			runsOn:  "arm64-npu-1",
			wantAff: true,
			wantExpr: []struct {
				key      string
				operator string
				values   []string
			}{
				{key: "node.kubernetes.io/npu.chip.name", operator: "NotIn", values: []string{"310P3"}},
			},
		},
		{
			name:    "npu_uppercase",
			podSpec: &volcano.PodSpec{},
			runsOn:  "arm64-NPU-1",
			wantAff: true,
			wantExpr: []struct {
				key      string
				operator string
				values   []string
			}{
				{key: "node.kubernetes.io/npu.chip.name", operator: "NotIn", values: []string{"310P3"}},
			},
		},
		{
			name: "existing_affinity_preserved",
			podSpec: &volcano.PodSpec{
				Affinity: &volcano.Affinity{
					NodeAffinity: &volcano.NodeAffinity{
						RequiredDuringSchedulingIgnoredDuringExecution: &volcano.NodeSelector{
							NodeSelectorTerms: []volcano.NodeSelectorTerm{
								{
									MatchExpressions: []volcano.NodeSelectorRequirement{
										{Key: "kubernetes.io/arch", Operator: "In", Values: []string{"arm64"}},
									},
								},
							},
						},
					},
				},
			},
			runsOn:  "arm64-npu-1",
			wantAff: true,
			wantExpr: []struct {
				key      string
				operator string
				values   []string
			}{
				{key: "kubernetes.io/arch", operator: "In", values: []string{"arm64"}},
				{key: "node.kubernetes.io/npu.chip.name", operator: "NotIn", values: []string{"310P3"}},
			},
		},
		{
			name:    "invalid_runs_on",
			podSpec: &volcano.PodSpec{},
			runsOn:  "invalid",
			wantAff: false,
		},
		{
			name:    "empty_runs_on",
			podSpec: &volcano.PodSpec{},
			runsOn:  "",
			wantAff: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddNPUAffinity(tt.podSpec, tt.runsOn)

			if !tt.wantAff {
				if tt.podSpec != nil && tt.podSpec.Affinity != nil {
					t.Errorf("expected no affinity, got affinity")
				}
				return
			}

			if tt.podSpec == nil || tt.podSpec.Affinity == nil {
				t.Errorf("expected affinity, got nil")
				return
			}

			nodeAff := tt.podSpec.Affinity.NodeAffinity
			if nodeAff == nil || nodeAff.RequiredDuringSchedulingIgnoredDuringExecution == nil {
				t.Errorf("expected nodeAffinity with required selector, got nil")
				return
			}

			terms := nodeAff.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
			if len(terms) == 0 {
				t.Errorf("expected at least one nodeSelectorTerm")
				return
			}

			exprs := terms[0].MatchExpressions
			for i, want := range tt.wantExpr {
				if i >= len(exprs) {
					t.Errorf("missing expression %d: want %s %s %v", i, want.key, want.operator, want.values)
					continue
				}
				got := exprs[i]
				if got.Key != want.key {
					t.Errorf("expression %d: key mismatch, got %s, want %s", i, got.Key, want.key)
				}
				if got.Operator != want.operator {
					t.Errorf("expression %d: operator mismatch, got %s, want %s", i, got.Operator, want.operator)
				}
				if len(got.Values) != len(want.values) {
					t.Errorf("expression %d: values length mismatch, got %v, want %v", i, got.Values, want.values)
				}
				for j, v := range want.values {
					if j >= len(got.Values) || got.Values[j] != v {
						t.Errorf("expression %d: value %d mismatch", i, j)
					}
				}
			}
		})
	}
}
