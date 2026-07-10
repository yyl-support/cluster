package namespace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetNamespaceFromRepoName(t *testing.T) {
	tests := []struct {
		name     string
		repoName string
		expected string
	}{
		{
			name:     "ragsdk repo",
			repoName: "ragsdk/repo",
			expected: RagsdkNamespace,
		},
		{
			name:     "ragsdk in name",
			repoName: "my-ragsdk-tool",
			expected: RagsdkNamespace,
		},
		{
			name:     "ascendnpu-ir repo",
			repoName: "Ascend/AscendNPU-IR",
			expected: RagsdkNamespace,
		},
		{
			name:     "ascendnpu-ir lowercase",
			repoName: "ascend/ascendnpu-ir",
			expected: RagsdkNamespace,
		},
		{
			name:     "test repo",
			repoName: "testorg/testrepo",
			expected: DefaultNamespace,
		},
		{
			name:     "test prefix",
			repoName: "testorg/test-prefix",
			expected: DefaultNamespace,
		},
		{
			name:     "testorg testrepo",
			repoName: "testorg/testrepo-test1",
			expected: DefaultNamespace,
		},
		{
			name:     "multimodalsdk repo",
			repoName: "ascend-multimodalsdk/repo",
			expected: MultimodalsdkNamespace,
		},
		{
			name:     "multimodalsdk in name",
			repoName: "my-ascend-multimodalsdk-tool",
			expected: MultimodalsdkNamespace,
		},
		{
			name:     "indexsdk repo",
			repoName: "ascend-indexsdk/repo",
			expected: IndexsdkNamespace,
		},
		{
			name:     "indexsdk in name",
			repoName: "my-ascend-indexsdk-tool",
			expected: IndexsdkNamespace,
		},
		{
			name:     "random repo",
			repoName: "random-org/random-repo",
			expected: DefaultNamespace,
		},
		{
			name:     "empty repo name",
			repoName: "",
			expected: DefaultNamespace,
		},
		{
			name:     "uppercase ragsdk",
			repoName: "RAGSDK/repo",
			expected: RagsdkNamespace,
		},
		{
			name:     "uppercase test prefix",
			repoName: "TESTORG/test-repo",
			expected: DefaultNamespace,
		},
		{
			name:     "uppercase test repo",
			repoName: "TESTORG/TESTREPO",
			expected: DefaultNamespace,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetNamespaceFromRepoName(tt.repoName)
			if result != tt.expected {
				t.Errorf("GetNamespaceFromRepoName(%q) = %q, want %q", tt.repoName, result, tt.expected)
			}
		})
	}
}

func TestGetRepoNameFromWorkflow(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		expected     string
		expectError  bool
		errorContain string
	}{
		{
			name: "with jobRepositoryName label",
			yamlContent: `apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
  labels:
    jobRepositoryName: testorg/testrepo
spec:
  tasks: []`,
			expected: "testorg/testrepo",
		},
		{
			name: "without jobRepositoryName label",
			yamlContent: `apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
  labels:
    other-label: value
spec:
  tasks: []`,
			expected: "",
		},
		{
			name: "no labels at all",
			yamlContent: `apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
  name: test-job
spec:
  tasks: []`,
			expected: "",
		},
		{
			name:         "invalid yaml",
			yamlContent:  `invalid: yaml: content: [`,
			expectError:  true,
			errorContain: "failed to parse workflow YAML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			workflowPath := filepath.Join(tmpDir, "workflow.yaml")

			if tt.expectError && tt.yamlContent == "" {
				workflowPath = filepath.Join(tmpDir, "nonexistent.yaml")
			} else {
				err := os.WriteFile(workflowPath, []byte(tt.yamlContent), 0644)
				if err != nil {
					t.Fatalf("Failed to write test file: %v", err)
				}
			}

			result, err := GetRepoNameFromWorkflow(workflowPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorContain)
				} else if tt.errorContain != "" && !contains(err.Error(), tt.errorContain) {
					t.Errorf("Expected error containing %q, got %q", tt.errorContain, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("GetRepoNameFromWorkflow() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetRepoNameFromWorkflowFileNotFound(t *testing.T) {
	_, err := GetRepoNameFromWorkflow("/nonexistent/path/workflow.yaml")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
	if !contains(err.Error(), "failed to read workflow file") {
		t.Errorf("Expected error containing 'failed to read workflow file', got %q", err.Error())
	}
}

func TestGetNamespaceFromWorkflow(t *testing.T) {
	tests := []struct {
		name         string
		yamlContent  string
		expected     string
		expectError  bool
		errorContain string
	}{
		{
			name: "ragsdk namespace from workflow",
			yamlContent: `apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
  labels:
    jobRepositoryName: ragsdk/repo
spec:
  tasks: []`,
			expected: RagsdkNamespace,
		},
		{
			name: "default namespace from workflow",
			yamlContent: `apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
  labels:
    jobRepositoryName: other-org/repo
spec:
  tasks: []`,
			expected: DefaultNamespace,
		},
		{
			name: "empty repo name defaults to argo",
			yamlContent: `apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
  labels: {}
spec:
  tasks: []`,
			expected: DefaultNamespace,
		},
		{
			name:         "invalid yaml returns default with error",
			yamlContent:  `invalid: yaml: [`,
			expected:     DefaultNamespace,
			expectError:  true,
			errorContain: "failed to parse workflow YAML",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			workflowPath := filepath.Join(tmpDir, "workflow.yaml")

			err := os.WriteFile(workflowPath, []byte(tt.yamlContent), 0644)
			if err != nil {
				t.Fatalf("Failed to write test file: %v", err)
			}

			result, err := GetNamespaceFromWorkflow(workflowPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error containing %q, got nil", tt.errorContain)
				} else if tt.errorContain != "" && !contains(err.Error(), tt.errorContain) {
					t.Errorf("Expected error containing %q, got %q", tt.errorContain, err.Error())
				}
			} else if err != nil {
				t.Fatalf("Unexpected error: %v", err)
			}

			if result != tt.expected {
				t.Errorf("GetNamespaceFromWorkflow() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[0:len(substr)] == substr || len(s) > len(substr) && contains(s[1:], substr)
}