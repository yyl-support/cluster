# CP_shm Parameter Design

**Date:** 2026-06-09  
**Author:** OpenCode  
**Status:** Design Approved

## Overview

Add support for `CP_shm` environment variable to configure shared memory (`/dev/shm`) as an emptyDir volume with Memory medium in Volcano Jobs.

## Problem Statement

Users need to configure shared memory size for containers running memory-intensive workloads (e.g., PyTorch DataLoader workers, large model inference). Currently, there's no way to configure `/dev/shm` size through environment variables.

## Requirements

1. Parse `CP_shm` environment variable (e.g., `CP_shm=8G`)
2. Convert size format from Kubernetes shorthand ("8G") to standard ("8Gi")
3. Add emptyDir volume with Memory medium to pod spec
4. Mount volume at `/dev/shm` in container
5. Follow existing manager pattern for code organization
6. Add test case to validate implementation

## Architecture

### Data Structure Changes

**File:** `go/cmd/converter/dto/volcano/volcano_job_yaml.go`

Add EmptyDir support to Volume struct:

```go
type Volume struct {
    Name                  string                       `yaml:"name"`
    HostPath              *HostPath                    `yaml:"hostPath,omitempty"`
    PersistentVolumeClaim *PersistentVolumeClaimVolume `yaml:"persistentVolumeClaim,omitempty"`
    EmptyDir              *EmptyDir                    `yaml:"emptyDir,omitempty"`
}

type EmptyDir struct {
    Medium    string `yaml:"medium,omitempty"`
    SizeLimit string `yaml:"sizeLimit,omitempty"`
}
```

### Parameter Handling

**File:** `go/cmd/converter/package/cp_config.go`

Add CP_shm parsing and size normalization:

```go
func GetCPConfig() (..., cpShm string, ...) {
    cpShm = filterCPEnv("CP_shm")
    ...
}

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

### Manager Implementation

**File:** `go/cmd/converter/package/shm_manager.go` (NEW)

Create dedicated manager following existing pattern:

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

### Integration

**File:** `go/cmd/converter/package/convert_script_to_volcano.go`

Call manager after Job struct is populated:

```go
func ConvertScriptToVolcano(..., cpShm string, ...) VolcanoConversionResult {
    ...
    AddShmVolume(&volcanoJob, cpShm)
    ...
}
```

## Data Flow

1. User sets `CP_shm=8G` in env.sh
2. GetCPConfig parses and returns cpShm="8G"
3. ConvertScriptToVolcano passes cpShm to AddShmVolume
4. AddShmVolume normalizes size ("8G" → "8Gi")
5. Manager adds Volume (emptyDir, medium=Memory, sizeLimit=8Gi)
6. Manager adds VolumeMount (name=shm, mountPath=/dev/shm)
7. YAML marshaling includes the volume configuration

## Generated YAML Example

```yaml
spec:
  tasks:
    - template:
        spec:
          volumes:
            - name: shm
              emptyDir:
                medium: Memory
                sizeLimit: "8Gi"
          containers:
            - volumeMounts:
                - name: shm
                  mountPath: /dev/shm
```

## Testing

**Test Directory:** `go/cmd/converter/case/newtest/test25-shm/`

**env.sh:**
```bash
export CP_runs_on="amd64-cpu-1-mem-1G"
export CP_docker_image="swr.cn-southwest-2.myhuaweicloud.com/base_image/ascend-ci/cann:8.2.rc1-910b-ubuntu22.04-py3.11"
export CP_pipeline_run_id="test-shm-123"
export CP_merge_id="25"
export CP_repo_url="https://github.com/testorg/testrepo-test25.git"
export CP_shm="8G"
```

**shell.sh:**
```bash
#!/bin/sh
df -h /dev/shm
echo "shm size test complete"
```

**expected.yaml:** Contains shm volume and mount configuration.

**Validation:**
- Volume exists with name "shm"
- emptyDir.medium = "Memory"
- emptyDir.sizeLimit = "8Gi" (normalized from "8G")
- VolumeMount.mountPath = "/dev/shm"

## Edge Cases

1. **Empty CP_shm:** No volume added, skip silently
2. **Already has "i" suffix:** Pass through unchanged ("8Gi" stays "8Gi")
3. **Various units:** Support G/Gi, M/Mi, K/Ki (add "i" if missing)

## Design Decisions

1. **Manager pattern:** Follows affinity_manager, dataset_manager, image_proxy_manager for consistency
2. **Fixed mount path:** `/dev/shm` is standard, no flexibility needed
3. **Size normalization:** Kubernetes requires "Gi" format, convert shorthand automatically
4. **Memory medium:** Use Memory for tmpfs-backed shm (performance over disk-backed)

## Implementation Complexity

- **Low complexity:** Single manager file, struct extension, simple normalization
- **Estimated LOC:** ~40 lines (manager + normalization)
- **No breaking changes:** Pure addition, doesn't affect existing functionality

## Success Criteria

1. CP_shm parsed correctly from environment
2. Size normalized to Kubernetes format
3. Volume and mount added to YAML
4. Test case passes with correct YAML generation
5. No impact on existing test cases