package converter

import (
	"testing"

	"github.com/opensourceways/codearts-workflow-image-go/cmd/common/testutil"
	"github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"
	"go.yaml.in/yaml/v3"
)

func Test_convertEnv(t *testing.T) {
	type args struct {
		env map[string]string
	}
	tests := []struct {
		name    string
		args    args
		want    []volcano.EnvVar
		wantErr bool
	}{
		{
			name: "empty map",
			args: args{
				env: map[string]string{},
			},
			want:    []volcano.EnvVar{},
			wantErr: false,
		},
		{
			name: "single env var",
			args: args{
				env: map[string]string{
					"FOO": "bar",
				},
			},
			want: []volcano.EnvVar{
				{Name: "FOO", Value: "bar"},
			},
			wantErr: false,
		},
		{
			name: "multiple env vars",
			args: args{
				env: map[string]string{
					"DB_HOST": "localhost",
					"DB_PORT": "5432",
					"DEBUG":   "true",
				},
			},
			want: []volcano.EnvVar{
				{Name: "DB_HOST", Value: "localhost"},
				{Name: "DB_PORT", Value: "5432"},
				{Name: "DEBUG", Value: "true"},
			},
			wantErr: false,
		},
		{
			name: "env var with empty value",
			args: args{
				env: map[string]string{
					"EMPTY": "",
				},
			},
			want: []volcano.EnvVar{
				{Name: "EMPTY", Value: ""},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertEnv(tt.args.env)
			gotYaml, _ := yaml.Marshal(got)
			wantYaml, _ := yaml.Marshal(tt.want)
			if ok, err := testutil.YamlEqual(gotYaml, wantYaml); !ok || err != nil {
				t.Errorf("convertEnv() YAML mismatch, err=%v", err)
			}
		})
	}
}
