# CP_shm Parameter Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add CP_shm parameter support to configure /dev/shm emptyDir volume for Volcano Jobs

**Architecture:** Add EmptyDir struct to Volume, create shm_manager.go following existing manager pattern, parse and normalize size from CP_shm env var

**Tech Stack:** Go, YAML, Volcano Job CRD, existing manager pattern

---

## File Structure

**Create:**
- `go/cmd/converter/package/shm_manager.go` - AddShmVolume manager logic
- `go/cmd/converter/package/shm_manager_test.go` - Unit tests for shm_manager
- `go/cmd/converter/case/newtest/test-shm/env.sh` - Test case environment
- `go/cmd/converter/case/newtest/test-shm/shell.sh` - Test case script
- `go/cmd/converter/case/newtest/test-shm/expected.yaml` - Expected Volcano Job YAML

**Modify:**
- `go/cmd/converter/dto/volcano/volcano_job_yaml.go` - Add EmptyDir struct to Volume
- `go/cmd/converter/package/cp_config.go` - Parse CP_shm, add normalizeShmSize function
- `go/cmd/converter/package/convert_script_to_volcano.go` - Pass cpShm parameter, call AddShmVolume
- `go/cmd/converter/package/cp_config_test.go` - Test normalizeShmSize function

---

### Task 1: Add EmptyDir Struct to Volume

**Files:**
- Modify: `go/cmd/converter/dto/volcano/volcano_job_yaml.go:107-111`

- [ ] **Step 1: Add EmptyDir struct definition**

Add after PersistentVolumeClaimVolume struct (around line 121):

```go
type EmptyDir struct {
	Medium    string `yaml:"medium,omitempty"`
	SizeLimit string `yaml:"sizeLimit,omitempty"`
}
```

- [ ] **Step 2: Update Volume struct to include EmptyDir**

Modify Volume struct (lines 107-111):

```go
type Volume struct {
	Name                  string                       `yaml:"name"`
	HostPath              *HostPath                    `yaml:"hostPath,omitempty"`
	PersistentVolumeClaim *PersistentVolumeClaimVolume `yaml:"persistentVolumeClaim,omitempty"`
	EmptyDir              *EmptyDir                    `yaml:"emptyDir,omitempty"`
}
```

- [ ] **Step 3: Verify struct compiles**

Run: `cd go/cmd/converter/dto/volcano && go build`
Expected: No errors

- [ ] **Step 4: Commit**

```bash
git add go/cmd/converter/dto/volcano/volcano_job_yaml.go
git commit -m "feat: add EmptyDir struct to Volume for shm support"
```

---

### Task 2: Add normalizeShmSize Function to cp_config

**Files:**
- Modify: `go/cmd/converter/package/cp_config.go:10-44`
- Modify: `go/cmd/converter/package/cp_config_test.go` (create if missing or append)

- [ ] **Step 1: Write failing test for normalizeShmSize**

Add to `go/cmd/converter/package/cp_config_test.go`:

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go/cmd/converter/package && go test -run TestNormalizeShmSize -v`
Expected: FAIL with "undefined: normalizeShmSize"

- [ ] **Step 3: Implement normalizeShmSize function**

Add to `go/cmd/converter/package/cp_config.go` after line 53:

```go
func normalizeShmSize(size string) string {
	if size == "" {
		return ""
	}
	if strings.HasSuffix(size, "i") {
		return size
	}
	return size + "i"
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go/cmd/converter/package && go test -run TestNormalizeShmSize -v`
Expected: PASS with all test cases passing

- [ ] **Step 5: Commit**

```bash
git add go/cmd/converter/package/cp_config.go go/cmd/converter/package/cp_config_test.go
git commit -m "feat: add normalizeShmSize function for CP_shm parameter"
```

---

### Task 3: Parse CP_shm in GetCPConfig

**Files:**
- Modify: `go/cmd/converter/package/cp_config.go:10-44`

- [ ] **Step 1: Update GetCPConfig signature to return cpShm**

Modify `GetCPConfig` function signature (line 10):

```go
func GetCPConfig() (runsOn, dockerImage, pipelineRunID, mergeID, repoURL, targetBranch, cpArtifacts, cpArtifactsTempFolder, cpDataset, cpImageProxy, cpShm string, cpTimeoutSeconds int) {
```

- [ ] **Step 2: Parse CP_shm environment variable**

Add after line 24 (after cpImageProxy parsing):

```go
cpShm = filterCPEnv("CP_shm")
```

- [ ] **Step 3: Update return statement**

Verify return statement includes cpShm (should already be there after modifying signature).

- [ ] **Step 4: Verify compiles**

Run: `cd go/cmd/converter/package && go build`
Expected: No errors

- [ ] **Step 5: Commit**

```bash
git add go/cmd/converter/package/cp_config.go
git commit -m "feat: parse CP_shm environment variable in GetCPConfig"
```

---

### Task 4: Create shm_manager.go

**Files:**
- Create: `go/cmd/converter/package/shm_manager.go`
- Create: `go/cmd/converter/package/shm_manager_test.go`

- [ ] **Step 1: Write failing test for AddShmVolume**

Create `go/cmd/converter/package/shm_manager_test.go`:

```go
package converter

import (
	"testing"

	"github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"
)

func TestAddShmVolume(t *testing.T) {
	tests := []struct {
		name   string
		cpShm  string
		expect bool // expect volume to be added
	}{
		{
			name:   "empty CP_shm - no volume added",
			cpShm:  "",
			expect: false,
		},
		{
			name:   "CP_shm=8G - volume added",
			cpShm:  "8G",
			expect: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			job := volcano.Job{
				Spec: volcano.JobSpec{
					Tasks: []volcano.TaskSpec{
						{
							Template: volcano.PodTemplateSpec{
								Spec: volcano.PodSpec{
									Containers: []volcano.Container{
										{Name: "test"},
									},
								},
							},
						},
					},
				},
			}

			AddShmVolume(&job, tt.cpShm)

			if tt.expect {
				if len(job.Spec.Tasks[0].Template.Spec.Volumes) == 0 {
					t.Error("expected volume to be added, but none found")
				}
				vol := job.Spec.Tasks[0].Template.Spec.Volumes[0]
				if vol.Name != "shm" {
					t.Errorf("expected volume name 'shm', got %s", vol.Name)
				}
				if vol.EmptyDir == nil {
					t.Error("expected EmptyDir volume")
				}
				if vol.EmptyDir.Medium != "Memory" {
					t.Errorf("expected Medium 'Memory', got %s", vol.EmptyDir.Medium)
				}
				if vol.EmptyDir.SizeLimit != "8Gi" {
					t.Errorf("expected SizeLimit '8Gi', got %s", vol.EmptyDir.SizeLimit)
				}

				container := job.Spec.Tasks[0].Template.Spec.Containers[0]
				if len(container.VolumeMounts) == 0 {
					t.Error("expected volumeMount to be added")
				}
				mount := container.VolumeMounts[0]
				if mount.Name != "shm" {
					t.Errorf("expected mount name 'shm', got %s", mount.Name)
				}
				if mount.MountPath != "/dev/shm" {
					t.Errorf("expected mountPath '/dev/shm', got %s", mount.MountPath)
				}
			} else {
				if len(job.Spec.Tasks[0].Template.Spec.Volumes) > 0 {
					t.Errorf("expected no volume, but got %d volumes", len(job.Spec.Tasks[0].Template.Spec.Volumes))
				}
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go/cmd/converter/package && go test -run TestAddShmVolume -v`
Expected: FAIL with "undefined: AddShmVolume"

- [ ] **Step 3: Implement AddShmVolume function**

Create `go/cmd/converter/package/shm_manager.go`:

```go
package converter

import (
	"github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"
)

func AddShmVolume(job *volcano.Job, cpShm string) {
	if cpShm == "" {
		return
	}

	sizeLimit := normalizeShmSize(cpShm)

	task := job.Spec.Tasks[0]
	task.Template.Spec.Volumes = append(task.Template.Spec.Volumes, volcano.Volume{
		Name: "shm",
		EmptyDir: &volcano.EmptyDir{
			Medium:    "Memory",
			SizeLimit: sizeLimit,
		},
	})

	container := task.Template.Spec.Containers[0]
	container.VolumeMounts = append(container.VolumeMounts, volcano.VolumeMount{
		Name:      "shm",
		MountPath: "/dev/shm",
	})
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go/cmd/converter/package && go test -run TestAddShmVolume -v`
Expected: PASS with both test cases passing

- [ ] **Step 5: Commit**

```bash
git add go/cmd/converter/package/shm_manager.go go/cmd/converter/package/shm_manager_test.go
git commit -m "feat: create shm_manager with AddShmVolume function"
```

---

### Task 5: Integrate AddShmVolume in ConvertScriptToVolcano

**Files:**
- Modify: `go/cmd/converter/package/convert_script_to_volcano.go:20-34`

- [ ] **Step 1: Update ConvertScriptToVolcano signature**

Modify function signature to accept cpShm parameter (line 20):

```go
func ConvertScriptToVolcano(
	scriptContent string,
	runsOn string,
	dockerImage string,
	envVars map[string]string,
	pipelineRunID string,
	mergeID string,
	repoURL string,
	targetBranch string,
	uniqueID string,
	yamlTemplatePath string,
	cpDataset string,
	cpArtifacts string,
	cpArtifactsTempFolder string,
	cpTimeoutSeconds int,
	cpShm string, // NEW
) VolcanoConversionResult {
```

- [ ] **Step 2: Call AddShmVolume in conversion logic**

Add after line 135 (after dataset handling):

```go
AddShmVolume(&volcanoJob, cpShm)
```

- [ ] **Step 3: Verify compiles**

Run: `cd go/cmd/converter && go build`
Expected: No errors (will need to update callers next)

- [ ] **Step 4: Commit**

```bash
git add go/cmd/converter/package/convert_script_to_volcano.go
git commit -m "feat: integrate AddShmVolume in ConvertScriptToVolcano"
```

---

### Task 6: Update ConvertScriptToVolcano Caller

**Files:**
- Modify: `go/cmd/converter/main.go` (find call to ConvertScriptToVolcano)

- [ ] **Step 1: Find and update caller**

Search for ConvertScriptToVolcano call in `go/cmd/converter/main.go` and update to pass cpShm:

```go
result := ConvertScriptToVolcano(
	scriptContent,
	runsOn,
	dockerImage,
	envVars,
	pipelineRunID,
	mergeID,
	repoURL,
	targetBranch,
	uniqueID,
	yamlTemplatePath,
	cpDataset,
	cpArtifacts,
	cpArtifactsTempFolder,
	cpTimeoutSeconds,
	cpShm, // NEW
)
```

- [ ] **Step 2: Verify compiles**

Run: `cd go/cmd/converter && go build`
Expected: No errors

- [ ] **Step 3: Commit**

```bash
git add go/cmd/converter/main.go
git commit -m "feat: pass cpShm parameter to ConvertScriptToVolcano"
```

---

### Task 7: Create Test Case - env.sh

**Files:**
- Create: `go/cmd/converter/case/newtest/test-shm/env.sh`

- [ ] **Step 1: Create env.sh file**

Create `go/cmd/converter/case/newtest/test-shm/env.sh`:

```bash
export CP_runs_on="amd64-cpu-1-mem-1G"
export CP_docker_image="swr.cn-southwest-2.myhuaweicloud.com/base_image/ascend-ci/cann:8.2.rc1-910b-ubuntu22.04-py3.11"
export CP_pipeline_run_id="test-shm-123"
export CP_merge_id="25"
export CP_repo_url="https://github.com/testorg/testrepo-test25.git"
export CP_shm="8G"
```

- [ ] **Step 2: Verify file exists**

Run: `ls -la go/cmd/converter/case/newtest/test-shm/env.sh`
Expected: File exists with correct permissions

- [ ] **Step 3: Commit**

```bash
git add go/cmd/converter/case/newtest/test-shm/env.sh
git commit -m "test: add env.sh for test-shm case"
```

---

### Task 8: Create Test Case - shell.sh

**Files:**
- Create: `go/cmd/converter/case/newtest/test-shm/shell.sh`

- [ ] **Step 1: Create shell.sh file**

Create `go/cmd/converter/case/newtest/test-shm/shell.sh`:

```bash
#!/bin/sh
df -h /dev/shm
echo "shm size test complete"
```

- [ ] **Step 2: Verify file exists**

Run: `ls -la go/cmd/converter/case/newtest/test-shm/shell.sh`
Expected: File exists with executable permissions

- [ ] **Step 3: Commit**

```bash
git add go/cmd/converter/case/newtest/test-shm/shell.sh
git commit -m "test: add shell.sh for test-shm case"
```

---

### Task 9: Generate expected.yaml for Test Case

**Files:**
- Create: `go/cmd/converter/case/newtest/test-shm/expected.yaml`

- [ ] **Step 1: Run converter to generate YAML**

Run: `cd go/cmd/converter && go test -v -run Test_main/test-shm`
Expected: PASS, generates expected.yaml

- [ ] **Step 2: Verify expected.yaml contains shm volume**

Run: `grep -A 5 "name: shm" go/cmd/converter/case/newtest/test-shm/expected.yaml`
Expected: Shows emptyDir volume with Medium: Memory and SizeLimit: 8Gi

- [ ] **Step 3: Verify expected.yaml contains shm mount**

Run: `grep "mountPath: /dev/shm" go/cmd/converter/case/newtest/test-shm/expected.yaml`
Expected: Shows volumeMount with mountPath: /dev/shm

- [ ] **Step 4: Commit**

```bash
git add go/cmd/converter/case/newtest/test-shm/expected.yaml
git commit -m "test: add expected.yaml for test-shm case"
```

---

### Task 10: Run Full Test Suite

**Files:**
- None (validation only)

- [ ] **Step 1: Run all unit tests**

Run: `cd go/cmd/converter && go test -cover ./...`
Expected: All tests pass with >90% coverage on new code

- [ ] **Step 2: Run E2E tests**

Run: `cd go/cmd/converter && go test -v -run Test_main`
Expected: All E2E tests pass including test-shm

- [ ] **Step 3: Verify final commit**

Run: `git log --oneline -10`
Expected: See all commits from Tasks 1-9

---

## Self-Review Checklist

After writing this plan, I've checked:

1. **Spec coverage:** All sections covered - EmptyDir struct, cp_config parsing, shm_manager, integration, test case
2. **Placeholder scan:** No TBDs, TODOs, or vague instructions
3. **Type consistency:** All struct names, function signatures consistent throughout tasks

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-06-09-cp-shm-parameter.md`.

**Two execution options:**

1. **Subagent-Driven (recommended)** - Fresh subagent per task, review between tasks, fast iteration
2. **Inline Execution** - Execute tasks in this session, batch execution with checkpoints

**Which approach do you prefer?**