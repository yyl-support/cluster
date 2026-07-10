package converter

import (
	"os"
	"testing"
)

// DISABLED: sensitivePatterns are empty, IsSensitiveEnvName always returns false
// func TestIsSensitiveEnvName(t *testing.T) {
// 	tests := []struct {
// 		name     string
// 		input    string
// 		expected bool
// 	}{
// 		{"password lowercase", "password", true},
// 		{"password uppercase", "PASSWORD", true},
// 		{"password mixed case", "MyPassword", true},
// 		{"passwd", "passwd", true},
// 		{"token", "api_token", true},
// 		{"TOKEN uppercase", "API_TOKEN", true},
// 		{"access", "ACCESS_KEY", true},
// 		{"ak", "AK", true},
// 		{"sk", "SK", true},
// 		{"secret", "my_secret", true},
// 		{"key", "api_key", true},
// 		{"credential", "CREDENTIAL", true},
// 		{"db_password", "DB_PASSWORD", true},
// 		{"access_key", "access_key", true},
// 		{"plain string", "plain_var", false},
// 		{"username", "username", false},
// 		{"path", "PATH", false},
// 		{"home", "HOME", false},
// 		{"workspace", "WORKSPACE", false},
// 		{"empty", "", false},
// 	}
//
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			result := IsSensitiveEnvName(tt.input)
// 			if result != tt.expected {
// 				t.Errorf("IsSensitiveEnvName(%q) = %v, want %v", tt.input, result, tt.expected)
// 			}
// 		})
// 	}
// }

// DISABLED: sensitivePatterns are empty, no secrets are filtered
// func TestFilterSensitiveEnv(t *testing.T) {
// 	env := map[string]string{
// 		"DB_PASSWORD":    "secret1",
// 		"API_TOKEN":      "secret2",
// 		"PLAIN_VAR":      "plain1",
// 		"USER_NAME":      "plain2",
// 		"ACCESS_KEY":     "secret3",
// 		"CREDENTIAL_VAR": "secret4",
// 		"HOME_DIR":       "/home",
// 	}
//
// 	sensitive, plain := FilterSensitiveEnv(env)
//
// 	expectedSensitive := map[string]string{
// 		"DB_PASSWORD":    "secret1",
// 		"API_TOKEN":      "secret2",
// 		"ACCESS_KEY":     "secret3",
// 		"CREDENTIAL_VAR": "secret4",
// 	}
//
// 	expectedPlain := map[string]string{
// 		"PLAIN_VAR": "plain1",
// 		"USER_NAME": "plain2",
// 		"HOME_DIR":  "/home",
// 	}
//
// 	for k, v := range expectedSensitive {
// 		if val, exists := sensitive[k]; !exists || val != v {
// 			t.Errorf("Sensitive env: expected %s=%s, got %s=%s", k, v, k, val)
// 		}
// 	}
//
// 	for k, v := range expectedPlain {
// 		if val, exists := plain[k]; !exists || val != v {
// 			t.Errorf("Plain env: expected %s=%s, got %s=%s", k, v, k, val)
// 		}
// 	}
//
// 	if len(sensitive) != len(expectedSensitive) {
// 		t.Errorf("Sensitive count: got %d, want %d", len(sensitive), len(expectedSensitive))
// 	}
//
// 	if len(plain) != len(expectedPlain) {
// 		t.Errorf("Plain count: got %d, want %d", len(plain), len(expectedPlain))
// 	}
// }

// DISABLED: sensitivePatterns are empty, no secrets are extracted from scripts
// func TestExtractSensitiveFromScript(t *testing.T) {
// 	tests := []struct {
// 		name            string
// 		script          string
// 		envSetup        func()
// 		expectedKeys    []string
// 		expectCleanLine string
// 	}{
// 		{
// 			name: "export assignment",
// 			script: `#!/bin/bash
// export DB_PASSWORD="my-password"
// export API_TOKEN="my-token"
// echo "done"`,
// 			envSetup: func() {
// 				os.Setenv("DB_PASSWORD", "test-db-password")
// 				os.Setenv("API_TOKEN", "test-api-token")
// 			},
// 			expectedKeys: []string{"DB_PASSWORD", "API_TOKEN"},
// 		},
// 		{
// 			name: "direct assignment",
// 			script: `SECRET_KEY=my-secret
// echo $SECRET_KEY`,
// 			envSetup: func() {
// 				os.Setenv("SECRET_KEY", "test-secret-key")
// 			},
// 			expectedKeys: []string{"SECRET_KEY"},
// 		},
// 		{
// 			name: "direct assignment",
// 			script: `SECRET_KEY=my-secret
// echo $SECRET_KEY`,
// 			envSetup:        func() {},
// 			expectedKeys:    []string{"SECRET_KEY"},
// 			expectCleanLine: "echo $SECRET_KEY",
// 		},
// 		{
// 			name: "plain assignment",
// 			script: `PLAIN_VAR=plain-value
// echo $PLAIN_VAR`,
// 			envSetup:        func() {},
// 			expectedKeys:    []string{},
// 			expectCleanLine: "PLAIN_VAR=plain-value\necho $PLAIN_VAR",
// 		},
// 		{
// 			name:   "${} reference",
// 			script: `echo "Password is ${DB_PASSWORD}"`,
// 			envSetup: func() {
// 				os.Setenv("DB_PASSWORD", "test-password")
// 			},
// 			expectedKeys:    []string{"DB_PASSWORD"},
// 			expectCleanLine: `echo "Password is ${DB_PASSWORD}"`,
// 		},
// 		{
// 			name: "mixed export and reference",
// 			script: `export API_TOKEN="token"
// echo ${ACCESS_KEY}`,
// 			envSetup: func() {
// 				os.Setenv("API_TOKEN", "test-api-token")
// 				os.Setenv("ACCESS_KEY", "test-access-key")
// 			},
// 			expectedKeys: []string{"API_TOKEN", "ACCESS_KEY"},
// 		},
// 	}
//
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			defer os.Unsetenv("DB_PASSWORD")
// 			defer os.Unsetenv("ACCESS_KEY")
//
// 			tt.envSetup()
//
// 			sensitive, _ := ExtractSensitiveFromScript(tt.script)
//
// 			for _, key := range tt.expectedKeys {
// 				if _, exists := sensitive[key]; !exists {
// 					t.Errorf("Expected key %s not found in sensitive map", key)
// 				}
// 				if sensitive[key] == "" && key != "" {
// 					t.Errorf("Expected non-empty value for key %s", key)
// 				}
// 			}
//
// 			if len(sensitive) != len(tt.expectedKeys) {
// 				t.Errorf("Got %d sensitive keys, want %d", len(sensitive), len(tt.expectedKeys))
// 			}
// 		})
// 	}
// }

func TestResolveSensitiveEnvValues(t *testing.T) {
	defer os.Unsetenv("DB_PASSWORD")
	defer os.Unsetenv("API_TOKEN")
	defer os.Unsetenv("MY_TOKEN")

	os.Setenv("DB_PASSWORD", "actual-db-password")
	os.Setenv("API_TOKEN", "actual-api-token")

	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name: "resolved from env",
			input: map[string]string{
				"DB_PASSWORD": "${DB_PASSWORD}",
				"API_TOKEN":   "${API_TOKEN}",
			},
			expected: map[string]string{
				"DB_PASSWORD": "actual-db-password",
				"API_TOKEN":   "actual-api-token",
			},
		},
		{
			name: "plain value unchanged",
			input: map[string]string{
				"PLAIN_VAR": "plain-value",
			},
			expected: map[string]string{
				"PLAIN_VAR": "plain-value",
			},
		},
		{
			name: "undefined env var stays as placeholder",
			input: map[string]string{
				"TOKEN": "${MY_TOKEN}",
			},
			expected: map[string]string{
				"TOKEN": "${MY_TOKEN}",
			},
		},
		{
			name: "empty env var stays as placeholder",
			input: map[string]string{
				"EMPTY_VAR": "${EMPTY_VAR}",
			},
			expected: map[string]string{
				"EMPTY_VAR": "${EMPTY_VAR}",
			},
		},
		{
			name: "mixed resolved and unresolved",
			input: map[string]string{
				"DB_PASSWORD": "${DB_PASSWORD}",
				"PLAIN":       "plain",
				"UNDEFINED":   "${UNKNOWN_VAR}",
			},
			expected: map[string]string{
				"DB_PASSWORD": "actual-db-password",
				"PLAIN":       "plain",
				"UNDEFINED":   "${UNKNOWN_VAR}",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ResolveSensitiveEnvValues(tt.input)
			for k, expectedVal := range tt.expected {
				if val, exists := result[k]; !exists || val != expectedVal {
					t.Errorf("ResolveSensitiveEnvValues[%s] = %q, want %q", k, val, expectedVal)
				}
			}
			if len(result) != len(tt.expected) {
				t.Errorf("Got %d entries, want %d", len(result), len(tt.expected))
			}
		})
	}
}

func TestMergeSensitiveEnvs(t *testing.T) {
	env1 := map[string]string{
		"DB_PASSWORD": "from-env1",
		"API_TOKEN":   "from-env1",
	}

	env2 := map[string]string{
		"ACCESS_KEY": "from-env2",
		"API_TOKEN":  "from-env2", // duplicate
	}

	merged := MergeSensitiveEnvs(env1, env2)

	if merged["DB_PASSWORD"] != "from-env1" {
		t.Errorf("DB_PASSWORD: got %s, want from-env1", merged["DB_PASSWORD"])
	}

	if merged["ACCESS_KEY"] != "from-env2" {
		t.Errorf("ACCESS_KEY: got %s,", merged["ACCESS_KEY"])
	}

	if merged["API_TOKEN"] != "from-env1" {
		t.Errorf("API_TOKEN: expected first value preserved, got %s", merged["API_TOKEN"])
	}

	if len(merged) != 3 {
		t.Errorf("Got %d merged entries, want 3", len(merged))
	}
}

// DISABLED: sensitivePatterns are empty, no secrets are skipped during env var replacement
// func TestReplaceEnvVarsInStringSkippingSensitive(t *testing.T) {
// 	defer os.Unsetenv("DB_PASSWORD")
// 	defer os.Unsetenv("API_TOKEN")
// 	defer os.Unsetenv("PLAIN_VAR")
// 	defer os.Unsetenv("WORKSPACE")
//
// 	os.Setenv("DB_PASSWORD", "secret-db-pass")
// 	os.Setenv("API_TOKEN", "secret-api-token")
// 	os.Setenv("PLAIN_VAR", "plain-value")
// 	os.Setenv("WORKSPACE", "/workspace")
//
// 	tests := []struct {
// 		name     string
// 		input    string
// 		expected string
// 	}{
// 		{
// 			name:     "sensitive password kept as placeholder",
// 			input:    "password is ${DB_PASSWORD}",
// 			expected: "password is ${DB_PASSWORD}",
// 		},
// 		{
// 			name:     "sensitive token kept as placeholder",
// 			input:    "token is ${API_TOKEN}",
// 			expected: "token is ${API_TOKEN}",
// 		},
// 		{
// 			name:     "plain variable rendered",
// 			input:    "value is ${PLAIN_VAR}",
// 			expected: "value is plain-value",
// 		},
// 		{
// 			name:     "workspace kept as placeholder",
// 			input:    "workspace is ${WORKSPACE}",
// 			expected: "workspace is ${WORKSPACE}",
// 		},
// 		{
// 			name:     "mixed sensitive and plain",
// 			input:    "${DB_PASSWORD} and ${PLAIN_VAR}",
// 			expected: "${DB_PASSWORD} and plain-value",
// 		},
// 		{
// 			name:     "undefined var kept as placeholder",
// 			input:    "undefined is ${UNDEFINED_VAR}",
// 			expected: "undefined is ${UNDEFINED_VAR}",
// 		},
// 		{
// 			name:     "multiple sensitive vars",
// 			input:    "${DB_PASSWORD} ${API_TOKEN}",
// 			expected: "${DB_PASSWORD} ${API_TOKEN}",
// 		},
// 		{
// 			name:     "all sensitive patterns",
// 			input:    "${SECRET_KEY} ${ACCESS_KEY} ${CREDENTIAL}",
// 			expected: "${SECRET_KEY} ${ACCESS_KEY} ${CREDENTIAL}",
// 		},
// 		{
// 			name:     "non-sensitive rendered",
// 			input:    "${CUSTOM_VAR} ${ANOTHER_VAR}",
// 			expected: "${CUSTOM_VAR} ${ANOTHER_VAR}",
// 		},
// 	}
//
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			result := ReplaceEnvVarsInStringSkippingSensitive(tt.input)
// 			if result != tt.expected {
// 				t.Errorf("ReplaceEnvVarsInStringSkippingSensitive(%q) = %q, want %q", tt.input, result, tt.expected)
// 			}
// 		})
// 	}
// }
