package main

import (
	"fmt"
	"testing"
)

func TestFormatPodFailureError(t *testing.T) {
	tests := []struct {
		name        string
		phase       string
		originalErr error
		mockReason  string
		wantContain string
	}{
		{
			name:        "with reason and error",
			phase:       "Failed",
			originalErr: fmt.Errorf("some error"),
			mockReason:  "ContainerCannotRun, exitCode=137, message=OOMKilled",
			wantContain: "原因: ContainerCannotRun",
		},
		{
			name:        "with reason only",
			phase:       "Failed",
			originalErr: nil,
			mockReason:  "Error, exitCode=1",
			wantContain: "原因: Error",
		},
		{
			name:        "without reason but with error",
			phase:       "Failed",
			originalErr: fmt.Errorf("timeout"),
			mockReason:  "",
			wantContain: "错误: timeout",
		},
		{
			name:        "without reason and error",
			phase:       "Failed",
			originalErr: nil,
			mockReason:  "",
			wantContain: "状态: Failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				KubeconfigPath: "/tmp/test.yaml",
				Namespace:      "test",
			}

			err := formatPodFailureError(cfg, "test-pod", "test-job", tt.phase, tt.originalErr)
			if err == nil {
				t.Fatal("expected error, got nil")
			}

			errStr := err.Error()
			if !contains(errStr, tt.phase) {
				t.Errorf("error should contain phase %q, got %q", tt.phase, errStr)
			}
			if !contains(errStr, "test-job") {
				t.Errorf("error should contain jobName %q, got %q", "test-job", errStr)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
