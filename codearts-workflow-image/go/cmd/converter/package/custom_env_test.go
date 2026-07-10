package converter

import (
	"testing"

	"github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"
)

func TestAddCustomEnv(t *testing.T) {
	tests := []struct {
		name      string
		envVars   map[string]string
		resources volcano.Resources
		want      map[string]string
	}{
		{
			name:    "empty env with cpu",
			envVars: map[string]string{},
			resources: volcano.Resources{
				Requests: volcano.ResourceList{"cpu": "2"},
			},
			want: map[string]string{
				"MAX_JOBS":                   "2",
				"CMAKE_BUILD_PARALLEL_LEVEL": "2",
			},
		},
		{
			name: "existing env with cpu",
			envVars: map[string]string{
				"WORKSPACE": "/workspace",
				"FOO":       "bar",
			},
			resources: volcano.Resources{
				Requests: volcano.ResourceList{"cpu": "8"},
			},
			want: map[string]string{
				"WORKSPACE":                  "/workspace",
				"FOO":                        "bar",
				"MAX_JOBS":                   "8",
				"CMAKE_BUILD_PARALLEL_LEVEL": "8",
			},
		},
		{
			name: "no cpu in requests",
			envVars: map[string]string{
				"WORKSPACE": "/workspace",
			},
			resources: volcano.Resources{
				Requests: volcano.ResourceList{"memory": "8Gi"},
			},
			want: map[string]string{
				"WORKSPACE": "/workspace",
			},
		},
		{
			name: "empty requests",
			envVars: map[string]string{
				"WORKSPACE": "/workspace",
			},
			resources: volcano.Resources{},
			want: map[string]string{
				"WORKSPACE": "/workspace",
			},
		},
		{
			name:    "nil envVars",
			envVars: nil,
			resources: volcano.Resources{
				Requests: volcano.ResourceList{"cpu": "4"},
			},
			want: map[string]string{
				"MAX_JOBS":                   "4",
				"CMAKE_BUILD_PARALLEL_LEVEL": "4",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := AddCustomEnv(tt.envVars, tt.resources)
			for k, v := range tt.want {
				if got[k] != v {
					t.Errorf("AddCustomEnv()[%s] = %s, want %s", k, got[k], v)
				}
			}
			if len(got) != len(tt.want) {
				t.Errorf("AddCustomEnv() returned %d keys, want %d", len(got), len(tt.want))
			}
		})
	}
}
