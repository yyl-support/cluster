package converter

import (
	"testing"

	"github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/gitcode"
)

func Test_convertUsesToString(t *testing.T) {
	type args struct {
		jobStep    gitcode.Step
		prefixPath string
	}

	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "helloworld",
			args: args{
				jobStep: gitcode.Step{
					Uses: "actions/helloworld",
					With: map[string]string{"greeting": "hello test"},
				},
				prefixPath: "../case",
			},
			want: `
echo 'step: Say hello 1'
echo "hello test from composite action!"
echo 'step: Say hello 2'
uname -m`,
			wantErr: false,
		},

		{
			name: "helloworld",
			args: args{
				jobStep: gitcode.Step{
					Uses: "./../case/actions/recursivecall",
					With: map[string]string{"greeting": "hello test"},
				},
				prefixPath: "../case",
			},
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := convertUsesToString(tt.args.jobStep, tt.args.prefixPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("convertUsesToString() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("convertUsesToString() = %v, want %v", got, tt.want)
			}
		})
	}
}
