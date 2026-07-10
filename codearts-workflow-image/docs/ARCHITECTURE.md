# Architecture

## Overview

This project converts CI/CD pipeline configurations into Volcano Job CRDs for Kubernetes execution.

**Goal**: `shell.sh` + `env.sh` + `workflow_templatev2.yaml` → `workflow.yaml` (Volcano Job CRD) + `workflow-secret.yaml` (if secrets exist)

---

## Tree Diagram

```
codearts-workflow-image
│
├─ Phase 1: Parse Input Files
│   ├─ shell.sh              → Script content to execute
│   ├─ env.sh                → Environment variables (CP_* + user vars)
│   ├─ workflow_templatev2.yaml → Base Volcano Job template
│   │
│   └─ Key CP_* Variables:
│       ├─ CP_runs_on          → Arch, CPU, Memory, NPU spec
│       ├─ CP_docker_image     → Container image
│       ├─ CP_repo_url         → Git repository URL
│       ├─ CP_merge_id         → Merge request ID
│       ├─ CP_dataset          → Dataset PVC mount path
│       └─ CP_pipeline_run_id  → Pipeline run identifier
│
├─ Phase 2: Convert to Volcano Job CRD
│   │
│   ├─ go/cmd/converter/package/
│   │   │
│   │   ├─ cp_config.go
│   │   │   └─ GetCPConfig() → Extract CP_* from env
│   │   │
│   │   ├─ secret_filter.go
│   │   │   ├─ IsSensitiveEnvName() → Detect secrets by keyword
│   │   │   ├─ FilterSensitiveEnv() → Split env into sensitive/plain
│   │   │   └─ ResolveSensitiveEnvValues() → Resolve ${VAR} refs
│   │   │
│   │   ├─ script_handler_request.go
│   │   │   ├─ gitRequest.GenerateGitCloneScript()
│   │   │   │   ├─ Detect provider (gitcode, github, gitee, codehub)
│   │   │   │   ├─ Configure git cache CDN URL
│   │   │   │   ├─ Clone repo + checkout branch
│   │   │   │   └─ Merge PR if mergeID exists
│   │   │   │
│   │   │   └─ artifactsRequest.GenerateArtifactsCopyScript()
│   │   │
│   │   ├─ job_arch.go
│   │   │   └─ convertJobArch() → Node selector (kubernetes.io/arch, NPU chip)
│   │   │
│   │   ├─ job_resource.go
│   │   │   └─ convertJobResource() → CPU/Memory/NPU resources
│   │   │       ├─ NPU scaling table (0-8 NPUs → CPU/Memory map)
│   │   │       └─ Default: 2 CPU, 8Gi memory
│   │   │
│   │   ├─ dataset_manager.go
│   │   │   ├─ DatasetManager → PVC claim name mapping
│   │   │   └─ GetDatasetClaimName() → Resolve PVC name
│   │   │
│   │   ├─ secret_manager.go
│   │   │   ├─ BuildSecretName() → SHA256 hash of pipelineRunID+uniqueID
│   │   │   └─ BuildSecretManifest() → K8s Secret YAML
│   │   │
│   │   └─ convert_script_to_volcano.go
│   │       └─ ConvertScriptToVolcano() → Main orchestrator
│   │           ├─ Load template YAML
│   │           ├─ Inject git clone script
│   │           ├─ Set container image
│   │           ├─ Configure resources (CPU/Mem/NPU)
│   │           ├─ Add NPU volumes (ascend-driver hostPath)
│   │           ├─ Add dataset PVC volumes
│   │           ├─ Inject env vars (plain + secretKeyRef)
│   │           ├─ Set labels (pipeline/run-id, jobPRID, jobRepositoryName)
│   │           └─ Generate secret manifest if sensitive vars exist
│   │
│   └─ Output:
│       ├─ workflow.yaml      → Volcano Job CRD
│       └─ workflow-secret.yaml → K8s Secret (if secrets)
│
├─ Phase 3: Submit to Kubernetes
│   │
│   ├─ go/cmd/submit/main.go
│   │   │
│   │   ├─ loadConfig()
│   │   │   └─ Args: --namespace, --work-dir, --workflow-output, --kubeconfig-path
│   │   │
│   │   ├─ namespace/namespace.go
│   │   │   └─ GetNamespaceFromRepoName() → Route to argo or ragsdk namespace
│   │   │
│   │   ├─ pvccluster/pvc_cluster.go
│   │   │   ├─ IsKarmadaCluster() → Detect multi-cluster setup
│   │   │   ├─ ExtractPVCClaimNames() → Find PVCs in workflow
│   │   │   ├─ GetPVCClusterMembers() → Find which cluster has PVC
│   │   │   └─ BuildClusterLabelPatch() → Add dispatch/<cluster>=true label
│   │   │
│   │   ├─ run(cfg)
│   │   │   ├─ kubectl create -f workflow.yaml -n <namespace>
│   │   │   ├─ Apply secret with ownerReference (auto-cleanup)
│   │   │   ├─ Karmada: Wait for ResourceBinding, patch secret label
│   │   │   ├─ Wait for main-script pod
│   │   │   ├─ Stream logs (followLogs)
│   │   │   └─ Wait for pod completion (Succeeded/Failed)
│   │   │
│   │   └─ logs.go
│   │       └─ followLogs() → kubectl logs -f <pod>
│   │
│   └─ Karmada Multi-Cluster Flow:
│       ├─ Submit Volcano Job to control plane
│       ├─ ResourceBinding created → dispatches to member cluster
│       ├─ Wait for binding, get target cluster name
│       ├─ Patch secret with dispatch/<cluster>=true
│       ├─ Secret propagates to same member cluster
│       └─ Pod runs in member cluster
│
├─ Phase 4: Cleanup
│   │
│   └─ TTLSecondsAfterFinished → Auto-delete after completion
│   └─ ownerReference on Secret → Auto-delete with Job
│
└─ Test Infrastructure
    │
    ├─ go/cmd/converter/case/newtest/
    │   ├─ test1-simple/          → Basic script execution
    │   ├─ test2-with-secrets/    → Secret injection + ownerReference
    │   ├─ test3-custom-resources → Custom CPU/Memory
    │   ├─ test8-git-clone/       → Git clone functionality
    │   ├─ test9-910b4/           → NPU 910B4 chip
    │   ├─ test15-dataset/        → Dataset PVC mount
    │   ├─ test17-image-pull-failure → Image pull retry detection
    │   ├─ test21-dataset/        → Dataset mapping
    │   └─ ... (22 test cases total)
    │
    ├─ Test Case Structure:
    │   ├─ env.sh              → CP_* variables
    │   ├─ shell.sh            → Script to execute
    │   ├─ workflow_templatev2.yaml → Custom template (optional)
    │   ├─ expected.yaml       → Expected Volcano Job output
    │   ├─ expected-secret.yaml → Expected Secret output (optional)
    │   └─ eval.sh             → Validation script
    │
    └─ .opencode/skills/submit-test/
        ├─ test-cases.json     → Test definitions
        ├─ scripts/main.sh     → Test orchestrator
        └─ scripts/lib/
            ├─ argo.sh         → Kubectl helpers
            ├─ test-cases.sh   → Test case parsing
            └─ utils.sh        → Logging, colors
```

---

## Detailed Component Breakdown

### 1. Input Files

| File | Purpose | Required |
|------|---------|----------|
| `shell.sh` | User script to execute in container | Yes |
| `env.sh` | Environment variables (CP_* configs) | Yes |
| `workflow_templatev2.yaml` | Base Volcano Job template | Yes |

**CP_* Environment Variables:**

| Variable | Example | Description |
|----------|---------|-------------|
| `CP_runs_on` | `arm64-cpu-4-mem-16G` or `arm64-910b4-2` | Architecture, resources, NPU |
| `CP_docker_image` | `swr.cn-southwest-2.myhuaweicloud.com/modelfoundry/git:latest` | Container image |
| `CP_repo_url` | `https://gitcode.com/org/repo.git` | Git repository |
| `CP_merge_id` | `123` | Merge request ID for PR checkout |
| `CP_target_branch` | `main` | Branch to clone |
| `CP_dataset` | `/dataset` | Dataset PVC mount path |
| `CP_pipeline_run_id` | `abc123` | Pipeline run identifier |

---

### 2. Conversion Process

#### 2.1 Script Processing

```
shell.sh (raw)
    │
    ├─ gitRequest.GenerateGitCloneScript()
    │   ├─ git config url."<cdn>".insteadOf https://gitcode.com
    │   ├─ git clone --single-branch -b <branch> <repo> $WORKSPACE
    │   └─ git fetch + git merge (if mergeID)
    │
    ├─ User script content
    │
    └─ artifactsRequest.GenerateArtifactsCopyScript()
    │   └─ cp -r --parents <patterns> /output/
    │
    └─ Final script → container.args[0]
```

#### 2.2 Environment Processing

```
env.sh (all env vars)
    │
    ├─ Filter by isConfigEnv() → Exclude CP_*, WORKSPACE, etc.
    ├─ Filter by isSystemEnv() → Exclude PATH, HOME, KDE_*, etc.
    │
    ├─ FilterSensitiveEnv()
    │   ├─ IsSensitiveEnvName() → Match: password, token, secret, key, ak, sk
    │   ├─ sensitive: {API_TOKEN, DB_PASSWORD} → Secret
    │   └─ plain: {DEBUG, LOG_LEVEL} → Container env
    │
    ├─ AddCustomEnv() → Inject resource info env vars
    │
    └─ Output:
        ├─ Plain env → container.env (value)
        └─ Sensitive env → secretKeyRef
```

#### 2.3 Resource Calculation

**NPU Scaling Table:**

| NPU Count | CPU Request | Memory Request/Limit |
|-----------|-------------|---------------------|
| 0 | 8 | 16Gi |
| 1 | 12 | 48Gi |
| 2 | 20 | 80Gi |
| 3 | 28 | 112Gi |
| 4 | 36 | 144Gi |
| 5 | 44 | 176Gi |
| 6 | 52 | 208Gi |
| 7 | 60 | 240Gi |
| 8 | 64 | 512Gi |

**CPU/Memory Override:**
- If `CP_runs_on` specifies `cpu-N`, use N/2 as request
- If `CP_runs_on` specifies `mem-XG`, use XGi as request/limit

---

### 3. Volcano Job CRD Structure

```yaml
apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
  generateName: <org-repo>-
  labels:
    pipeline/run-id: <CP_pipeline_run_id>
    jobPRID: <CP_merge_id>
    jobRepositoryName: <org-repo>
spec:
  queue: large-task-shared-queue
  maxRetry: 3
  ttlSecondsAfterFinished: 300
  tasks:
    - name: main-script
      replicas: 1
      template:
        spec:
          nodeSelector:
            kubernetes.io/arch: arm64  # or amd64
            node.kubernetes.io/npu.chip.name: 910B4  # if NPU
          containers:
            - name: main
              image: <CP_docker_image>
              command: [bash, -c]
              args: [<git-clone-script + user-script>]
              resources:
                requests:
                  cpu: "4"
                  memory: "16Gi"
                  huawei.com/ascend-1980: "2"  # if NPU
                limits:
                  memory: "16Gi"
                  huawei.com/ascend-1980: "2"
              env:
                - name: DEBUG
                  value: "true"
                - name: API_TOKEN
                  valueFrom:
                    secretKeyRef:
                      name: pipeline-secret-<hash>
                      key: API_TOKEN
              volumeMounts:
                - name: dataset
                  mountPath: /dataset
                - name: ascend-driver
                  mountPath: /usr/local/Ascend/driver
                  readOnly: true
          volumes:
            - name: dataset
              persistentVolumeClaim:
                claimName: <repo-name>
            - name: ascend-driver
              hostPath:
                path: /usr/local/Ascend/driver
```

---

### 4. Secret Management

```
Sensitive Env Vars
    │
    ├─ BuildSecretName()
    │   └─ SHA256(pipelineRunID + uniqueID)[:16]
    │   └─ Result: pipeline-secret-<id>-<hash>
    │
    ├─ BuildSecretManifest()
    │   └─ Base64 encode values
    │   └─ Set pipeline/run-id label
    │
    └─ Apply with ownerReference
        ├─ apiVersion: batch.volcano.sh/v1alpha1
        ├─ kind: Job
        ├─ name: <job-name>
        ├─ uid: <job-uid>
        ├─ blockOwnerDeletion: true
        │
        └─ Secret auto-deleted when Job deleted
```

---

### 5. Karmada Multi-Cluster Dispatch

```
Submit Workflow
    │
    ├─ kubectl create -f workflow.yaml (control plane)
    │
    ├─ Karmada creates ResourceBinding
    │   ├─ volcano-job → dispatched to member cluster
    │   └─ Binding name: <job-name>-job
    │
    ├─ waitForResourceBinding()
    │   └─ Get target cluster name from binding
    │
    ├─ Patch Secret with dispatch/<cluster>=true
    │   └─ Secret propagates to same member cluster
    │
    └─ Pod runs in member cluster
        ├─ Logs streamed via karmada proxy
        └─ Events fetched via /apis/cluster.karmada.io/.../proxy
```

---

### 6. Namespace Routing

| Repo Name Pattern | Namespace | Purpose |
|-------------------|-----------|---------|
| `*ragsdk*` | `ragsdk` | RAG SDK workloads |
| `*ascendnpu-ir*` | `ragsdk` | Ascend NPU inference |
| `testorg-testrepo-test21` | `ragsdk` | Dataset test case |
| Default | `argo` | General workloads |

---

### 7. Dataset PVC Mapping

```go
defaultDatasetManager = {
    "ascend-op-plugin":          "ascend-op-plugin",
    "ascend-pytorch":           "ascend-op-plugin",
    "ascend-text-embeddings":   "ascend-ragsdk",
    "ascend-ragsdk":            "ascend-ragsdk",
}
```

---

## Test Infrastructure

### Test Categories

| Test | Focus Area |
|------|------------|
| test1-simple | Basic script execution |
| test2-with-secrets | Secret injection, ownerReference validation |
| test3-custom-resources | Custom CPU/Memory limits |
| test4-custom-image | Custom docker image |
| test5-no-merge-id | No PR merge |
| test8-git-clone | Git clone functionality |
| test9-910b4 | NPU 910B4 chip selection |
| test15-dataset | Dataset PVC mount |
| test16-dataset-mapping | Dataset PVC name mapping |
| test17-image-pull-failure | Image pull retry detection |
| test20-ascend-driver | Ascend driver volume mount |
| test21-dataset | Dataset mapping test |
| test22-git-cdn | Multi-CDN git clone |

### Validation Flow

```
eval.sh
    │
    ├─ Wait for Volcano Job completion
    ├─ Fetch pod logs
    │   └─ Verify secret values appear in output
    ├─ Fetch workflow CRD
    │   └─ Verify secretKeyRef exists
    │   └─ Verify resources correct
    │   └─ Verify node selector correct
    ├─ Validate secret ownerReference
    │   └─ Check blockOwnerDeletion: true
    └─ Cleanup test resources
```

---

## Key Files Reference

| Path | Purpose |
|------|---------|
| `go/cmd/converter/convertv2_to_yaml.go` | Main entry point |
| `go/cmd/converter/package/convert_script_to_volcano.go` | Core converter |
| `go/cmd/converter/package/script_handler_request.go` | Git clone script generation |
| `go/cmd/converter/package/secret_filter.go` | Secret detection |
| `go/cmd/converter/package/job_resource.go` | Resource calculation |
| `go/cmd/converter/package/job_arch.go` | Node selector generation |
| `go/cmd/converter/package/dataset_manager.go` | PVC mapping |
| `go/cmd/converter/package/secret_manager.go` | Secret YAML generation |
| `go/cmd/submit/main.go` | Job submission |
| `go/cmd/submit/logs.go` | Log streaming |
| `go/cmd/common/run_on_parser.go` | CP_runs_on parsing |
| `go/cmd/common/pvccluster/pvc_cluster.go` | Karmada PVC routing |
| `go/cmd/common/namespace/namespace.go` | Namespace routing |
| `go/cmd/converter/dto/volcano/volcano_job_yaml.go` | Volcano CRD types |

---

## Run-on Spec Format

```
CP_runs_on: <arch>-<key>-<value>-...

Examples:
- arm64-cpu-4-mem-16G        → 4 CPU cores, 16Gi memory, arm64
- arm64-910b4-2              → 2 NPUs, 910B4 chip, arm64
- amd64-cpu-1-mem-1G         → 1 CPU core, 1Gi memory, amd64
- arm64-910a-8-cpu-64-mem-512G → 8 NPUs, 64 CPU, 512Gi memory
```

---

## Git CDN Configuration

```
Provider         → CDN URL prefix
gitcode.com      → http://git-cache-http-server.git-cache.svc.cluster.local:8080
github.com       → http://git-cache-http-server.git-cache.svc.cluster.local:8080/github
gitee.com        → http://git-cache-http-server.git-cache.svc.cluster.local:8080/gitee
codehub.devcloud → http://git-cache-http-server.git-cache.svc.cluster.local:8080/codehub
```

---

## Error Handling

| Error Type | Detection | Action |
|------------|-----------|--------|
| Image pull failure | `Pulling` event count ≥ 2 | Delete pod, retry |
| Pod pending timeout | No pod after timeout | Print events, fail |
| NPU scheduling failure | Pending > 30s | Print pending events |
| Secret not found | secretKeyRef missing | Fail validation |
| Job failed | Pod phase != Succeeded | Return error |