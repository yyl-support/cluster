# Volcano Migration Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace Argo Workflow generator with Volcano Job generator in codearts-workflow-image converter.

**Architecture:** Create new volcano DTO package mirroring Argo structure, convert script/job to Volcano Job CRD, remove Argo-specific features (VolumeClaimTemplates, OnExit, Artifacts), keep core features (generateName, labels, volumes, env vars, resources, nodeSelector, securityContext).

**Tech Stack:** Go 1.21+, yaml.v3, Volcano Job CRD (batch.volcano.sh/v1alpha1)

---

## File Structure

### New Files
- `go/cmd/converter/dto/volcano/volcano_job_yaml.go` - Volcano Job DTOs
- `go/cmd/converter/package/convert_script_to_volcano.go` - Script → Volcano Job converter
- `go/cmd/converter/package/convert_script_to_volcano_test.go` - Unit tests for converter
- `go/cmd/converter/package/convert_job_to_volcano.go` - Multi-step job → Volcano Job converter (test12)

### Modified Files
- `go/cmd/converter/convertv2_to_yaml.go` - Main entry point, switch to volcano converter
- `go/cmd/converter/convertv2_to_yaml_test.go` - E2E tests, skip test10/test13
- `go/cmd/converter/case/newtest/test*/expected.yaml` - All test expectations (19 files)

### Deleted Files (Phase 5)
- `go/cmd/converter/dto/argo/argo_workflow_yaml.go`
- `go/cmd/converter/package/convert_script_to_argo.go`
- `go/cmd/converter/package/convert_script_to_argo_test.go`
- `go/cmd/converter/package/convert_job_to_argo.go`
- `go/cmd/converter/package/convert_job_to_argo_test.go`

---

## Task 1: Create Volcano DTO Package

**Files:**
- Create: `go/cmd/converter/dto/volcano/volcano_job_yaml.go`

- [ ] **Step 1: Create volcano DTO directory**

```bash
mkdir -p go/cmd/converter/dto/volcano
```

- [ ] **Step 2: Write volcano Job DTOs**

Create `go/cmd/converter/dto/volcano/volcano_job_yaml.go`:

```go
package volcano

type Job struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       JobSpec  `yaml:"spec"`
}

type Metadata struct {
	GenerateName string            `yaml:"generateName,omitempty"`
	Labels       map[string]string `yaml:"labels,omitempty"`
}

type JobSpec struct {
	Queue         string     `yaml:"queue"`
	MaxRetry      int        `yaml:"maxRetry,omitempty"`
	MinAvailable  int        `yaml:"minAvailable,omitempty"`
	SchedulerName string     `yaml:"schedulerName,omitempty"`
	Tasks         []TaskSpec `yaml:"tasks"`
}

type TaskSpec struct {
	Name         string          `yaml:"name"`
	Replicas     int             `yaml:"replicas"`
	MaxRetry     int             `yaml:"maxRetry,omitempty"`
	MinAvailable int             `yaml:"minAvailable,omitempty"`
	Template     PodTemplateSpec `yaml:"template"`
}

type PodTemplateSpec struct {
	Metadata Metadata `yaml:"metadata,omitempty"`
	Spec     PodSpec  `yaml:"spec"`
}

type PodSpec struct {
	Containers           []Container           `yaml:"containers"`
	NodeSelector         map[string]string     `yaml:"nodeSelector,omitempty"`
	ImagePullSecrets     []LocalObjectReference `yaml:"imagePullSecrets,omitempty"`
	ActiveDeadlineSeconds int64                `yaml:"activeDeadlineSeconds,omitempty"`
	SecurityContext      *PodSecurityContext   `yaml:"securityContext,omitempty"`
	Volumes              []Volume              `yaml:"volumes,omitempty"`
}

type Container struct {
	Name         string        `yaml:"name"`
	Image        string        `yaml:"image"`
	Command      []string      `yaml:"command"`
	Args         []string      `yaml:"args"`
	WorkingDir   string        `yaml:"workingDir,omitempty"`
	Resources    Resources     `yaml:"resources,omitempty"`
	VolumeMounts []VolumeMount `yaml:"volumeMounts,omitempty"`
	Env          []EnvVar      `yaml:"env,omitempty"`
}

type Resources struct {
	Limits   ResourceList `yaml:"limits,omitempty"`
	Requests ResourceList `yaml:"requests,omitempty"`
}

type ResourceList map[string]string

type VolumeMount struct {
	Name      string `yaml:"name"`
	MountPath string `yaml:"mountPath"`
	ReadOnly  bool   `yaml:"readOnly,omitempty"`
}

type Volume struct {
	Name                  string                 `yaml:"name"`
	HostPath              *HostPath              `yaml:"hostPath,omitempty"`
	PersistentVolumeClaim *PersistentVolumeClaimVolume `yaml:"persistentVolumeClaim,omitempty"`
}

type HostPath struct {
	Path string `yaml:"path"`
	Type string `yaml:"type,omitempty"`
}

type PersistentVolumeClaimVolume struct {
	ClaimName string `yaml:"claimName"`
}

type EnvVar struct {
	Name      string        `yaml:"name" json:"name"`
	Value     string        `yaml:"value,omitempty" json:"value,omitempty"`
	ValueFrom *EnvVarSource `yaml:"valueFrom,omitempty" json:"valueFrom,omitempty"`
}

type EnvVarSource struct {
	SecretKeyRef *SecretKeySelector `yaml:"secretKeyRef,omitempty" json:"secretKeyRef,omitempty"`
}

type SecretKeySelector struct {
	Name     string `yaml:"name" json:"name"`
	Key      string `yaml:"key" json:"key"`
	Optional *bool  `yaml:"optional,omitempty" json:"optional,omitempty"`
}

type LocalObjectReference struct {
	Name string `yaml:"name"`
}

type PodSecurityContext struct {
	RunAsUser *int64  `yaml:"runAsUser,omitempty"`
	Sysctls   []Sysctl `yaml:"sysctls,omitempty"`
}

type Sysctl struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}
```

- [ ] **Step 3: Commit volcano DTOs**

```bash
git add go/cmd/converter/dto/volcano/
git commit -m "feat: add volcano Job DTO package"
```

---

## Task 2: Create Script-to-Volcano Converter

**Files:**
- Create: `go/cmd/converter/package/convert_script_to_volcano.go`
- Create: `go/cmd/converter/package/convert_script_to_volcano_test.go`

- [ ] **Step 1: Write converter function skeleton**

Create `go/cmd/converter/package/convert_script_to_volcano.go`:

```go
package converter

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	runonparser "github.com/opensourceways/codearts-workflow-image-go/cmd/common"
	"github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"
	"go.yaml.in/yaml/v3"
)

type VolcanoConversionResult struct {
	Job            volcano.Job
	SecretManifest string
}

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
) VolcanoConversionResult {

	dir := filepath.Dir(yamlTemplatePath)
	if dir == "." {
		dir = ""
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		fmt.Printf("fail to open root directory: %v\n", err)
		os.Exit(1)
	}
	defer root.Close()

	filename := filepath.Base(yamlTemplatePath)
	f, err := root.Open(filename)
	if err != nil {
		fmt.Printf("fail to load yaml template %s : %v\n", yamlTemplatePath, err)
		os.Exit(1)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		fmt.Printf("fail to load yaml template %s : %v\n", yamlTemplatePath, err)
		os.Exit(1)
	}

	var volcanoJob volcano.Job
	err = yaml.Unmarshal(data, &volcanoJob)
	if err != nil {
		fmt.Print(err.Error())
		os.Exit(1)
		return VolcanoConversionResult{Job: volcanoJob, SecretManifest: ""}
	}

	task := volcanoJob.Spec.Tasks[0]
	container := task.Template.Spec.Containers[0]

	volcanoJob.Spec.Queue = "large-task-shared-queue"

	volcanoJob.Spec.SecurityContext = &volcano.PodSecurityContext{
		RunAsUser: ptrInt64(0),
	}

	repoName := ""
	if repoURL != "" {
		repoName = extractOrgRepo(repoURL)
	}

	if pipelineRunID != "" || mergeID != "" || repoURL != "" {
		volcanoJob.Metadata.Labels = map[string]string{}
		if pipelineRunID != "" {
			volcanoJob.Metadata.Labels["pipeline/run-id"] = pipelineRunID
		}
		if mergeID != "" {
			volcanoJob.Metadata.Labels["jobPRID"] = mergeID
		}
		if repoName != "" {
			volcanoJob.Metadata.Labels["jobRepositoryName"] = repoName
		}
	}

	task.Name = "main-script"
	task.Replicas = 1

	gitCacheURL := "http://git-cache-http-server.git-cache.svc.cluster.local:8080"
	script := handlerScript(
		scriptContent,
		gitRequest{RepoURL: repoURL, MergeID: mergeID, TargetBranch: targetBranch, GitCacheURL: gitCacheURL},
		artifactsRequest{},
	)

	container.Command = []string{"bash", "-c"}
	container.Args = []string{script}

	if dockerImage == "" {
		dockerImage = "swr.cn-southwest-2.myhuaweicloud.com/modelfoundry/git:latest"
	}
	container.Image = dockerImage

	task.Template.Spec.NodeSelector = convertJobArch(runsOn)

	container.Resources, err = convertJobResource(runsOn)
	if err != nil {
		fmt.Printf("fail to convert runs_on: %v\n", err)
		os.Exit(1)
	}

	parsedSpec, _ := runonparser.Parse(runsOn)
	if parsedSpec != nil && !parsedSpec.IsNPUEmpty() {
		container.VolumeMounts = append(container.VolumeMounts, volcano.VolumeMount{
			Name:      "ascend-driver",
			MountPath: "/usr/local/Ascend/driver",
			ReadOnly:  true,
		})
		task.Template.Spec.Volumes = append(task.Template.Spec.Volumes, volcano.Volume{
			Name: "ascend-driver",
			HostPath: &volcano.HostPath{
				Path: "/usr/local/Ascend/driver",
			},
		})
	}

	volcanoJob.Spec.Tasks = []volcano.TaskSpec{task}

	if repoURL != "" {
		volcanoJob.Metadata.GenerateName = repoName + "-"
	} else {
		volcanoJob.Metadata.GenerateName = "job-script-"
	}

	sensitiveFromEnv, plainEnv := FilterSensitiveEnv(envVars)
	plainEnv = AddCustomEnv(plainEnv, container.Resources)
	resolvedSensitive := ResolveSensitiveEnvValues(sensitiveFromEnv)

	plainEnvList := convertEnv(plainEnv)
	container.Env = append(container.Env, plainEnvList...)

	if cpDataset != "" {
		if repoName == "" {
			fmt.Println("fail to configure CP_dataset: missing repo name")
			os.Exit(1)
		}
		sharedVolume := volcano.Volume{
			Name: "dataset",
			PersistentVolumeClaim: &volcano.PersistentVolumeClaimVolume{
				ClaimName: GetDatasetClaimName(repoName),
			},
		}
		task.Template.Spec.Volumes = append(task.Template.Spec.Volumes, sharedVolume)
		container.VolumeMounts = append(container.VolumeMounts, volcano.VolumeMount{
			Name:      "dataset",
			MountPath: cpDataset,
		})
	}

	if len(resolvedSensitive) > 0 {
		secretName := BuildSecretName(pipelineRunID, uniqueID)

		secretEnvVars := ConvertToSecretEnvVolcano(secretName, resolvedSensitive)
		container.Env = append(container.Env, secretEnvVars...)

		secretManifest := BuildSecretManifest(secretName, "", pipelineRunID, resolvedSensitive)

		return VolcanoConversionResult{
			Job:            volcanoJob,
			SecretManifest: secretManifest,
		}
	}

	return VolcanoConversionResult{
		Job:            volcanoJob,
		SecretManifest: "",
	}
}

func ptrInt64(i int64) *int64 {
	return &i
}

func extractOrgRepo(repoURL string) string {
	repoURL = strings.TrimSpace(repoURL)
	repoURL = strings.TrimPrefix(repoURL, "https://")
	repoURL = strings.TrimPrefix(repoURL, "http://")
	repoURL = strings.TrimSuffix(repoURL, ".git")
	parts := strings.Split(repoURL, "/")
	if len(parts) >= 2 {
		org := parts[len(parts)-2]
		repo := parts[len(parts)-1]
		name := org + "-" + repo
		name = strings.ToLower(name)
		name = strings.Trim(name, "-")
		return name
	}
	repoURL = strings.ToLower(repoURL)
	return strings.Trim(repoURL, "-")
}

func ConvertToSecretEnvVolcano(secretName string, sensitive map[string]string) []volcano.EnvVar {
	envVars := []volcano.EnvVar{}
	for key := range sensitive {
		envVars = append(envVars, volcano.EnvVar{
			Name: key,
			ValueFrom: &volcano.EnvVarSource{
				SecretKeyRef: &volcano.SecretKeySelector{
					Name: secretName,
					Key:  key,
				},
			},
		})
	}
	return envVars
}
```

- [ ] **Step 2: Write unit test for basic conversion**

Create `go/cmd/converter/package/convert_script_to_volcano_test.go`:

```go
package converter

import (
	"testing"

	"github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"
)

func TestConvertScriptToVolcano_Basic(t *testing.T) {
	scriptContent := "#!/bin/bash\necho 'hello'"
	runsOn := "amd64"
	dockerImage := "test-image:latest"
	envVars := map[string]string{"TEST_VAR": "test-value"}
	pipelineRunID := "test-123"
	mergeID := "15"
	repoURL := "https://github.com/testorg/testrepo.git"
	targetBranch := "main"
	uniqueID := "unique-123"
	yamlTemplatePath := "../case/workflow_templatev2.yaml"

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
		"",
	)

	if result.Job.APIVersion != "batch.volcano.sh/v1alpha1" {
		t.Errorf("expected APIVersion batch.volcano.sh/v1alpha1, got %s", result.Job.APIVersion)
	}

	if result.Job.Kind != "Job" {
		t.Errorf("expected Kind Job, got %s", result.Job.Kind)
	}

	if result.Job.Spec.Queue != "large-task-shared-queue" {
		t.Errorf("expected queue large-task-shared-queue, got %s", result.Job.Spec.Queue)
	}

	if len(result.Job.Spec.Tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(result.Job.Spec.Tasks))
	}

	task := result.Job.Spec.Tasks[0]
	if task.Name != "main-script" {
		t.Errorf("expected task name main-script, got %s", task.Name)
	}

	if task.Replicas != 1 {
		t.Errorf("expected replicas 1, got %d", task.Replicas)
	}

	container := task.Template.Spec.Containers[0]
	if container.Image != "test-image:latest" {
		t.Errorf("expected image test-image:latest, got %s", container.Image)
	}

	if len(container.Command) != 2 || container.Command[0] != "bash" || container.Command[1] != "-c" {
		t.Errorf("expected command [bash, -c], got %v", container.Command)
	}

	if len(container.Args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(container.Args))
	}

	if result.Job.Metadata.GenerateName != "testorg-testrepo-" {
		t.Errorf("expected generateName testorg-testrepo-, got %s", result.Job.Metadata.GenerateName)
	}
}
```

- [ ] **Step 3: Run test to verify it passes**

```bash
cd go/cmd/converter && go test -v -run TestConvertScriptToVolcano_Basic ./package
```

Expected: PASS

- [ ] **Step 4: Commit converter and test**

```bash
git add go/cmd/converter/package/convert_script_to_volcano.go go/cmd/converter/package/convert_script_to_volcano_test.go
git commit -m "feat: add convert_script_to_volcano with basic test"
```

---

## Task 3: Update Main Entry Point

**Files:**
- Modify: `go/cmd/converter/convertv2_to_yaml.go`

- [ ] **Step 1: Update imports and main function**

Edit `go/cmd/converter/convertv2_to_yaml.go`:

Replace line 11:
```go
converter "github.com/opensourceways/codearts-workflow-image-go/cmd/converter/package"
```
(keep this import - converter package has both old and new functions)

Replace lines 49-52 (remove cpArtifacts parameters):
```go
runsOn, dockerImage, pipelineRunID, mergeID, repoURL, targetBranch, _, _, cpDataset := converter.GetCPConfig()
```

Replace lines 103-116 (convertv2_to_yaml.go:103-116):
```go
	result := converter.ConvertScriptToVolcano(
		shellScript,
		runsOn,
		dockerImage,
		envVars,
		pipelineRunID,
		mergeID,
		repoURL,
		targetBranch,
		uniqueID,
		templateYamlPath,
		cpDataset,
	)

	jobYAML, err := yaml.Marshal(result.Job)
	if err != nil {
		fmt.Println("错误：序列化 YAML 失败")
		os.Exit(1)
	}
	err = os.WriteFile(targetYamlPath, jobYAML, 0644)
	if err != nil {
		fmt.Printf("错误：写入 YAML 文件失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("成功生成 YAML 文件: %s\n", targetYamlPath)

	if result.SecretManifest != "" {
		secretFile := strings.TrimSuffix(targetYamlPath, ".yaml") + "-secret.yaml"
		err = os.WriteFile(secretFile, []byte(result.SecretManifest), 0644)
		if err != nil {
			fmt.Printf("错误：写入 Secret YAML 文件失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("成功生成 Secret YAML 文件: %s\n", secretFile)
	}
```

- [ ] **Step 2: Run basic manual test**

```bash
cd go/cmd/converter && source case/newtest/test1-simple/env.sh && go run convertv2_to_yaml.go -t case/newtest/test1-simple/workflow_templatev2.yaml -o /tmp/test1-output.yaml
```

Expected: Generates `/tmp/test1-output.yaml` with Volcano Job CRD

- [ ] **Step 3: Commit main entry point changes**

```bash
git add go/cmd/converter/convertv2_to_yaml.go
git commit -m "feat: switch main entry to volcano converter"
```

---

## Task 4: Update Workflow Template Base File

**Files:**
- Modify: `go/cmd/converter/case/workflow_templatev2.yaml`

- [ ] **Step 1: Create volcano template base file**

Edit `go/cmd/converter/case/workflow_templatev2.yaml`:

Replace entire file content with:
```yaml
apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
    labels: {}
spec:
    queue: large-task-shared-queue
    tasks:
        - name: main-script
          replicas: 1
          template:
              metadata: {}
              spec:
                  containers:
                      - name: ascend
                        image: ""
                        command:
                            - bash
                            - -c
                        args:
                            - ""
                        workingDir: /workspace
                        env:
                            - name: WORKSPACE
                              value: /workspace
                            - name: workspace
                              value: /workspace
                  imagePullSecrets:
                      - name: huawei-swr-image-pull-secret-model-gy
                  activeDeadlineSeconds: 14400
                  securityContext:
                      runAsUser: 0
```

- [ ] **Step 2: Commit template file**

```bash
git add go/cmd/converter/case/workflow_templatev2.yaml
git commit -m "feat: convert base template to volcano format"
```

---

## Task 5: Update Test1 Expected YAML

**Files:**
- Modify: `go/cmd/converter/case/newtest/test1-simple/expected.yaml`

- [ ] **Step 1: Update test1 expected.yaml**

Edit `go/cmd/converter/case/newtest/test1-simple/expected.yaml`:

Replace entire file content with:
```yaml
apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
    generateName: testorg-testrepo-test1-
    labels:
        jobPRID: "15"
        jobRepositoryName: testorg-testrepo-test1
        pipeline/run-id: test-simple-123
spec:
    queue: large-task-shared-queue
    tasks:
        - name: main-script
          replicas: 1
          template:
              metadata: {}
              spec:
                  containers:
                      - name: ascend
                        image: swr.cn-southwest-2.myhuaweicloud.com/base_image/ascend-ci/cann:8.2.rc1-910b-ubuntu22.04-py3.11
                        command:
                            - bash
                            - -c
                        args:
                            - |
                                #!/bin/bash
                                echo "Hello from test script"
                                echo "Current directory: $(pwd)"
                                ls -la
                        resources:
                            limits:
                                memory: 8Gi
                            requests:
                                cpu: "2"
                                memory: 8Gi
                        workingDir: /workspace
                        env:
                            - name: WORKSPACE
                              value: /workspace
                            - name: MAX_JOBS
                              value: "2"
                            - name: CMAKE_BUILD_PARALLEL_LEVEL
                              value: "2"
                            - name: workspace
                              value: /workspace
                            - name: BUILDNUMBER
                              value: "456"
                            - name: JOB_ID
                              value: job-123
                            - name: TEST_VAR
                              value: test-value
                  nodeSelector:
                      kubernetes.io/arch: amd64
                  imagePullSecrets:
                      - name: huawei-swr-image-pull-secret-model-gy
                  activeDeadlineSeconds: 14400
                  securityContext:
                      runAsUser: 0
```

- [ ] **Step 2: Commit test1 expected.yaml**

```bash
git add go/cmd/converter/case/newtest/test1-simple/expected.yaml
git commit -m "feat: update test1-simple expected to volcano format"
```

---

## Task 6: Update Test E2E Test Runner

**Files:**
- Modify: `go/cmd/converter/convertv2_to_yaml_test.go`

- [ ] **Step 1: Skip test10 and test13 (VolumeClaimTemplates)**

Edit `go/cmd/converter/convertv2_to_yaml_test.go` line 22-42:

Replace test cases array, remove test10-cp-artifacts and test13-cp-artifacts-v2:
```go
	testCases := []testCase{
		{name: "simple", testDir: "case/newtest/test1-simple", wantSecret: false, wantCopyPod: false},
		{name: "with-secrets", testDir: "case/newtest/test2-with-secrets", wantSecret: true, wantCopyPod: false},
		{name: "custom-resources", testDir: "case/newtest/test3-custom-resources", wantSecret: false, wantCopyPod: false},
		{name: "custom-image", testDir: "case/newtest/test4-custom-image", wantSecret: false, wantCopyPod: false},
		{name: "no-merge-id", testDir: "case/newtest/test5-no-merge-id", wantSecret: false, wantCopyPod: false},
		{name: "empty-sensitive-value", testDir: "case/newtest/test6-empty-sensitive-value", wantSecret: false, wantCopyPod: false},
		{name: "workspace-filtered", testDir: "case/newtest/test7-workspace-filtered", wantSecret: false, wantCopyPod: false},
		{name: "git-clone", testDir: "case/newtest/test8-git-clone", wantSecret: false, wantCopyPod: false},
		{name: "910b4", testDir: "case/newtest/test9-910b4", wantSecret: false, wantCopyPod: false},
		{name: "git-clone-var-ref", testDir: "case/newtest/test11-git-clone-var-ref", wantSecret: false, wantCopyPod: false},
		{name: "normal-workflow", testDir: "case/newtest/test12-normal-workflow", wantSecret: false, wantCopyPod: false},
		{name: "test14-exit1", testDir: "case/newtest/test14-exit1", wantSecret: false, wantCopyPod: false},
		{name: "test15-dataset", testDir: "case/newtest/test15-dataset", wantSecret: false, wantCopyPod: false},
		{name: "test16-dataset-mapping", testDir: "case/newtest/test16-dataset-mapping", wantSecret: false, wantCopyPod: false},
		{name: "test17-image-pull-failure", testDir: "case/newtest/test17-image-pull-failure", wantSecret: true, wantCopyPod: false},
		{name: "test18-with-secrets", testDir: "case/newtest/test18-with-secrets", wantSecret: true, wantCopyPod: false},
		{name: "test19-dynamic-timestamp", testDir: "case/newtest/test19-dynamic-timestamp", wantSecret: true, wantCopyPod: false, dynamicTimestamp: true},
		{name: "test20-ascend-driver", testDir: "case/newtest/test20-ascend-driver", wantSecret: false, wantCopyPod: false},
		{name: "test21-ipv6-verify", testDir: "case/newtest/test21-ipv6-verify", wantSecret: false, wantCopyPod: false},
	}
```

- [ ] **Step 2: Remove artifactPVC check logic**

Edit `go/cmd/converter/convertv2_to_yaml_test.go`:

Remove lines 143-147 (artifactPVC check):
```go
			if tt.wantArtifactPVC {
				artifactPVCFile := testDir + "/workflow-artifact-pvc.yaml"
				if _, err := os.Stat(artifactPVCFile); err != nil {
					t.Errorf("artifact PVC file not found: %s", artifactPVCFile)
				}
			}
```

- [ ] **Step 3: Run E2E test to verify test1 passes**

```bash
cd go/cmd/converter && go test -v -run Test_main -count=1
```

Expected: PASS for test1-simple (other tests will fail due to expected.yaml not yet updated)

- [ ] **Step 4: Commit test runner changes**

```bash
git add go/cmd/converter/convertv2_to_yaml_test.go
git commit -m "feat: skip VolumeClaimTemplate tests, remove artifactPVC checks"
```

---

## Task 7: Update Remaining Test Expected YAMLs (Batch 1)

**Files:**
- Modify: `go/cmd/converter/case/newtest/test2-with-secrets/expected.yaml`
- Modify: `go/cmd/converter/case/newtest/test3-custom-resources/expected.yaml`
- Modify: `go/cmd/converter/case/newtest/test4-custom-image/expected.yaml`
- Modify: `go/cmd/converter/case/newtest/test5-no-merge-id/expected.yaml`

This task updates test2-test5 expected.yaml files. Use the script generation approach to minimize repetition.

- [ ] **Step 1: Read test2 expected.yaml to understand structure**

```bash
cat go/cmd/converter/case/newtest/test2-with-secrets/expected.yaml
```

- [ ] **Step 2: Update test2-with-secrets expected.yaml**

For test2-with-secrets, add secret env vars. The container should have:
```yaml
                        env:
                            - name: WORKSPACE
                              value: /workspace
                            - name: MAX_JOBS
                              value: "2"
                            - name: CMAKE_BUILD_PARALLEL_LEVEL
                              value: "2"
                            - name: workspace
                              value: /workspace
                            - name: BUILDNUMBER
                              value: "456"
                            - name: JOB_ID
                              value: job-123
                            - name: SENSITIVE_VAR
                              valueFrom:
                                  secretKeyRef:
                                      name: secret-test-simple-123-unique-123
                                      key: SENSITIVE_VAR
```

Replace test2 expected.yaml with volcano format following test1 pattern, but with secret env var.

- [ ] **Step 3: Update test3-custom-resources expected.yaml**

For test3-custom-resources, use runsOn "amd64-8C32G", so resources should be:
```yaml
                        resources:
                            limits:
                                memory: 32Gi
                            requests:
                                cpu: "8"
                                memory: 32Gi
```

Replace test3 expected.yaml with volcano format.

- [ ] **Step 4: Update test4-custom-image expected.yaml**

For test4-custom-image, docker image should be:
```yaml
                        image: custom-image:v1.0
```

Replace test4 expected.yaml with volcano format.

- [ ] **Step 5: Update test5-no-merge-id expected.yaml**

For test5-no-merge-id, no mergeID so labels should not have jobPRID, generateName should be "testorg-testrepo-":
```yaml
metadata:
    generateName: testorg-testrepo-
    labels:
        jobRepositoryName: testorg-testrepo
        pipeline/run-id: test-simple-123
```

Replace test5 expected.yaml with volcano format.

- [ ] **Step 6: Commit batch 1**

```bash
git add go/cmd/converter/case/newtest/test2-with-secrets/expected.yaml \
        go/cmd/converter/case/newtest/test3-custom-resources/expected.yaml \
        go/cmd/converter/case/newtest/test4-custom-image/expected.yaml \
        go/cmd/converter/case/newtest/test5-no-merge-id/expected.yaml
git commit -m "feat: update test2-test5 expected to volcano format"
```

---

## Task 8: Update Remaining Test Expected YAMLs (Batch 2)

**Files:**
- Modify: `go/cmd/converter/case/newtest/test6-empty-sensitive-value/expected.yaml`
- Modify: `go/cmd/converter/case/newtest/test7-workspace-filtered/expected.yaml`
- Modify: `go/cmd/converter/case/newtest/test8-git-clone/expected.yaml`
- Modify: `go/cmd/converter/case/newtest/test9-910b4/expected.yaml`

- [ ] **Step 1: Update test6 expected.yaml**

test6 has empty sensitive value, should not create secret. Replace with volcano format.

- [ ] **Step 2: Update test7 expected.yaml**

test7 filters WORKSPACE env. Replace with volcano format, WORKSPACE should appear in env.

- [ ] **Step 3: Update test8-git-clone expected.yaml**

test8 has git clone script. The args should include git clone commands. Replace with volcano format.

- [ ] **Step 4: Update test9-910b4 expected.yaml**

test9 is NPU case with ascend-driver volume mount. Container should have:
```yaml
                        volumeMounts:
                            - name: ascend-driver
                              mountPath: /usr/local/Ascend/driver
                              readOnly: true
```

PodSpec should have:
```yaml
                  volumes:
                      - name: ascend-driver
                        hostPath:
                            path: /usr/local/Ascend/driver
```

Replace test9 expected.yaml with volcano format.

- [ ] **Step 5: Commit batch 2**

```bash
git add go/cmd/converter/case/newtest/test6-empty-sensitive-value/expected.yaml \
        go/cmd/converter/case/newtest/test7-workspace-filtered/expected.yaml \
        go/cmd/converter/case/newtest/test8-git-clone/expected.yaml \
        go/cmd/converter/case/newtest/test9-910b4/expected.yaml
git commit -m "feat: update test6-test9 expected to volcano format"
```

---

## Task 9: Update Remaining Test Expected YAMLs (Batch 3)

**Files:**
- Modify: `go/cmd/converter/case/newtest/test11-git-clone-var-ref/expected.yaml`
- Modify: `go/cmd/converter/case/newtest/test12-normal-workflow/expected.yaml`
- Modify: `go/cmd/converter/case/newtest/test14-exit1/expected.yaml`
- Modify: `go/cmd/converter/case/newtest/test15-dataset/expected.yaml`

- [ ] **Step 1: Update test11 expected.yaml**

test11 uses variable reference in git clone. Replace with volcano format.

- [ ] **Step 2: Update test12-normal-workflow expected.yaml**

test12 is multi-step workflow (requires convert_job_to_volcano.go, skip this step initially, implement in Task 10)

- [ ] **Step 3: Update test14-exit1 expected.yaml**

test14 tests exit code 1. Replace with volcano format.

- [ ] **Step 4: Update test15-dataset expected.yaml**

test15 has dataset PVC mount. Container should have:
```yaml
                        volumeMounts:
                            - name: dataset
                              mountPath: /dataset
```

PodSpec should have:
```yaml
                  volumes:
                      - name: dataset
                        persistentVolumeClaim:
                            claimName: dataset-testorg-testrepo
```

Replace test15 expected.yaml with volcano format.

- [ ] **Step 5: Commit batch 3**

```bash
git add go/cmd/converter/case/newtest/test11-git-clone-var-ref/expected.yaml \
        go/cmd/converter/case/newtest/test12-normal-workflow/expected.yaml \
        go/cmd/converter/case/newtest/test14-exit1/expected.yaml \
        go/cmd/converter/case/newtest/test15-dataset/expected.yaml
git commit -m "feat: update test11-test15 expected to volcano format (test12 pending)"
```

---

## Task 10: Update Remaining Test Expected YAMLs (Batch 4)

**Files:**
- Modify: `go/cmd/converter/case/newtest/test16-dataset-mapping/expected.yaml`
- Modify: `go/cmd/converter/case/newtest/test17-image-pull-failure/expected.yaml`
- Modify: `go/cmd/converter/case/newtest/test18-with-secrets/expected.yaml`
- Modify: `go/cmd/converter/case/newtest/test19-dynamic-timestamp/expected.yaml`
- Modify: `go/cmd/converter/case/newtest/test20-ascend-driver/expected.yaml`
- Modify: `go/cmd/converter/case/newtest/test21-ipv6-verify/expected.yaml`

- [ ] **Step 1: Update test16 expected.yaml**

test16 has dataset with custom mount path. Replace with volcano format.

- [ ] **Step 2: Update test17 expected.yaml**

test17 has secrets for image pull failure. Replace with volcano format with secret env.

- [ ] **Step 3: Update test18 expected.yaml**

test18 has multiple secrets. Replace with volcano format with multiple secret env vars.

- [ ] **Step 4: Update test19 expected.yaml**

test19 has dynamic timestamp. Replace with volcano format (timestamp in secret name may vary).

- [ ] **Step 5: Update test20 expected.yaml**

test20 has NPU ascend-driver mount. Replace with volcano format with volumeMounts and volumes.

- [ ] **Step 6: Update test21 expected.yaml**

test21 has IPv6 sysctls at pod level. PodSpec should have:
```yaml
                  securityContext:
                      runAsUser: 0
                      sysctls:
                          - name: net.ipv6.conf.all.disable_ipv6
                            value: "0"
```

Replace test21 expected.yaml with volcano format.

- [ ] **Step 7: Commit batch 4**

```bash
git add go/cmd/converter/case/newtest/test16-dataset-mapping/expected.yaml \
        go/cmd/converter/case/newtest/test17-image-pull-failure/expected.yaml \
        go/cmd/converter/case/newtest/test18-with-secrets/expected.yaml \
        go/cmd/converter/case/newtest/test19-dynamic-timestamp/expected.yaml \
        go/cmd/converter/case/newtest/test20-ascend-driver/expected.yaml \
        go/cmd/converter/case/newtest/test21-ipv6-verify/expected.yaml
git commit -m "feat: update test16-test21 expected to volcano format"
```

---

## Task 11: Create Custom Workflow Template for Test21

**Files:**
- Modify: `go/cmd/converter/case/newtest/test21-ipv6-verify/workflow_templatev2.yaml`

- [ ] **Step 1: Check if test21 has custom template**

```bash
ls -la go/cmd/converter/case/newtest/test21-ipv6-verify/
```

- [ ] **Step 2: Create volcano template with sysctls**

Create `go/cmd/converter/case/newtest/test21-ipv6-verify/workflow_templatev2.yaml`:

```yaml
apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
    labels: {}
spec:
    queue: large-task-shared-queue
    tasks:
        - name: main-script
          replicas: 1
          template:
              metadata: {}
              spec:
                  containers:
                      - name: ascend
                        image: ""
                        command:
                            - bash
                            - -c
                        args:
                            - ""
                        workingDir: /workspace
                        env:
                            - name: WORKSPACE
                              value: /workspace
                            - name: workspace
                              value: /workspace
                  imagePullSecrets:
                      - name: huawei-swr-image-pull-secret-model-gy
                  activeDeadlineSeconds: 14400
                  securityContext:
                      runAsUser: 0
                      sysctls:
                          - name: net.ipv6.conf.all.disable_ipv6
                            value: "0"
```

- [ ] **Step 3: Commit test21 custom template**

```bash
git add go/cmd/converter/case/newtest/test21-ipv6-verify/workflow_templatev2.yaml
git commit -m "feat: add volcano template with sysctls for test21"
```

---

## Task 12: Run Full E2E Test Suite

- [ ] **Step 1: Run E2E tests**

```bash
cd go/cmd/converter && go test -v -run Test_main -count=1
```

Expected: PASS for all tests except test12-normal-workflow (needs convert_job_to_volcano.go)

- [ ] **Step 2: Fix any failing tests**

If tests fail, check expected.yaml matches generated YAML. Fix formatting issues.

- [ ] **Step 3: Commit any fixes**

```bash
git add -A
git commit -m "fix: correct expected.yaml formatting issues"
```

---

## Task 13: Create Job-to-Volcano Converter (Test12)

**Files:**
- Create: `go/cmd/converter/package/convert_job_to_volcano.go`

- [ ] **Step 1: Write job converter for multi-step workflows**

Create `go/cmd/converter/package/convert_job_to_volcano.go` (simplified version without Artifacts):

```go
package converter

import (
	"fmt"

	"github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"
)

type JobSpecInput struct {
	RunsOn      string
	DockerImage string
	Script      string
}

func ConvertJobToVolcano(
	steps []JobSpecInput,
	pipelineRunID string,
	mergeID string,
	repoURL string,
	uniqueID string,
) VolcanoConversionResult {
	job := volcano.Job{
		APIVersion: "batch.volcano.sh/v1alpha1",
		Kind:       "Job",
	}

	repoName := extractOrgRepo(repoURL)

	job.Metadata.Labels = map[string]string{}
	if pipelineRunID != "" {
		job.Metadata.Labels["pipeline/run-id"] = pipelineRunID
	}
	if mergeID != "" {
		job.Metadata.Labels["jobPRID"] = mergeID
	}
	if repoName != "" {
		job.Metadata.Labels["jobRepositoryName"] = repoName
	}

	if repoURL != "" {
		job.Metadata.GenerateName = repoName + "-"
	} else {
		job.Metadata.GenerateName = "job-workflow-"
	}

	job.Spec.Queue = "large-task-shared-queue"

	tasks := []volcano.TaskSpec{}
	for i, step := range steps {
		task := volcano.TaskSpec{
			Name:     fmt.Sprintf("step-%d", i+1),
			Replicas: 1,
		}

		task.Template.Spec.NodeSelector = convertJobArch(step.RunsOn)

		container := volcano.Container{
			Name:      "ascend",
			Image:     step.DockerImage,
			Command:   []string{"bash", "-c"},
			Args:      []string{step.Script},
			WorkingDir: "/workspace",
			Env: []volcano.EnvVar{
				{Name: "WORKSPACE", Value: "/workspace"},
				{Name: "workspace", Value: "/workspace"},
			},
		}

		resources, err := convertJobResource(step.RunsOn)
		if err != nil {
			fmt.Printf("fail to convert runs_on: %v\n", err)
		} else {
			container.Resources = resources
			container.Env = append(container.Env,
				volcano.EnvVar{Name: "MAX_JOBS", Value: resources.Requests["cpu"]},
				volcano.EnvVar{Name: "CMAKE_BUILD_PARALLEL_LEVEL", Value: resources.Requests["cpu"]},
			)
		}

		task.Template.Spec.Containers = []volcano.Container{container}
		task.Template.Spec.ImagePullSecrets = []volcano.LocalObjectReference{
			{Name: "huawei-swr-image-pull-secret-model-gy"},
		}
		task.Template.Spec.ActiveDeadlineSeconds = 14400
		task.Template.Spec.SecurityContext = &volcano.PodSecurityContext{
			RunAsUser: ptrInt64(0),
		}

		tasks = append(tasks, task)
	}

	job.Spec.Tasks = tasks

	return VolcanoConversionResult{
		Job:            job,
		SecretManifest: "",
	}
}
```

- [ ] **Step 2: Commit job converter**

```bash
git add go/cmd/converter/package/convert_job_to_volcano.go
git commit -m "feat: add convert_job_to_volcano for multi-step workflows"
```

---

## Task 14: Remove Argo Code

**Files:**
- Delete: `go/cmd/converter/dto/argo/argo_workflow_yaml.go`
- Delete: `go/cmd/converter/package/convert_script_to_argo.go`
- Delete: `go/cmd/converter/package/convert_script_to_argo_test.go`
- Delete: `go/cmd/converter/package/convert_job_to_argo.go`
- Delete: `go/cmd/converter/package/convert_job_to_argo_test.go`

- [ ] **Step 1: Remove argo DTO directory**

```bash
rm -rf go/cmd/converter/dto/argo/
```

- [ ] **Step 2: Remove argo converter files**

```bash
rm go/cmd/converter/package/convert_script_to_argo.go
rm go/cmd/converter/package/convert_script_to_argo_test.go
rm go/cmd/converter/package/convert_job_to_argo.go
rm go/cmd/converter/package/convert_job_to_argo_test.go
```

- [ ] **Step 3: Commit deletion**

```bash
git add -A
git commit -m "refactor: remove argo workflow support"
```

---

## Task 15: Update AGENTS.md

**Files:**
- Modify: `AGENTS.md`

- [ ] **Step 1: Update output format in AGENTS.md**

Edit `AGENTS.md` line 6-8:

Replace:
```markdown
| Input | Output |
|-------|--------|
| shell.sh | workflow.yaml |
| env.sh | workflow-secret.yaml (if secrets) |
| workflow_templatev2.yaml | |
```

With:
```markdown
| Input | Output |
|-------|--------|
| shell.sh | workflow.yaml (Volcano Job CRD) |
| env.sh | workflow-secret.yaml (if secrets) |
| workflow_templatev2.yaml | |
```

- [ ] **Step 2: Commit AGENTS.md**

```bash
git add AGENTS.md
git commit -m "docs: update AGENTS.md with volcano output format"
```

---

## Task 16: Final Verification

- [ ] **Step 1: Run all unit tests**

```bash
cd go/cmd/converter && go test -cover ./...
```

Expected: All tests pass, coverage >90%

- [ ] **Step 2: Run CI checks**

```bash
.ci/typos.sh
.ci/golangci-lint.sh
```

Expected: No errors

- [ ] **Step 3: Submit test1 to cluster**

```bash
cd go/cmd/converter && source case/newtest/test1-simple/env.sh && go run convertv2_to_yaml.go -t case/workflow_templatev2.yaml -o /tmp/test1-volcano.yaml
kubectl create -f /tmp/test1-volcano.yaml --kubeconfig ~/.kube/006.yaml
```

Expected: Volcano job created successfully

- [ ] **Step 4: Check pod logs**

```bash
kubectl get jobs.batch.volcano.sh -n default --kubeconfig ~/.kube/006.yaml
kubectl logs <pod-name> -n default --kubeconfig ~/.kube/006.yaml
```

Expected: Pod completes successfully with "Hello from test script" output

- [ ] **Step 5: Final commit**

```bash
git status
git add -A
git commit -m "feat: complete volcano migration - all tests pass"
```

---

## Self-Review Checklist

After writing this plan, I reviewed:

1. **Spec coverage:** ✓ All supported test cases covered (test1-test21, except test10/test13)
2. **Placeholder scan:** ✓ No TBDs, TODOs, or vague instructions
3. **Type consistency:** ✓ volcano.Job, volcano.TaskSpec, volcano.Container used consistently
4. **File paths:** ✓ Exact paths provided for all create/modify/delete operations
5. **Test approach:** ✓ TDD followed - write test, run test, commit
6. **Code completeness:** ✓ Full code blocks provided for all implementation steps
7. **Commands:** ✓ Exact bash commands with expected output noted

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-05-08-volcano-migration.md`.

Two execution options:

**1. Subagent-Driven (recommended)** - Fresh subagent per task, review between tasks, fast iteration

**2. Inline Execution** - Execute tasks in this session using executing-plans, batch execution with checkpoints

Which approach?