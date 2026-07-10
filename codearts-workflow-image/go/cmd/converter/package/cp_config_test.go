package converter

import (
	"os"
	"testing"
)

func TestGetCPConfig_Timeout(t *testing.T) {
	tests := []struct {
		name            string
		cpTimeout       string
		expectedSeconds int
	}{
		{
			name:            "default_timeout_when_not_set",
			cpTimeout:       "",
			expectedSeconds: 14400,
		},
		{
			name:            "timeout_1_hour",
			cpTimeout:       "1",
			expectedSeconds: 3600,
		},
		{
			name:            "timeout_2_hours",
			cpTimeout:       "2",
			expectedSeconds: 7200,
		},
		{
			name:            "timeout_8_hours",
			cpTimeout:       "8",
			expectedSeconds: 28800,
		},
		{
			name:            "timeout_24_hours",
			cpTimeout:       "24",
			expectedSeconds: 86400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			os.Setenv("CP_runs_on", "amd64")

			if tt.cpTimeout != "" {
				os.Setenv("CP_timeout", tt.cpTimeout)
			}

			_, _, _, _, _, _, _, _, _, _, _, _, _, cpTimeoutSeconds, _ := GetCPConfig()

			if cpTimeoutSeconds != tt.expectedSeconds {
				t.Errorf("expected timeout %d seconds, got %d", tt.expectedSeconds, cpTimeoutSeconds)
			}
		})
	}
}

func TestGetCPConfig_ArtifactsDefault(t *testing.T) {
	os.Clearenv()
	os.Setenv("CP_runs_on", "amd64")
	os.Setenv("CP_artifacts", "/output/artifacts")

	_, _, _, _, _, _, cpArtifacts, cpArtifactsTempFolder, _, _, _, _, _, _, _ := GetCPConfig()

	if cpArtifacts != "/output/artifacts" {
		t.Errorf("expected cpArtifacts /output/artifacts, got %s", cpArtifacts)
	}
	if cpArtifactsTempFolder != "/output" {
		t.Errorf("expected cpArtifactsTempFolder /output, got %s", cpArtifactsTempFolder)
	}
}

func TestGetCPConfig_FilterEnv(t *testing.T) {
	os.Clearenv()
	os.Setenv("CP_runs_on", "amd64")
	os.Setenv("CP_timeout", "$UNSET_VAR")

	_, _, _, _, _, _, _, _, _, _, _, _, _, cpTimeoutSeconds, _ := GetCPConfig()

	if cpTimeoutSeconds != 14400 {
		t.Errorf("expected default timeout when CP_timeout starts with $, got %d", cpTimeoutSeconds)
	}
}

func TestNormalizeShmSize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "8G to 8Gi",
			input:    "8G",
			expected: "8Gi",
		},
		{
			name:     "512M to 512Mi",
			input:    "512M",
			expected: "512Mi",
		},
		{
			name:     "already has i suffix",
			input:    "8Gi",
			expected: "8Gi",
		},
		{
			name:     "already has i suffix Mi",
			input:    "512Mi",
			expected: "512Mi",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeShmSize(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeShmSize(%s) = %s, want %s", tt.input, result, tt.expected)
			}
		})
	}
}

func TestGetCPConfig_Shm(t *testing.T) {
	tests := []struct {
		name        string
		cpShm       string
		expectedShm string
	}{
		{
			name:        "empty_shm",
			cpShm:       "",
			expectedShm: "",
		},
		{
			name:        "shm_8G",
			cpShm:       "8G",
			expectedShm: "8G",
		},
		{
			name:        "shm_512Mi",
			cpShm:       "512Mi",
			expectedShm: "512Mi",
		},
		{
			name:        "shm_filtered_env",
			cpShm:       "$UNSET_VAR",
			expectedShm: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			os.Setenv("CP_runs_on", "amd64")

			if tt.cpShm != "" {
				os.Setenv("CP_shm", tt.cpShm)
			}

			_, _, _, _, _, _, _, _, _, _, cpShm, _, _, _, _ := GetCPConfig()

		if cpShm != tt.expectedShm {
			t.Errorf("expected cpShm %s, got %s", tt.expectedShm, cpShm)
		}
		})
	}
}

func TestGetCPConfig_DelayExit(t *testing.T) {
	tests := []struct {
		name            string
		cpDelayExit     string
		expectedSeconds int
	}{
		{
			name:            "default_when_not_set",
			cpDelayExit:     "",
			expectedSeconds: 10,
		},
		{
			name:            "custom_30_seconds",
			cpDelayExit:     "30",
			expectedSeconds: 30,
		},
		{
			name:            "zero_seconds_valid",
			cpDelayExit:     "0",
			expectedSeconds: 0,
		},
		{
			name:            "negative_rejected_falls_back_to_default",
			cpDelayExit:     "-5",
			expectedSeconds: 10,
		},
		{
			name:            "non_numeric_rejected_falls_back_to_default",
			cpDelayExit:     "abc",
			expectedSeconds: 10,
		},
		{
			name:            "unresolved_shell_var_filtered_falls_back_to_default",
			cpDelayExit:     "$UNSET_VAR",
			expectedSeconds: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Clearenv()
			os.Setenv("CP_runs_on", "amd64")

			if tt.cpDelayExit != "" {
				os.Setenv("CP_delay_exit", tt.cpDelayExit)
			}

			_, _, _, _, _, _, _, _, _, _, _, _, _, _, cpDelayExitSeconds := GetCPConfig()

			if cpDelayExitSeconds != tt.expectedSeconds {
				t.Errorf("expected delay exit %d seconds, got %d", tt.expectedSeconds, cpDelayExitSeconds)
			}
		})
	}
}

func TestNormalizeImagePullPolicy(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"IfNotPresent", "IfNotPresent"},
		{"ifnotpresent", "IfNotPresent"},
		{"IFNOTPRESENT", "IfNotPresent"},
		{"Always", "Always"},
		{"always", "Always"},
		{"ALWAYS", "Always"},
		{"Never", "Never"},
		{"never", "Never"},
		{"NEVER", "Never"},
		{"", ""},
		{"invalid", ""},
		{"once", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := normalizeImagePullPolicy(tt.input)
			if got != tt.expected {
				t.Errorf("normalizeImagePullPolicy(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
