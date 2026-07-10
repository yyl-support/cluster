# Namespace Mapping Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add namespace mapping in submitter to route workflows to different namespaces based on repository name label.

**Architecture:** Submitter reads `jobRepositoryName` label from workflow.yaml, maps repo name to namespace via hardcoded rules, submits Job and Secret to mapped namespace. Converter unchanged.

**Tech Stack:** Go, go.yaml.in/yaml/v3, kubectl

---

## File Structure

| File | Purpose |
|------|---------|
| `go/cmd/submit/main.go` | Add `getNamespaceFromRepoName()` and `getRepoNameFromWorkflow()`, update `run()` |
| `go/cmd/submit/namespace_test.go` | Unit tests for namespace mapping and workflow parsing |

---

### Task 1: Add getNamespaceFromRepoName function

**Files:**
- Create: `go/cmd/submit/namespace_test.go`
- Modify: `go/cmd/submit/main.go`

- [ ] **Step 1: Write the failing test**

```go
package main

import "testing"

func TestGetNamespaceFromRepoName(t *testing.T) {
	tests := []struct {
		name     string
		repoName string
		want     string
	}{
		{
			name:     "ragsdk repo",
			repoName: "ascend-ragsdk",
			want:     "ragsdk",
		},
		{
			name:     "ragsdk in name",
			repoName: "other-ragsdk-project",
			want:     "ragsdk",
		},
		{
			name:     "test repo",
			repoName: "test1",
			want:     "ragsdk",
		},
		{
			name:     "test prefix",
			repoName: "test-repo",
			want:     "ragsdk",
		},
		{
			name:     "random repo",
			repoName: "random-repo",
			want:     "argo",
		},
		{
			name:     "empty repo name",
			repoName: "",
			want:     "argo",
		},
		{
			name:     "uppercase ragsdk",
			repoName: "RAGSDK-PROJECT",
			want:     "ragsdk",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getNamespaceFromRepoName(tt.repoName)
			if got != tt.want {
				t.Errorf("getNamespaceFromRepoName(%q) = %q, want %q", tt.repoName, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go/cmd/submit && go test -run TestGetNamespaceFromRepoName -v`
Expected: FAIL with "undefined: getNamespaceFromRepoName"

- [ ] **Step 3: Write minimal implementation**

Add to `go/cmd/submit/main.go` after imports:

```go
func getNamespaceFromRepoName(repoName string) string {
	repoNameLower := strings.ToLower(repoName)

	if strings.Contains(repoNameLower, "ragsdk") {
		return "ragsdk"
	}
	if strings.HasPrefix(repoNameLower, "test") {
		return "ragsdk"
	}

	return defaultNamespace
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go/cmd/submit && go test -run TestGetNamespaceFromRepoName -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add go/cmd/submit/main.go go/cmd/submit/namespace_test.go
git commit -m "feat(submit): add getNamespaceFromRepoName function with tests"
```

---

### Task 2: Add getRepoNameFromWorkflow function

**Files:**
- Modify: `go/cmd/submit/main.go`
- Modify: `go/cmd/submit/namespace_test.go`

- [ ] **Step 1: Write the failing test**

Add to `go/cmd/submit/namespace_test.go`:

```go
import (
	"os"
	"testing"
)

func TestGetRepoNameFromWorkflow(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		want     string
		wantErr  bool
	}{
		{
			name: "with jobRepositoryName label",
			yaml: `apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
  generateName: ascend-ragsdk-
  labels:
    jobRepositoryName: ascend-ragsdk
spec:
  tasks: []
`,
			want: "ascend-ragsdk",
		},
		{
			name: "without jobRepositoryName label",
			yaml: `apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
  generateName: job-
  labels:
    pipeline/run-id: test-123
spec:
  tasks: []
`,
			want: "",
		},
		{
			name: "no labels at all",
			yaml: `apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
  generateName: job-
spec:
  tasks: []
`,
			want: "",
		},
		{
			name:    "invalid yaml",
			yaml:    `invalid: yaml: content`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpFile, err := os.CreateTemp("", "workflow_*.yaml")
			if err != nil {
				t.Fatalf("failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.WriteString(tt.yaml); err != nil {
				t.Fatalf("failed to write yaml: %v", err)
			}
			tmpFile.Close()

			got, err := getRepoNameFromWorkflow(tmpFile.Name())
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("getRepoNameFromWorkflow() = %q, want %q", got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go/cmd/submit && go test -run TestGetRepoNameFromWorkflow -v`
Expected: FAIL with "undefined: getRepoNameFromWorkflow"

- [ ] **Step 3: Add yaml import**

Add to imports in `go/cmd/submit/main.go`:

```go
import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/opensourceways/codearts-workflow-image-go/cmd/kubeconfig/kubectl"
	"go.yaml.in/yaml/v3"
)
```

- [ ] **Step 4: Write minimal implementation**

Add to `go/cmd/submit/main.go` after `getNamespaceFromRepoName`:

```go
func getRepoNameFromWorkflow(workflowPath string) (string, error) {
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		return "", fmt.Errorf("读取 workflow 文件失败: %w", err)
	}

	var job struct {
		Metadata struct {
			Labels map[string]string `yaml:"labels"`
		} `yaml:"metadata"`
	}

	if err := yaml.Unmarshal(data, &job); err != nil {
		return "", fmt.Errorf("解析 workflow YAML 失败: %w", err)
	}

	return job.Metadata.Labels["jobRepositoryName"], nil
}
```

- [ ] **Step 5: Run test to verify it passes**

Run: `cd go/cmd/submit && go test -run TestGetRepoNameFromWorkflow -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add go/cmd/submit/main.go go/cmd/submit/namespace_test.go
git commit -m "feat(submit): add getRepoNameFromWorkflow function with tests"
```

---

### Task 3: Update run() function to use namespace mapping

**Files:**
- Modify: `go/cmd/submit/main.go`

- [ ] **Step 1: Update run() function**

Modify `run()` function in `go/cmd/submit/main.go` at line 103-107 (after `os.Chdir`). Insert after the secret file check:

Current code at line 108-117:
```go
	secretFile := cfg.SecretFile
	hasSecret := false
	if secretFile == "" {
		secretFile = strings.TrimSuffix(cfg.WorkflowOutput, ".yaml") + "-secret.yaml"
	}
	if _, err := os.Stat(secretFile); err == nil {
		hasSecret = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查 secret 文件失败: %w", err)
	}
```

Insert after this block (before `submitOutput` line):

```go
	repoName, err := getRepoNameFromWorkflow(cfg.WorkflowOutput)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: Could not read repo name from workflow: %v\n", err)
	} else if repoName != "" {
		mappedNamespace := getNamespaceFromRepoName(repoName)
		if mappedNamespace != cfg.Namespace {
			fmt.Printf("Mapping repo '%s' to namespace '%s'\n", repoName, mappedNamespace)
			cfg.Namespace = mappedNamespace
		}
	}
```

- [ ] **Step 2: Run all tests**

Run: `cd go/cmd/submit && go test -v`
Expected: All tests PASS

- [ ] **Step 3: Commit**

```bash
git add go/cmd/submit/main.go
git commit -m "feat(submit): integrate namespace mapping into run function"
```

---

### Task 4: Run linter and final verification

**Files:**
- None

- [ ] **Step 1: Run golangci-lint**

Run: `.ci/golangci-lint.sh`
Expected: PASS (no errors)

- [ ] **Step 2: Run all submit tests**

Run: `cd go/cmd/submit && go test -v ./...`
Expected: All tests PASS

- [ ] **Step 3: Final commit if needed**

If any fixes were made:
```bash
git add go/cmd/submit/
git commit -m "fix(submit): address linting issues"
```

---

## Self-Review

**Spec coverage:**
- ✅ Task 1: `getNamespaceFromRepoName()` - covers mapping logic
- ✅ Task 2: `getRepoNameFromWorkflow()` - covers YAML parsing
- ✅ Task 3: `run()` update - integrates mapping into submit flow
- ✅ Task 4: Linter and verification

**Placeholder scan:**
- ✅ No TBD, TODO, or vague descriptions
- ✅ All code blocks contain complete implementation
- ✅ All commands specify exact paths and expected output

**Type consistency:**
- ✅ `getNamespaceFromRepoName(string) string` - consistent throughout
- ✅ `getRepoNameFromWorkflow(string) (string, error)` - consistent throughout
- ✅ `defaultNamespace` constant used consistently