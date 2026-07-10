# Design: Replace Argo Workflow with Volcano Job

## Overview

Convert the codearts-workflow-image converter from generating Argo Workflow CRDs to generating Volcano Job CRDs.

## Motivation

- Simplify architecture by using Volcano directly instead of Argo with volcano scheduler
- Better integration with cluster queue management (large-task-shared-queue)
- Remove dependency on Argo-specific features not needed in Volcano

## Architecture Changes

### Current (Argo)

- `dto/argo/argo_workflow_yaml.go` - Argo Workflow DTOs
- `convert_script_to_argo.go` - Script → Argo Workflow converter
- `convert_job_to_argo.go` - Job spec → Argo Workflow converter
- Output: `Workflow` CRD (apiVersion: argoproj.io/v1alpha1)

### Target (Volcano)

- `dto/volcano/volcano_job_yaml.go` - Volcano Job DTOs
- `convert_script_to_volcano.go` - Script → Volcano Job converter
- `convert_job_to_volcano.go` - Job spec → Volcano Job converter
- Output: `Job` CRD (apiVersion: batch.volcano.sh/v1alpha1)

## Key Structural Differences

| Feature | Argo Workflow | Volcano Job |
|---------|---------------|-------------|
| API Version | argoproj.io/v1alpha1 | batch.volcano.sh/v1alpha1 |
| Kind | Workflow | Job |
| Entry point | templates[].script | tasks[].template.spec.containers[] |
| Scheduler | schedulerName: volcano | schedulerName: volcano (implicit) |
| Queue | N/A | queue: large-task-shared-queue |
| Script execution | script.source: \| | containers[].args: \| (with command: [bash, -c]) |
| Task name | templates[].name | tasks[].name + replicas: 1 |
| GenerateName | metadata.generateName | metadata.generateName (same) |

## Implementation Plan

### Phase 1: Create Volcano DTOs

Create `go/cmd/converter/dto/volcano/volcano_job_yaml.go` with:

```go
type Job struct {
    APIVersion string   `yaml:"apiVersion"`
    Kind       string   `yaml:"kind"`
    Metadata   Metadata `yaml:"metadata"`
    Spec       JobSpec  `yaml:"spec"`
}

type JobSpec struct {
    Queue         string     `yaml:"queue"`
    MaxRetry      int        `yaml:"maxRetry,omitempty"`      // default: 3
    MinAvailable  int        `yaml:"minAvailable,omitempty"`  // default: 1
    SchedulerName string     `yaml:"schedulerName,omitempty"` // default: volcano
    Tasks         []TaskSpec `yaml:"tasks"`
}

type TaskSpec struct {
    Name         string          `yaml:"name"`
    Replicas     int             `yaml:"replicas"`
    MaxRetry     int             `yaml:"maxRetry,omitempty"`     // default: 3
    MinAvailable int             `yaml:"minAvailable,omitempty"` // default: 1
    Template     PodTemplateSpec `yaml:"template"`
}

type PodTemplateSpec struct {
    Metadata Metadata `yaml:"metadata,omitempty"` // can be {}
    Spec     PodSpec  `yaml:"spec"`
}

type PodSpec struct {
    Containers          []Container         `yaml:"containers"`
    NodeSelector        map[string]string   `yaml:"nodeSelector,omitempty"`
    ImagePullSecrets    []LocalObjectReference `yaml:"imagePullSecrets,omitempty"`
    ActiveDeadlineSeconds int64              `yaml:"activeDeadlineSeconds,omitempty"`
    SecurityContext     *PodSecurityContext `yaml:"securityContext,omitempty"`
    Volumes             []Volume            `yaml:"volumes,omitempty"`
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
```

Reuse existing types from Argo DTO where applicable:
- Metadata (generateName, labels)
- Volume, VolumeMount, HostPath, PersistentVolumeClaimVolume
- EnvVar, EnvVarSource, ConfigMapKeySelector, SecretKeySelector
- Resources, ResourceList
- LocalObjectReference
- PodSecurityContext (runAsUser, sysctls) - already defined in Argo DTO

### Phase 2: Create Converter Functions

Create `convert_script_to_volcano.go`:

1. Copy structure from `convert_script_to_argo.go`
2. Change return type from `argo.Workflow` to `volcano.Job`
3. Map template structure:
   - `argoWorkflow.Spec.Templates[0].Script` → `volcanoJob.Spec.Tasks[0].Template.Spec.Containers[0]`
4. Script source transformation:
   - Argo: `template.Script.Source = script`
   - Volcano: `container.Command = ["bash", "-c"]`, `container.Args = [script]`
5. Add queue: `volcanoJob.Spec.Queue = "large-task-shared-queue"`
6. Add replicas: `task.Replicas = 1`
7. Remove Argo-specific: VolumeClaimTemplates, OnExit, ArtifactRepositoryRef, Steps

Create `convert_job_to_volcano.go` for multi-step workflows (test12-normal-workflow).

### Phase 3: Update Main Entry Point

Update `convertv2_to_yaml.go`:

- Import volcano DTO instead of argo
- Call `ConvertScriptToVolcano()` instead of `ConvertScriptToArgo()`
- Change result type from `ScriptConversionResult` to `VolcanoConversionResult`
- Output filename: keep `workflow.yaml` (minimal change for existing tooling)

### Phase 4: Update Tests

1. Update all `expected.yaml` files in test cases:
   - Change `apiVersion: argoproj.io/v1alpha1` → `batch.volcano.sh/v1alpha1`
   - Change `kind: Workflow` → `kind: Job`
   - Add `queue: large-task-shared-queue` to spec
   - Move script content from `templates[].script.source` to `tasks[].template.spec.containers[].args`
   - Add `replicas: 1` to each task
   - Change template structure from Argo to Volcano format

2. Skip tests with VolumeClaimTemplates:
   - test10-cp-artifacts
   - test13-cp-artifacts-v2

3. Update test utilities:
   - Change result type expectations
   - Remove artifactPVC checks

### Phase 5: Cleanup

1. Remove unused Argo DTOs:
   - Delete `go/cmd/converter/dto/argo/` directory
   
2. Remove unused converters:
   - Delete `convert_script_to_argo.go`
   - Delete `convert_job_to_argo.go`

3. Remove Argo-specific code:
   - VolumeClaimTemplates handling
   - OnExit handlers
   - Artifact S3 support
   - CopyPod generation

4. Update imports in all remaining files

## Supported Test Cases

Phase 1 implementation supports these test cases (no VolumeClaimTemplates or Artifacts):

- test1-simple
- test2-with-secrets
- test3-custom-resources
- test4-custom-image
- test5-no-merge-id
- test6-empty-sensitive-value
- test7-workspace-filtered
- test8-git-clone
- test9-910b4 (NPU with ascend-driver)
- test11-git-clone-var-ref
- test12-normal-workflow (multi-step)
- test14-exit1
- test15-dataset
- test16-dataset-mapping
- test17-image-pull-failure
- test18-with-secrets
- test19-dynamic-timestamp
- test20-ascend-driver (NPU volume mount)
- test21-ipv6-verify (pod-level sysctls)

**Skipped (Argo-specific features):**
- test10-cp-artifacts (VolumeClaimTemplates)
- test13-cp-artifacts-v2 (VolumeClaimTemplates)

## Feature Mapping

### Features to Maintain

1. **GenerateName**: Keep metadata.generateName (works in both)
2. **Labels**: Keep metadata.labels (works in both)
3. **SecurityContext**: Pod-level securityContext with runAsUser and sysctls
4. **Volumes**: HostPath and PersistentVolumeClaim volumes
5. **VolumeMounts**: Container volume mounts (ascend-driver, dataset)
6. **NodeSelector**: Architecture selection (amd64, arm64, arm64-npu-1, etc.)
7. **Resources**: CPU and memory requests/limits
8. **Environment Variables**: Plain env vars and sensitive env vars (secrets)
9. **ImagePullSecrets**: Huawei SWR image pull secret
10. **ActiveDeadlineSeconds**: Job timeout (14400s)

### Features to Remove

1. **VolumeClaimTemplates**: Argo-only feature for creating PVCs dynamically
2. **OnExit handlers**: Cleanup hooks for secrets (Argo-specific lifecycle)
3. **ArtifactRepositoryRef**: S3 artifact support
4. **Artifacts**: S3 input/output artifacts
5. **CopyPod**: Separate pod for copying artifacts from VolumeClaimTemplates
6. **Steps**: Argo workflow DAG structure (replace with Volcano tasks)

### Features to Add

1. **Queue**: Required for Volcano: `large-task-shared-queue`
2. **Replicas**: Required for each Volcano task: `replicas: 1`
3. **MinAvailable**: Optional for gang scheduling (default: same as replicas)

## Testing Strategy

### Manual Testing

1. Create Volcano Job YAML manually for test1-simple ✓
2. Submit to cluster: `kubectl create -f volcano-job.yaml --kubeconfig ~/.kube/006.yaml`
3. Verify job runs successfully
4. Check pod logs: `kubectl logs <pod-name> --kubeconfig ~/.kube/006.yaml`

### Unit Testing

1. Test DTO marshaling/unmarshaling
2. Test converter function logic:
   - Script → Container args transformation
   - Resource conversion
   - NodeSelector mapping
   - Volume mount handling
   - Secret environment variable handling

### E2E Testing

1. Run `go test -v -run Test_main` in `go/cmd/converter`
2. Validate YAML generation matches expected.yaml
3. Verify secret generation for sensitive env cases
4. Test NPU-specific cases (test9-910b4, test20-ascend-driver)
5. Test sysctl cases (test21-ipv6-verify)

### Cluster Testing

Submit each supported test case to cluster using submit-test skill:
```bash
skill submit-test -k ~/.kube/006.yaml -t all
```

## Error Handling

1. **Queue not found**: Error message with available queues list
2. **Image pull failure**: Keep existing test17 test case
3. **NPU scheduling**: May timeout (test9-910b4 known issue)
4. **Sysctl not allowlisted**: Pod rejected (test21-ipv6-verify cluster policy issue)

## Migration Checklist

- [ ] Create volcano DTO package
- [ ] Create convert_script_to_volcano.go
- [ ] Create convert_job_to_volcano.go
- [ ] Update convertv2_to_yaml.go main entry
- [ ] Update all expected.yaml files
- [ ] Update test utilities
- [ ] Remove dto/argo package
- [ ] Remove convert_script_to_argo.go
- [ ] Remove convert_job_to_argo.go
- [ ] Run unit tests
- [ ] Run e2e tests
- [ ] Submit test cases to cluster
- [ ] Update AGENTS.md with new output format

## Success Criteria

1. All unit tests pass (>90% coverage)
2. All E2E tests pass for supported test cases
3. test1-simple Volcano job runs successfully on cluster ✓
4. YAML output matches Volcano Job CRD schema
5. No Argo-specific code remains in codebase