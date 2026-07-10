package converter

import "testing"

func Test_makeArgoTemplateName(t *testing.T) {
	type args struct {
		parts []string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "normal case with mixed separators",
			args: args{parts: []string{"My_YAML.yaml", "Build-App_v2", "CI/CD-Job", "github.com/user/repo"}},
			want: "my-yaml-yaml-build-app-v2-ci-cd-job-github-com-user-repo",
		},
		{
			name: "all valid characters",
			args: args{parts: []string{"valid", "name123", "test.example"}},
			want: "valid-name123-test-example",
		},
		{
			name: "uppercase conversion",
			args: args{parts: []string{"UPPER", "Case"}},
			want: "upper-case",
		},
		{
			name: "special characters stripped",
			args: args{parts: []string{"job@v1!", "repo#2024", "user$name"}},
			want: "job-v1-repo-2024-user-name",
		},
		{
			name: "leading/trailing illegal chars",
			args: args{parts: []string{"_start", "end_", ".dot.", "-dash-"}},
			want: "start-end-dot-dash",
		},
		{
			name: "empty parts ignored",
			args: args{parts: []string{"", "valid", "", "part"}},
			want: "valid-part",
		},
		{
			name: "only invalid characters",
			args: args{parts: []string{"@@@", "###", ""}},
			want: "codearts-build",
		},
		{
			name: "long name truncated to 63 chars",
			args: args{parts: []string{
				"this-is-a-very-long-component-name-that-will-push-the-total-length-over-63-characters",
				"extra",
			}},
			want: "this-is-a-very-long-component-name-that-will-push-the-total-len",
		},
		{
			name: "truncation avoids trailing hyphen",
			args: args{parts: []string{
				"almost63chars123456789012345678901234567890123456789012345678901234567890---",
			}},
			want: "almost63chars12345678901234567890123456789012345678901234567890",
		},
		{
			name: "dots and slashes become hyphens",
			args: args{parts: []string{"org.example/app", "v1.2.3"}},
			want: "org-example-app-v1-2-3",
		},
		{
			name: "single valid part",
			args: args{parts: []string{"simple"}},
			want: "simple",
		},
		{
			name: "all empty parts",
			args: args{parts: []string{"", "", ""}},
			want: "codearts-build",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := makeArgoTemplateName(tt.args.parts...)
			if got != tt.want {
				t.Errorf("makeArgoTemplateName() = %v, want %v", got, tt.want)
			}
			// Additional validation: ensure result is a valid DNS label
			if len(got) == 0 || len(got) > 63 {
				t.Errorf("makeArgoTemplateName() produced invalid length: %d", len(got))
			}
			if !isValidDNSLabel(got) {
				t.Errorf("makeArgoTemplateName() produced invalid DNS label: %q", got)
			}
		})
	}
}

// Helper to validate DNS label (as per Kubernetes)
func isValidDNSLabel(name string) bool {
	if name == "" || len(name) > 63 {
		return false
	}
	for i, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			continue
		}
		if c == '-' {
			if i == 0 || i == len(name)-1 {
				return false // cannot start or end with '-'
			}
			continue
		}
		return false
	}
	return true
}
