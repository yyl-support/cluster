# CP Artifact Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate three artifact test cases from Argo Workflows to multi-task Volcano Jobs with Karmada-compatible artifact extraction.

**Architecture:** Extend existing Volcano converter with inline helpers for artifact volume and copy-artifact task generation. Use Volcano's spec.volumes[].volumeClaim for dynamic PVC creation. Submit-test uses kubectl exec on copy-artifact task pod to extract artifacts.

**Tech Stack:** Go 1.21+, Volcano Job CRD (batch.volcano.sh/v1alpha1), kubectl exec tar, YAML v3

---

## File Structure

**New Files:**
- `go/cmd/converter/package/cp_artifact_manager.go` - Pure functions for artifact logic
- `go/cmd/converter/package/cp_artifact_manager_test.go` - Unit tests for manager
- `go/cmd/converter/case/newtest/test10-cp-artifacts/expected.yaml` - Volcano Job expected output
- `go/cmd/converter/case/newtest/test10-cp-artifacts/eval.sh` - Volcano Job validation
- `go/cmd/converter/case/newtest/test12-normal-workflow/expected.yaml` - Volcano Job expected output
- `go/cmd/converter/case/newtest/test12-normal-workflow/eval.sh` - Volcano Job validation
- `go/cmd/converter/case/newtest/test13-cp-artifacts-v2/expected.yaml` - Volcano Job expected output
- `go/cmd/converter/case/newtest/test13-cp-artifacts-v2/eval.sh` - Volcano Job validation

**Modified Files:**
- `go/cmd/converter/dto/volcano/volcano_job_yaml.go` - Add VolumeClaim structures
- `go/cmd/converter/package/convert_script_to_volcano.go` - Add inline helpers + multi-task logic
- `go/cmd/converter/package/convert_script_to_volcano_test.go` - Unit tests for inline helpers
- `.opencode/skills/submit-test/test-cases.json` - Add 3 test case entries

---

### Task 1: Add VolumeClaim to Volcano DTO

**Files:**
- Modify: `go/cmd/converter/dto/volcano/volcano_job_yaml.go`
- Test: `go/cmd/converter/dto/volcano/volcano_job_yaml_test.go` (new)

- [ ] **Step 1: Write failing test for VolumeClaim marshaling**

```go
package volcano

import (
	"testing"
	"go.yaml.in/yaml/v3"
)

func TestVolumeWithVolumeClaim(t *testing.T) {
	volume := Volume{
		Name:      "output",
		MountPath: "/output/artifact",
		VolumeClaim: &VolumeClaim{
			AccessModes:      []string{"ReadWriteMany"},
			StorageClassName: "sfsturbo-subpath-sc",
			Resources:        ResourceRequest{
				Requests: map[string]string{"storage": "1Gi"},
			},
		},
	}
	
	data, err := yaml.Marshal(volume)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	
	expected := `name: output
mountPath: /output/artifact
volumeClaim:
  accessModes:
    - ReadWriteMany
  storageClassName: sfsturbo-subpath-sc
  resources:
    requests:
      storage: 1Gi
`
	
	if string(data) != expected {
		t.Errorf("expected:\n%s\ngot:\n%s", expected, string(data))
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go/cmd/converter/dto/volcano && go test -v -run TestVolumeWithVolumeClaim`
Expected: FAIL with "undefined: VolumeClaim" or "unknown field VolumeClaim"

- [ ] **Step 3: Add VolumeClaim structures to DTO**

```go
type Volume struct {
	Name                  string                       `yaml:"name"`
	MountPath             string                       `yaml:"mountPath,omitempty"`
	HostPath              *HostPath                    `yaml:"hostPath,omitempty"`
	PersistentVolumeClaim *PersistentVolumeClaimVolume `yaml:"persistentVolumeClaim,omitempty"`
	VolumeClaim           *VolumeClaim                 `yaml:"volumeClaim,omitempty"`
	VolumeClaimName       string                       `yaml:"volumeClaimName,omitempty"`
}

type VolumeClaim struct {
	AccessModes      []string       `yaml:"accessModes"`
	StorageClassName string         `yaml:"storageClassName,omitempty"`
	Resources        ResourceRequest `yaml:"resources"`
	DataSource       *VolumeDataSource `yaml:"dataSource,omitempty"`
	VolumeMode       string         `yaml:"volumeMode,omitempty"`
}

type VolumeDataSource struct {
	Name string `yaml:"name"`
	Kind string `yaml:"kind"`
}

type ResourceRequest struct {
	Requests map[string]string `yaml:"requests"`
}

type JobSpec struct {
	Policies                []Policy      `yaml:"policies,omitempty"`
	Queue                   string        `yaml:"queue"`
	MaxRetry                int           `yaml:"maxRetry"`
	MinAvailable            int           `yaml:"minAvailable,omitempty"`
	SchedulerName           string        `yaml:"schedulerName,omitempty"`
	TTLSecondsAfterFinished int           `yaml:"ttlSecondsAfterFinished,omitempty"`
	Tasks                   []TaskSpec    `yaml:"tasks"`
	Volumes                 []Volume      `yaml:"volumes,omitempty"`
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go/cmd/converter/dto/volcano && go test -v -run TestVolumeWithVolumeClaim`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd go/cmd/converter/dto/volcano
git add volcano_job_yaml.go volcano_job_yaml_test.go
git commit -m "feat: add VolumeClaim support to Volcano Job DTO"
```

---

### Task 2: Create cp_artifact_manager.go

**Files:**
- Create: `go/cmd/converter/package/cp_artifact_manager.go`
- Test: `go/cmd/converter/package/cp_artifact_manager_test.go`

- [ ] **Step 1: Write failing tests for manager functions**

```go
package converter

import "testing"

func TestNeedsArtifactMultiTask(t *testing.T) {
	tests := []struct {
		name                  string
		cpArtifacts           string
		cpArtifactsTempFolder string
		want                  bool
	}{
		{"both empty", "", "", false},
		{"temp folder only", "", "/output/artifact", true},
		{"both specified", "*.txt;*.log", "/output/artifact", true},
		{"artifacts only", "*.txt", "", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsArtifactMultiTask(tt.cpArtifacts, tt.cpArtifactsTempFolder); got != tt.want {
				t.Errorf("NeedsArtifactMultiTask() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetArtifactMountPath(t *testing.T) {
	tests := []struct {
		name                  string
		cpArtifactsTempFolder string
		want                  string
	}{
		{"empty defaults to /output", "", "/output"},
		{"specified path", "/output/artifact", "/output/artifact"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetArtifactMountPath(tt.cpArtifactsTempFolder); got != tt.want {
				t.Errorf("GetArtifactMountPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetCopyArtifactTaskName(t *testing.T) {
	if got := GetCopyArtifactTaskName(); got != "copy-artifact" {
		t.Errorf("GetCopyArtifactTaskName() = %v, want %v", got, "copy-artifact")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd go/cmd/converter/package && go test -v -run TestNeeds`
Expected: FAIL with "undefined: NeedsArtifactMultiTask"

- [ ] **Step 3: Implement manager functions**

```go
package converter

const (
	ArtifactVolumeName      = "output"
	ArtifactStorageClass    = "sfsturbo-subpath-sc"
	ArtifactStorageSize     = "1Gi"
)

func NeedsArtifactMultiTask(cpArtifacts, cpArtifactsTempFolder string) bool {
	return cpArtifactsTempFolder != ""
}

func GetArtifactMountPath(cpArtifactsTempFolder string) string {
	if cpArtifactsTempFolder == "" {
		return "/output"
	}
	return cpArtifactsTempFolder
}

func GetCopyArtifactTaskName() string {
	return "copy-artifact"
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd go/cmd/converter/package && go test -v -run TestNeeds`
Expected: PASS (all 3 test functions)

- [ ] **Step 5: Commit**

```bash
cd go/cmd/converter/package
git add cp_artifact_manager.go cp_artifact_manager_test.go
git commit -m "feat: add cp_artifact_manager for artifact handling logic"
```

---

### Task 3: Add addArtifactVolume inline helper

**Files:**
- Modify: `go/cmd/converter/package/convert_script_to_volcano.go`
- Test: `go/cmd/converter/package/convert_script_to_volcano_test.go`

- [ ] **Step 1: Write failing test for addArtifactVolume**

```go
func TestAddArtifactVolume(t *testing.T) {
	job := &volcano.Job{
		Spec: volcano.JobSpec{},
	}
	container := &volcano.Container{}
	mountPath := "/output/artifact"
	
	addArtifactVolume(container, job, mountPath)
	
	// Check container has volumeMount
	if len(container.VolumeMounts) != 1 {
		t.Fatalf("expected 1 volumeMount, got %d", len(container.VolumeMounts))
	}
	if container.VolumeMounts[0].Name != ArtifactVolumeName {
		t.Errorf("expected volumeMount name %s, got %s", ArtifactVolumeName, container.VolumeMounts[0].Name)
	}
	if container.VolumeMounts[0].MountPath != mountPath {
		t.Errorf("expected mountPath %s, got %s", mountPath, container.VolumeMounts[0].MountPath)
	}
	
	// Check job has volume
	if len(job.Spec.Volumes) != 1 {
		t.Fatalf("expected 1 volume, got %d", len(job.Spec.Volumes))
	}
	if job.Spec.Volumes[0].Name != ArtifactVolumeName {
		t.Errorf("expected volume name %s, got %s", ArtifactVolumeName, job.Spec.Volumes[0].Name)
	}
	if job.Spec.Volumes[0].MountPath != mountPath {
		t.Errorf("expected volume mountPath %s, got %s", mountPath, job.Spec.Volumes[0].MountPath)
	}
	if job.Spec.Volumes[0].VolumeClaim == nil {
		t.Error("expected VolumeClaim to be set")
	}
	if job.Spec.Volumes[0].VolumeClaim.StorageClassName != ArtifactStorageClass {
		t.Errorf("expected storageClassName %s, got %s", ArtifactStorageClass, job.Spec.Volumes[0].VolumeClaim.StorageClassName)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go/cmd/converter/package && go test -v -run TestAddArtifactVolume`
Expected: FAIL with "undefined: addArtifactVolume"

- [ ] **Step 3: Implement addArtifactVolume inline helper**

Add after line 212 (after `addDatasetVolume` function):

```go
func addArtifactVolume(container *volcano.Container, job *volcano.Job, mountPath string) {
	container.VolumeMounts = append(container.VolumeMounts, volcano.VolumeMount{
		Name:      ArtifactVolumeName,
		MountPath: mountPath,
	})
	
	artifactVolume := volcano.Volume{
		Name:      ArtifactVolumeName,
		MountPath: mountPath,
		VolumeClaim: &volcano.VolumeClaim{
			AccessModes:      []string{"ReadWriteMany"},
			StorageClassName: ArtifactStorageClass,
			Resources: volcano.ResourceRequest{
				Requests: map[string]string{"storage": ArtifactStorageSize},
			},
		},
	}
	job.Spec.Volumes = append(job.Spec.Volumes, artifactVolume)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go/cmd/converter/package && go test -v -run TestAddArtifactVolume`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd go/cmd/converter/package
git add convert_script_to_volcano.go convert_script_to_volcano_test.go
git commit -m "feat: add addArtifactVolume inline helper"
```

---

### Task 4: Add createCopyArtifactTask inline helper

**Files:**
- Modify: `go/cmd/converter/package/convert_script_to_volcano.go`
- Test: `go/cmd/converter/package/convert_script_to_volcano_test.go`

- [ ] **Step 1: Write failing test for createCopyArtifactTask**

```go
func TestCreateCopyArtifactTask(t *testing.T) {
	mountPath := "/output/artifact"
	nodeSelector := map[string]string{"kubernetes.io/arch": "amd64"}
	imagePullSecrets := []volcano.LocalObjectReference{{Name: "huawei-swr-image-pull-secret-model-gy"}}
	activeDeadlineSeconds := int64(14400)
	
	task := createCopyArtifactTask(mountPath, nodeSelector, imagePullSecrets, activeDeadlineSeconds)
	
	if task.Name != GetCopyArtifactTaskName() {
		t.Errorf("expected task name %s, got %s", GetCopyArtifactTaskName(), task.Name)
	}
	if task.Replicas != 1 {
		t.Errorf("expected replicas 1, got %d", task.Replicas)
	}
	if len(task.Template.Spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(task.Template.Spec.Containers))
	}
	
	container := task.Template.Spec.Containers[0]
	if container.Image != "alpine:3.23.3" {
		t.Errorf("expected image alpine:3.23.3, got %s", container.Image)
	}
	if len(container.Command) != 1 || container.Command[0] != "sh" {
		t.Errorf("expected command [sh], got %v", container.Command)
	}
	if len(container.Args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(container.Args))
	}
	if len(container.VolumeMounts) != 1 || container.VolumeMounts[0].Name != ArtifactVolumeName {
		t.Error("expected volumeMount with ArtifactVolumeName")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd go/cmd/converter/package && go test -v -run TestCreateCopyArtifactTask`
Expected: FAIL with "undefined: createCopyArtifactTask"

- [ ] **Step 3: Implement createCopyArtifactTask inline helper**

Add after `addArtifactVolume` function:

```go
func createCopyArtifactTask(mountPath string, nodeSelector map[string]string,
                            imagePullSecrets []volcano.LocalObjectReference,
                            activeDeadlineSeconds int64) volcano.TaskSpec {
	return volcano.TaskSpec{
		Name:     GetCopyArtifactTaskName(),
		Replicas: 1,
		Template: volcano.PodTemplateSpec{
			Spec: volcano.PodSpec{
				Containers: []volcano.Container{
					{
						Name:    "ascend",
						Image:   "alpine:3.23.3",
						Command: []string{"sh"},
						Args:    []string{"trap 'exit 0' TERM; sleep infinity & wait $!"},
						Resources: volcano.Resources{
							Limits:   volcano.ResourceList{"memory": "1Gi"},
							Requests: volcano.ResourceList{"cpu": "1", "memory": "1Gi"},
						},
						VolumeMounts: []volcano.VolumeMount{
							{Name: ArtifactVolumeName, MountPath: mountPath},
						},
						Env: []volcano.EnvVar{
							{Name: "WORKSPACE", Value: "/workspace"},
						},
					},
				},
				NodeSelector:          nodeSelector,
				ImagePullSecrets:      imagePullSecrets,
				ActiveDeadlineSeconds: activeDeadlineSeconds,
				SecurityContext:       &volcano.PodSecurityContext{RunAsUser: ptrInt64(0)},
				RestartPolicy:         "Never",
			},
		},
	}
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `cd go/cmd/converter/package && go test -v -run TestCreateCopyArtifactTask`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
cd go/cmd/converter/package
git add convert_script_to_volcano.go convert_script_to_volcano_test.go
git commit -m "feat: add createCopyArtifactTask inline helper"
```

---

### Task 5: Add multi-task conditional logic to converter

**Files:**
- Modify: `go/cmd/converter/package/convert_script_to_volcano.go`

- [ ] **Step 1: Locate insertion point**

Current code at line 140-160 (after cpDataset handling):

```go
task.Template.Spec.Containers = []volcano.Container{container}

task.Template.Spec.ActiveDeadlineSeconds = int64(cpTimeoutSeconds)

task.Template.Spec.SecurityContext = &volcano.PodSecurityContext{
    RunAsUser: ptrInt64(0),
}

volcanoJob.Spec.Tasks = []volcano.TaskSpec{task}
```

- [ ] **Step 2: Add multi-task conditional logic**

Replace line 160 (`volcanoJob.Spec.Tasks = []volcano.TaskSpec{task}`):

```go
task.Template.Spec.Containers = []volcano.Container{container}

task.Template.Spec.ActiveDeadlineSeconds = int64(cpTimeoutSeconds)

task.Template.Spec.SecurityContext = &volcano.PodSecurityContext{
    RunAsUser: ptrInt64(0),
}

if NeedsArtifactMultiTask(cpArtifacts, cpArtifactsTempFolder) {
    mountPath := GetArtifactMountPath(cpArtifactsTempFolder)
    
    addArtifactVolume(&container, &volcanoJob, mountPath)
    
    copyTask := createCopyArtifactTask(
        mountPath,
        task.Template.Spec.NodeSelector,
        task.Template.Spec.ImagePullSecrets,
        task.Template.Spec.ActiveDeadlineSeconds,
    )
    
    volcanoJob.Spec.Tasks = []volcano.TaskSpec{task, copyTask}
} else {
    volcanoJob.Spec.Tasks = []volcano.TaskSpec{task}
}
```

- [ ] **Step 3: Run unit tests to verify no regression**

Run: `cd go/cmd/converter/package && go test -v ./...`
Expected: PASS (all existing tests still pass)

- [ ] **Step 4: Run E2E test for simple case (no artifacts)**

Run: `cd go/cmd/converter && go test -v -run Test_main -case test1-simple`
Expected: PASS (single-task mode unchanged)

- [ ] **Step 5: Commit**

```bash
cd go/cmd/converter/package
git add convert_script_to_volcano.go
git commit -m "feat: add multi-task artifact handling to Volcano converter"
```

---

### Task 6: Migrate test10-cp-artifacts

**Files:**
- Copy from: `go/cmd/converter/case/skip/test10-cp-artifacts/*`
- Create: `go/cmd/converter/case/newtest/test10-cp-artifacts/expected.yaml`
- Create: `go/cmd/converter/case/newtest/test10-cp-artifacts/eval.sh`

- [ ] **Step 1: Copy test case files**

```bash
cp -r go/cmd/converter/case/skip/test10-cp-artifacts go/cmd/converter/case/newtest/
```

- [ ] **Step 2: Generate expected.yaml by running converter**

```bash
cd go/cmd/converter/case/newtest/test10-cp-artifacts
source env.sh
go run ../../convertv2_to_yaml.go shell.sh workflow_templatev2.yaml > expected.yaml
```

- [ ] **Step 3: Verify expected.yaml has multi-task structure**

Check expected.yaml contains:
- `apiVersion: batch.volcano.sh/v1alpha1`
- `kind: Job`
- `spec.volumes` with VolumeClaim
- `spec.tasks` with 2 tasks: `main-script` + `copy-artifact`
- NO `volumeClaimTemplates`
- NO `entrypoint`

- [ ] **Step 4: Update eval.sh for Volcano Job**

```bash
#!/bin/bash
WORKFLOW_NAME="$1"
NAMESPACE="$2"
KUBECONFIG="$3"
WORKSPACE="${4:-.}"
CP_ARTIFACTS_TEMP_FOLDER="${5:-}"

echo "=========================================="
echo "EVAL: test10-cp-artifacts"
echo "=========================================="

echo ""
echo "[1/4] Waiting for Volcano Job completion..."

max_wait=120
interval=10
elapsed=0

while [ $elapsed -lt $max_wait ]; do
    job_status=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o jsonpath='{.status.state.phase}' 2>&1)
    
    if [[ "${job_status}" == "Completed" ]]; then
        break
    fi
    
    if [[ "${job_status}" == "Failed" ]] || [[ "${job_status}" == "Error" ]]; then
        echo "FAIL: Volcano Job ${WORKFLOW_NAME} status: ${job_status}"
        exit 1
    fi
    
    echo "  Status: ${job_status} (${elapsed}s elapsed)"
    sleep $interval
    ((elapsed += interval))
done

if [[ "${job_status}" != "Completed" ]]; then
    echo "FAIL: Volcano Job timed out after ${max_wait}s"
    exit 1
fi
echo "PASS: Volcano Job completed"

echo ""
echo "[2/4] Validating workflow CRD..."

workflow_crd=$(kubectl get job.batch.volcano.sh ${WORKFLOW_NAME} -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" -o yaml)

if echo "$workflow_crd" | grep -q "mountPath: /output/artifact"; then
    echo "PASS: mountPath found in CRD"
else
    echo "FAIL: mountPath NOT found in CRD"
    exit 1
fi

if echo "$workflow_crd" | grep -q "name: output"; then
    echo "PASS: volume name 'output' found"
else
    echo "FAIL: volume name NOT found"
    exit 1
fi

echo ""
echo "[3/4] Validating artifacts..."

# Note: Artifacts are extracted by submit-test using kubectl exec tar on copy-artifact task pod
# eval.sh just validates files exist in WORKSPACE

if [ -f "${WORKSPACE}/test.txt" ]; then
    content=$(cat "${WORKSPACE}/test.txt")
    if [ "$content" = "artifact content 1" ]; then
        echo "PASS: Artifact test.txt found with correct content"
    else
        echo "FAIL: Incorrect content in test.txt"
        exit 1
    fi
else
    echo "FAIL: test.txt NOT found in ${WORKSPACE}"
    exit 1
fi

if [ -f "${WORKSPACE}/test2.txt" ]; then
    content=$(cat "${WORKSPACE}/test2.txt")
    if [ "$content" = "artifact content 2" ]; then
        echo "PASS: Artifact test2.txt found with correct content"
    else
        echo "FAIL: Incorrect content in test2.txt"
        exit 1
    fi
else
    echo "FAIL: test2.txt NOT found in ${WORKSPACE}"
    exit 1
fi

echo ""
echo "=========================================="
echo "PASS: test10-cp-artifacts - All validations passed"
echo "=========================================="
exit 0
```

- [ ] **Step 5: Run E2E test to verify**

Run: `cd go/cmd/converter && go test -v -run Test_main -case test10-cp-artifacts`
Expected: PASS (converter generates expected.yaml, tests validate structure)

- [ ] **Step 6: Commit**

```bash
git add go/cmd/converter/case/newtest/test10-cp-artifacts/
git commit -m "feat: migrate test10-cp-artifacts to Volcano Job multi-task"
```

---

### Task 7: Migrate test12-normal-workflow

**Files:**
- Copy from: `go/cmd/converter/case/skip/test12-normal-workflow/*`
- Update: `go/cmd/converter/case/newtest/test12-normal-workflow/expected.yaml`

- [ ] **Step 1: Copy test case files**

```bash
cp -r go/cmd/converter/case/skip/test12-normal-workflow go/cmd/converter/case/newtest/
```

- [ ] **Step 2: Generate expected.yaml**

```bash
cd go/cmd/converter/case/newtest/test12-normal-workflow
source env.sh
go run ../../convertv2_to_yaml.go shell.sh workflow_templatev2.yaml > expected.yaml
```

- [ ] **Step 3: Verify expected.yaml**

Check has:
- Multi-task structure (main-script + copy-artifact)
- Inline cp command in main script: `cp -r --parents *.txt *.log /output/`
- Artifact volume at `/output` (default path)

- [ ] **Step 4: Update eval.sh for Volcano Job**

Similar to test10, but validate `*.txt` and `*.log` files extracted to WORKSPACE.

- [ ] **Step 5: Run E2E test**

Run: `cd go/cmd/converter && go test -v -run Test_main -case test12-normal-workflow`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add go/cmd/converter/case/newtest/test12-normal-workflow/
git commit -m "feat: migrate test12-normal-workflow to Volcano Job multi-task"
```

---

### Task 8: Migrate test13-cp-artifacts-v2

**Files:**
- Copy from: `go/cmd/converter/case/skip/test13-cp-artifacts-v2/*`

- [ ] **Step 1: Copy test case files**

```bash
cp -r go/cmd/converter/case/skip/test13-cp-artifacts-v2 go/cmd/converter/case/newtest/
```

- [ ] **Step 2: Generate expected.yaml**

```bash
cd go/cmd/converter/case/newtest/test13-cp-artifacts-v2
source env.sh
go run ../../convertv2_to_yaml.go shell.sh workflow_templatev2.yaml > expected.yaml
```

- [ ] **Step 3: Verify expected.yaml**

Check has:
- Multi-task structure
- Inline cp command: `cp -r --parents *.txt *.log /output/artifact/`
- Artifact volume at `/output/artifact` (custom temp folder)

- [ ] **Step 4: Update eval.sh**

Similar pattern, validate artifacts extracted correctly.

- [ ] **Step 5: Run E2E test**

Run: `cd go/cmd/converter && go test -v -run Test_main -case test13-cp-artifacts-v2`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add go/cmd/converter/case/newtest/test13-cp-artifacts-v2/
git commit -m "feat: migrate test13-cp-artifacts-v2 to Volcano Job multi-task"
```

---

### Task 9: Update test-cases.json

**Files:**
- Modify: `.opencode/skills/submit-test/test-cases.json`

- [ ] **Step 1: Add test10 entry**

```json
"test10-cp-artifacts": {
    "test-dir": "go/cmd/converter/case/newtest/test10-cp-artifacts",
    "env": "go/cmd/converter/case/newtest/test10-cp-artifacts/env.sh",
    "shell-script": "go/cmd/converter/case/newtest/test10-cp-artifacts/shell.sh",
    "expected-yaml": "go/cmd/converter/case/newtest/test10-cp-artifacts/expected.yaml",
    "expected-secret": "",
    "validation": {
        "script": "eval.sh",
        "params": {
            "WORKFLOW_NAME": "${WORKFLOW_NAME}",
            "NAMESPACE": "argo",
            "KUBECONFIG": "${KUBECONFIG}",
            "WORKSPACE": "${WORKSPACE}",
            "CP_ARTIFACTS_TEMP_FOLDER": "${CP_ARTIFACTS_TEMP_FOLDER}"
        }
    }
}
```

- [ ] **Step 2: Add test12 entry**

```json
"test12-normal-workflow": {
    "test-dir": "go/cmd/converter/case/newtest/test12-normal-workflow",
    "env": "go/cmd/converter/case/newtest/test12-normal-workflow/env.sh",
    "shell-script": "go/cmd/converter/case/newtest/test12-normal-workflow/shell.sh",
    "workflow-template-v2": "go/cmd/converter/case/newtest/test12-normal-workflow/workflow_templatev2.yaml",
    "expected-yaml": "go/cmd/converter/case/newtest/test12-normal-workflow/expected.yaml",
    "expected-secret": "",
    "validation": {
        "script": "eval.sh",
        "params": {
            "WORKFLOW_NAME": "${WORKFLOW_NAME}",
            "NAMESPACE": "argo",
            "KUBECONFIG": "${KUBECONFIG}",
            "WORKSPACE": "${WORKSPACE}"
        }
    }
}
```

- [ ] **Step 3: Add test13 entry**

```json
"test13-cp-artifacts-v2": {
    "test-dir": "go/cmd/converter/case/newtest/test13-cp-artifacts-v2",
    "env": "go/cmd/converter/case/newtest/test13-cp-artifacts-v2/env.sh",
    "shell-script": "go/cmd/converter/case/newtest/test13-cp-artifacts-v2/shell.sh",
    "workflow-template-v2": "go/cmd/converter/case/newtest/test13-cp-artifacts-v2/workflow_templatev2.yaml",
    "expected-yaml": "go/cmd/converter/case/newtest/test13-cp-artifacts-v2/expected.yaml",
    "expected-secret": "",
    "validation": {
        "script": "eval.sh",
        "params": {
            "WORKFLOW_NAME": "${WORKFLOW_NAME}",
            "NAMESPACE": "argo",
            "KUBECONFIG": "${KUBECONFIG}",
            "WORKSPACE": "${WORKSPACE}",
            "CP_ARTIFACTS_TEMP_FOLDER": "${CP_ARTIFACTS_TEMP_FOLDER}"
        }
    }
}
```

- [ ] **Step 4: Validate JSON format**

Run: `jq . .opencode/skills/submit-test/test-cases.json > /dev/null`
Expected: No error (valid JSON)

- [ ] **Step 5: Commit**

```bash
git add .opencode/skills/submit-test/test-cases.json
git commit -m "feat: add test10, test12, test13 to test-cases.json"
```

---

### Task 10: Run full E2E test suite

**Files:**
- None (validation task)

- [ ] **Step 1: Run unit tests**

Run: `cd go/cmd/converter && go test -cover ./...`
Expected: All PASS, coverage > 90%

- [ ] **Step 2: Run all new test cases**

Run: `cd go/cmd/converter && go test -v -run Test_main -case "test10-cp-artifacts,test12-normal-workflow,test13-cp-artifacts-v2"`
Expected: All 3 PASS

- [ ] **Step 3: Run existing tests to verify no regression**

Run: `cd go/cmd/converter && go test -v -run Test_main`
Expected: All existing tests still PASS

- [ ] **Step 4: Run CI lint checks**

Run: `.ci/typos.sh && .ci/golangci-lint.sh`
Expected: No errors

---

### Task 11: Update submit-test to support Volcano Job artifact extraction

**Files:**
- Modify: `.opencode/skills/submit-test/scripts/lib/argo.sh`

- [ ] **Step 1: Add Volcano Job artifact extraction function**

```bash
extract_volcano_artifacts() {
    local workflow_name="$1"
    local namespace="$2"
    local kubeconfig="$3"
    local workspace="$4"
    local mount_path="$5"
    
    # Find copy-artifact task pod
    local pod_name
    pod_name=$(kubectl get pods -n "$namespace" --kubeconfig "$kubeconfig" \
        -l volcano.sh/job-name="$workflow_name",volcano.sh/task-name=copy-artifact \
        -o jsonpath='{.items[0].metadata.name}' 2>&1)
    
    if [[ -z "$pod_name" ]]; then
        log_error "copy-artifact task pod not found"
        return 1
    fi
    
    log_info "Extracting artifacts from pod: $pod_name"
    
    # Stream artifacts with path stripping
    kubectl exec "$pod_name" -n "$namespace" --kubeconfig "$kubeconfig" \
        -- tar czf - -C "$mount_path" . | tar xzf - -C "$workspace"
    
    if [[ $? -ne 0 ]]; then
        log_error "Artifact extraction failed"
        return 1
    fi
    
    log_success "Artifacts extracted to $workspace"
    return 0
}
```

- [ ] **Step 2: Update run_eval to detect Volcano Jobs**

Add logic to check job type and call appropriate extraction:
- If job is Volcano Job (batch.volcano.sh/v1alpha1) → use extract_volcano_artifacts
- If workflow is Argo Workflow → use existing copy pod mechanism

- [ ] **Step 3: Test extraction manually**

```bash
kubectl --kubeconfig ~/.kube/006.yaml get job.batch.volcano.sh
kubectl --kubeconfig ~/.kube/006.yaml get pods -l volcano.sh/task-name=copy-artifact
```

- [ ] **Step 4: Commit**

```bash
git add .opencode/skills/submit-test/scripts/lib/argo.sh
git commit -m "feat: add Volcano Job artifact extraction to submit-test"
```

---

## Self-Review Checklist

After completing all tasks, run this verification:

1. **Spec coverage:** Each design requirement implemented? ✅
2. **No placeholders:** All code complete, no TBD/TODO? ✅
3. **Type consistency:** Function signatures match across tasks? ✅
4. **Test coverage:** Unit + E2E tests for all new functions? ✅
5. **Backward compatibility:** Existing tests still pass? ✅
6. **Lint checks:** No typos, no golangci-lint errors? ✅

---

## Success Criteria

- ✅ All 3 test cases generate multi-task Volcano Jobs
- ✅ Converter generates expected.yaml with VolumeClaim
- ✅ Unit test coverage > 90% for new functions
- ✅ E2E tests pass for all migrated cases
- ✅ Existing test1-28 continue passing
- ✅ No lint errors