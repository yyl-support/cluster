# CP Artifact Migration Design

**Date:** 2026-06-08  
**Status:** Draft  
**Author:** Claude (via opencode)

## Overview

Migrate three artifact-related test cases from `skip/` directory to `newtest/`, enabling multi-task Volcano Jobs with Karmada-compatible artifact extraction. The system extends the existing Volcano Job converter to support dynamic PVC creation and artifact handling without `kubectl cp`.

## Goals

1. **Volcano Job Compatibility**: Convert Argo Workflow artifact tests to multi-task Volcano Jobs
2. **Karmada Compatibility**: Use `kubectl exec tar` instead of `kubectl cp` for artifact extraction
3. **Backward Compatibility**: Preserve single-task mode for non-artifact workflows
4. **Test Coverage**: Enable E2E testing of artifact handling with Volcano scheduler

## Background

### Current State

- **Argo-harbor branch**: Contains artifact tests using Argo Workflows with `volumeClaimTemplates`
- **Current converter**: Generates single-task Volcano Jobs with inline artifact copying
- **skip/ directory**: Contains 3 skipped test cases:
  - test10-cp-artifacts
  - test12-normal-workflow
  - test13-cp-artifacts-v2

### Problem

- Argo's `volumeClaimTemplates` is Argo-specific (not available in Volcano)
- `kubectl cp` doesn't work reliably in Karmada multi-cluster environments
- Current inline artifact copying doesn't support external extraction scenarios

## Input Parameters

The artifact handling logic depends on two environment variables:

| Parameter | Description | Example |
|-----------|-------------|---------|
| `CP_artifacts` | File patterns to copy (semicolon-separated) | `"*.txt;*.log"` |
| `CP_artifacts_temp_folder` | Destination folder inside pod | `/output/artifact` |

**Default behavior:** If `CP_artifacts_temp_folder` is empty and `CP_artifacts` is specified, defaults to `/output`.

### Three Scenarios

| Scenario | CP_artifacts | CP_artifacts_temp_folder | Behavior |
|----------|--------------|-------------------------|----------|
| **Direct Creation** | Empty | Specified | User writes directly to temp folder (test10) |
| **Inline Copy (Default)** | Specified | Empty → `/output` | Inline `cp -r --parents {patterns} /output/` |
| **Pattern Copy (Custom)** | Specified | Specified | Inline `cp -r --parents {patterns} {temp_folder}/` (test13) |

**Key insight:** Any scenario with `CP_artifacts_temp_folder != ""` requires multi-task Volcano Job for external artifact extraction.

## Architecture

### Component Overview

```
┌─────────────────────────────────────────────┐
│ Converter (convert_script_to_volcano.go)    │
│                                             │
│  1. Parse env vars                          │
│  2. Generate main-script task               │
│  3. IF CP_artifacts_temp_folder != ""       │
│     → addArtifactVolume() [inline helper]   │
│     → createCopyArtifactTask() [inline]     │
│     → volcanoJob.Spec.Volumes (dynamic PVC) │
│     → volcanoJob.Spec.Tasks = [main, copy]  │
│  4. ELSE                                    │
│     → volcanoJob.Spec.Tasks = [main]        │
└                                             │
└─────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────┐
│ cp_artifact_manager.go (pure functions)     │
│                                             │
│  • NeedsArtifactMultiTask()                 │
│  • GetArtifactMountPath()                   │
│  • Constants (volume name, storage, image)  │
└                                             │
└─────────────────────────────────────────────┘
         │
         ▼
┌─────────────────────────────────────────────┐
│ Volcano Job CRD                             │
│                                             │
│  spec:                                      │
│    volumes:                                 │
│      - name: output                         │
│        mountPath: /output/artifact          │
│        volumeClaim:                         │
│          accessModes: [ReadWriteMany]       │
│          storageClassName: sfsturbo-subpath-sc │
│          resources:                         │
│            requests: {storage: 1Gi}         │
│    tasks:                                   │
│      - name: main-script                    │
│        template:                             │
│          spec:                               │
│            containers:                       │
│              - volumeMounts: [{output, ...}]│
│      - name: copy-artifact                  │
│        template:                             │
│          spec:                               │
│            containers:                       │
│              - image: alpine:3.23.3          │
│              - command: [sh]                 │
│              - args: [sleep infinity]        │
│              - volumeMounts: [{output, ...}]│
└                                             │
└─────────────────────────────────────────────┘
         │
         ▼ (Submit to cluster)
┌─────────────────────────────────────────────┐
│ eval.sh (Artifact Extraction)               │
│                                             │
│  pod=$(kubectl get pods \                   │
│    -l volcano.sh/task-name=copy-artifact)   │
│                                             │
│  kubectl exec $pod -- tar czf - \           │
│    -C /output/artifact . \                  │
│    | tar xzf - -C $WORKSPACE                │
│                                             │
│  Result: /output/artifact/abc/test.txt      │
│          → $WORKSPACE/abc/test.txt          │
│          (prefix stripped)                  │
└                                             │
└─────────────────────────────────────────────┘
```

### Key Design Decisions

1. **Volcano's dynamic PVC**: Use `spec.volumes[].volumeClaim` instead of Argo's `volumeClaimTemplates`
2. **Inline extension strategy**: Follow argo-harbor pattern - inline helpers, no separate converter function
3. **Manager pattern**: Pure functions for constants and logic checks, inline helpers for Volcano Job modifications
4. **Path stripping**: Use `tar -C /output/artifact .` to strip prefix when extracting

## Implementation Details

### File Changes

#### 1. New File: `go/cmd/converter/package/cp_artifact_manager.go`

**Purpose:** Provide constants and pure functions for artifact handling.

**Content:**
- Constants: `ArtifactVolumeName`, `ArtifactStorageClass`, `ArtifactStorageSize`, `CopyArtifactImage`
- Functions:
  - `NeedsArtifactMultiTask(cpArtifacts, cpArtifactsTempFolder string) bool`
  - `GetArtifactMountPath(cpArtifactsTempFolder string) string`
  - `GetCopyArtifactTaskName() string`

**Pattern:** Follow existing `queue_manager.go` and `dataset_manager.go` structure.

#### 2. Modify: `go/cmd/converter/dto/volcano/volcano_job_yaml.go`

**Purpose:** Add VolumeClaim support to Volcano DTO.

**Changes:**
```go
type Volume struct {
    Name                  string
    MountPath             string                       // NEW: Required for Volcano spec.volumes
    HostPath              *HostPath
    PersistentVolumeClaim *PersistentVolumeClaimVolume
    VolumeClaim           *VolumeClaim                 // NEW: Dynamic PVC creation
    VolumeClaimName       string                       // NEW: Reference existing PVC
}

type VolumeClaim struct {                              // NEW struct
    AccessModes      []string
    StorageClassName string
    Resources        ResourceRequest
    DataSource       *VolumeDataSource `yaml:"dataSource,omitempty"`
    VolumeMode       string             `yaml:"volumeMode,omitempty"`
}

type VolumeDataSource struct {                         // NEW struct
    Name string `yaml:"name"`
    Kind string `yaml:"kind"`
}

type ResourceRequest struct {                          // NEW struct
    Requests map[string]string `yaml:"requests"`
}

type JobSpec struct {
    // ... existing fields ...
    Volumes []Volume  `yaml:"volumes,omitempty"`      // NEW: Job-level volumes
}
```

**Rationale:** Volcano Jobs support dynamic PVC creation at `spec.volumes[].volumeClaim` level (verified with `kubectl --kubeconfig ~/.kube/006.yaml explain job.spec.volumes.volumeClaim`).

#### 3. Modify: `go/cmd/converter/package/convert_script_to_volcano.go`

**Purpose:** Inline multi-task artifact handling.

**Changes:**
- Add inline helper `addArtifactVolume()` (after line 212, similar to `addDatasetVolume()`)
- Add inline helper `createCopyArtifactTask()` (after `addArtifactVolume()`)
- Inline multi-task logic (after line 140, similar to `if cpDataset != ""` block)

**Pattern:** Follow argo-harbor's inline approach from `convert_script_to_argo.go` lines 191-224.

**Code location:** After existing `cpDataset` handling (line 140), before `task.Template.Spec.Containers` assignment (line 152).

#### 4. Test Case Files: `go/cmd/converter/case/newtest/`

**test10-cp-artifacts/ migration:**
- Copy from `skip/test10-cp-artifacts/`
- Update `expected.yaml`:
  - `apiVersion: batch.volcano.sh/v1alpha1`
  - `kind: Job`
  - Add `spec.volumes` with dynamic PVC
  - Change `templates` → `tasks` with 2 tasks: `main-script` + `copy-artifact`
  - Remove `volumeClaimTemplates`, `entrypoint`
- Update `eval.sh`:
  - `kubectl get job.batch.volcano.sh`
  - Use Volcano Job label selectors
  - Validate files in WORKSPACE (extracted by submit-test using kubectl exec on copy-artifact task pod)
- Add to `test-cases.json`:
  - NO `expected-copy-pod` field (Volcano Jobs use built-in copy-artifact task)

**test12-normal-workflow/ migration:**
- Copy from `skip/test12-normal-workflow/`
- Already Volcano format, add multi-task structure
- Add artifact volume + copy-artifact task
- Preserve inline `cp -r --parents` in shell script
- Update `eval.sh` for Volcano Job validation
- Add to `test-cases.json`:
  - NO `expected-copy-pod` field (Volcano Jobs use built-in copy-artifact task)

**test13-cp-artifacts-v2/ migration:**
- Copy from `skip/test13-cp-artifacts-v2/`
- Pattern-based artifact copying with custom temp folder
- Similar migration pattern as test10
- Preserve inline copy logic: `cp -r --parents *.txt *.log /output/artifact/`
- Update `eval.sh` for Volcano Job validation
- Add to `test-cases.json`:
  - NO `expected-copy-pod` field (Volcano Jobs use built-in copy-artifact task)

### Inline Helper Functions

**addArtifactVolume() - Similar to addDatasetVolume():**
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

**createCopyArtifactTask():**
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

### Inline Usage in ConvertScriptToVolcano

**Location:** After line 140 (after `cpDataset` handling).

```go
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

### Submit-Test Integration

**Submit-test architecture:**
- Supports both Argo Workflows (test1-13) and Volcano Jobs (test14-28)
- Argo Workflows: Use separate copy pod (expected-copy-pod.yaml required)
- **Volcano Jobs: Use built-in copy-artifact task (NO separate copy pod file)**

**Volcano Job copy-artifact task extraction:**
```bash
# Step 1: Find copy-artifact task pod using Volcano labels
pod_name=$(kubectl get pods -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" \
  -l volcano.sh/job-name=${WORKFLOW_NAME},volcano.sh/task-name=copy-artifact \
  -o jsonpath='{.items[0].metadata.name}')

# Step 2: Stream artifacts with path stripping
kubectl exec "$pod_name" -n "$NAMESPACE" --kubeconfig "$KUBECONFIG" \
  -- tar czf - -C /output/artifact . | tar xzf - -C "${WORKSPACE}"
```

**Why no expected-copy-pod.yaml for Volcano Jobs:**
- Volcano Job already includes copy-artifact task (second task in tasks array)
- Task pod has label `volcano.sh/task-name=copy-artifact`
- Submit-test uses kubectl exec on task pod directly
- No separate pod creation needed

**Argo Workflow copy pod (different mechanism):**
- Argo uses separate copy pod (expected-copy-pod.yaml required)
- Copy pod template: `copy-pod-template.yaml`
- Image: `${CP_cp_image}` from env var
- Purpose: Mount PVC and allow artifact extraction

**Key differences:**
- Argo: expected-copy-pod.yaml required, separate pod created
- Volcano: NO expected-copy-pod.yaml, use built-in task pod

## Testing Strategy

### Unit Tests

**File:** `go/cmd/converter/package/cp_artifact_manager_test.go`

**Test cases:**
```go
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
        {"artifacts only", "*.txt", "", false},  // Inline copy mode
    }
    // ...
}

func TestGetArtifactMountPath(t *testing.T) {
    // Test default /output vs specified path
}
```

**File:** `go/cmd/converter/package/convert_script_to_volcano_test.go`

**Test cases:**
```go
func TestAddArtifactVolume(t *testing.T) {
    // Verify container.VolumeMounts appended
    // Verify job.Spec.Volumes appended with VolumeClaim
}

func TestCreateCopyArtifactTask(t *testing.T) {
    // Verify task name is "copy-artifact"
    // Verify container uses alpine image
    // Verify sleep infinity command
}
```

### E2E Tests

**Test cases:** All 3 migrated test cases in `newtest/` directory.

**Test flow:**
1. Run converter with test env.sh
2. Compare generated YAML with expected.yaml
3. Submit Volcano Job to cluster using ~/.kube/006.yaml
4. Wait for job completion
5. Extract artifacts using kubectl exec tar
6. Verify files exist in WORKSPACE with correct content

**Test command:**
```bash
cd go/cmd/converter && go test -v -run Test_main -case test10-cp-artifacts
```

### Integration Tests

**Existing test framework:** `go/cmd/converter/convertv2_to_yaml_test.go`

**Add test cases:**
- Test converter generates multi-task Volcano Job when `CP_artifacts_temp_folder` is specified
- Test converter generates single-task Volcano Job when both parameters empty
- Test inline artifact copy script generation when only `CP_artifacts` specified

## Error Handling

### Converter Errors

**Case 1:** `cpArtifactsTempFolder != ""` but repoName empty
- **Impact:** Cannot determine dataset PVC name (similar to cpDataset error)
- **Action:** Print error message, exit with code 1
- **Message:** `"fail to configure CP_artifacts_temp_folder: missing repo name"`
- **Note:** This check is NOT needed for artifacts (volumeClaim is self-contained)

**Case 2:** Invalid mountPath format
- **Impact:** Volume mount fails in pod
- **Action:** No validation needed (user-provided path is trusted)

**Case 3:** Artifact volume name collision
- **Impact:** VolumeMount name already used (e.g., "dataset", "ascend-driver")
- **Action:** Use unique name "output" (no collision risk)

### Volcano Job Validation

**Requirement 1:** VolumeClaim must have `accessModes` and `storage` requests
- **Validation:** DTO struct ensures required fields are set
- **Default:** `"ReadWriteMany"`, `"1Gi"`

**Requirement 2:** Copy-artifact task must mount same volume as main-script
- **Validation:** Both tasks use `ArtifactVolumeName` constant
- **Guarantee:** Inline helper ensures consistency

**Requirement 3:** Tasks array order must be preserved
- **Validation:** Main task first, copy task second
- **Reason:** Volcano executes tasks in array order

### Runtime Errors

**Case 1:** Copy-artifact task pod not found
- **Reason:** Pod not started yet or label selector mismatch
- **Action:** submit-test waits for task pod creation (max 120s)
- **Label selector:** `volcano.sh/job-name=${WORKFLOW_NAME},volcano.sh/task-name=copy-artifact`

**Case 2:** kubectl exec tar fails
- **Reason:** Task pod not ready, tar command not available, or volume not mounted
- **Action:** Check copy-artifact task uses alpine:3.23.3 (has tar)
- **Validation:** eval.sh validates files in WORKSPACE after submit-test extraction

**Case 3:** Extraction produces empty workspace
- **Reason:** Artifact volume empty or mountPath mismatch
- **Action:** Check Volcano Job created PVC, check both tasks mount same volume
- **Validation:** eval.sh checks directory not empty, exits with code 1

## Backward Compatibility

### Single-Task Mode

**Condition:** `cpArtifacts == ""` AND `cpArtifactsTempFolder == ""`

**Behavior:**
- Converter generates single-task Volcano Job
- No artifact volume added
- Inline artifact copy script (if `cpArtifacts` specified without temp folder)

**Existing test cases:** All test1-test9 continue working unchanged.

### Default Mount Path

**Fallback:** If `cpArtifactsTempFolder == ""` AND `cpArtifacts != ""` → use `/output`

**Implementation:** `GetArtifactMountPath()` returns `/output` when empty.

**Inline copy:** Script handler generates `cp -r --parents {patterns} /output/` in main script.

### No Breaking Changes

**Converter function signature:** No changes to `ConvertScriptToVolcano` parameters.

**VolcanoJob DTO:** Only additions (VolumeClaim, Volumes), no removals.

**Test framework:** E2E test structure unchanged, just add 3 new test cases.

## Performance Considerations

### PVC Creation Overhead

**Impact:** Dynamic PVC creation adds ~2-5s to job startup time.

**Mitigation:** Acceptable for artifact workflows (user scripts typically run minutes/hours).

**Alternative:** Pre-created PVCs (not chosen - requires external management).

### Copy-Artifact Task Resources

**Minimal resources:** 1 CPU, 1Gi memory (sufficient for `sleep infinity`).

**No actual copying:** Task only holds pod alive for external extraction.

**Cleanup:** Volcano's `ttlSecondsAfterFinished: 1800` auto-deletes PVC after 30 minutes.

### Copy-Artifact Task Efficiency

**Sleep command:** Copy-artifact task runs `sleep infinity` to stay alive for external extraction.

**Resource usage:** Minimal (1 CPU, 1Gi memory, just idle sleep).

**Extraction mechanism:** submit-test uses kubectl exec tar on task pod to stream artifacts to WORKSPACE.

**Cleanup:** Volcano Job's `ttlSecondsAfterFinished: 1800` auto-deletes job and PVC after 30 minutes.

## Security Considerations

### Pod Security Context

**RunAsUser: 0:** Required for volume mount permissions.

**Consistency:** Both main-script and copy-artifact tasks use same security context.

### PVC Access Modes

**ReadWriteMany:** Required for shared volume between tasks.

**StorageClass:** `sfsturbo-subpath-sc` (existing cluster configuration).

**No sensitive data:** Artifact volume contains build outputs, not secrets.

### kubectl exec Security

**RBAC requirements:** User must have `pods/exec` permission.

**Namespace scope:** Extraction limited to job namespace.

**No privileged operations:** tar command runs in pod container context.

## Timeline

**Phase 1: DTO and Manager (1 day)**
- Add VolumeClaim to volcano DTO
- Create cp_artifact_manager.go
- Write unit tests for manager functions

**Phase 2: Converter Inline Logic (2 days)**
- Add inline helpers to convert_script_to_volcano.go
- Add multi-task conditional logic
- Write converter unit tests

**Phase 3: Test Case Migration (3 days)**
- Migrate test10-cp-artifacts
- Migrate test12-normal-workflow
- Migrate test13-cp-artifacts-v2
- Update eval.sh scripts

**Phase 4: E2E Testing (2 days)**
- Run all 3 test cases with ~/.kube/006.yaml
- Verify artifact extraction works
- Fix any runtime issues

**Phase 5: Documentation (1 day)**
- Update AGENTS.md with new test cases
- Update README if needed
- Add inline comments explaining artifact logic

**Total: ~9 days**

## Success Criteria

1. ✅ All 3 test cases generate multi-task Volcano Jobs
2. ✅ Volcano Jobs successfully submitted to cluster with ~/.kube/006.yaml
3. ✅ Artifacts extracted with `kubectl exec tar` and prefix stripped
4. ✅ Unit test coverage > 90% for new functions
5. ✅ E2E tests pass for all migrated test cases
6. ✅ Existing test1-test9 continue passing (backward compatibility)
7. ✅ No lint errors (run `.ci/golangci-lint.sh`)
8. ✅ No typo errors (run `.ci/typos.sh`)

## References

- Argo-harbor branch: `origin/argo-harbor`
- Volcano Job CRD: `kubectl --kubeconfig ~/.kube/006.yaml explain job.spec.volumes.volumeClaim`
- Existing patterns: `dataset_manager.go`, `queue_manager.go`, `addDatasetVolume()`
- Mistake notebook: `docs/mistakenotebook/2026-03-30.md`
- Migration design: `docs/superpowers/specs/2026-05-08-volcano-migration-design.md`