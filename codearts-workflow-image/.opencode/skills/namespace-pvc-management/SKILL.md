# Skill: Namespace & PVC Management

Guide for adding new namespaces and PVC records. Use when asked to add a new namespace routing, register a PVC, or both.

## Adding a New Namespace

### Step 1: Add constant in `go/cmd/common/namespace/namespace.go`

```go
const (
    DefaultNamespace        = "argo"
    RagsdkNamespace         = "ragsdk"
    OpPluginNamespace       = "op-plugin"
    RecsdkNamespace         = "recsdk"
    MultimodalsdkNamespace  = "ascend-multimodalsdk"   // <-- add yours here
)
```

### Step 2: Add routing logic in `GetNamespaceFromRepoName`

```go
} else if strings.Contains(repoNameLower, "ascend-multimodalsdk") {
    return MultimodalsdkNamespace
}
```

Place in alphabetical-ish order, before the final `return DefaultNamespace`.

### Step 3: Add test cases in `go/cmd/common/namespace/namespace_test.go`

Add to `TestGetNamespaceFromRepoName`:

```go
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
```

### Step 4: Run tests

```bash
cd go/cmd/common && go test ./namespace/... -v -run "TestGetNamespaceFromRepoName"
```

### Step 5: Commit

Include `namespace.go`, `namespace_test.go`, and optionally PVC config.

## Adding a PVC Record

### Step 1: Edit `go/cmd/converter/pvcrecord/pvc-config.yaml`

Add a new entry under `pvc_records:` at the end:

```yaml
  - name: ascend-<your-name>
    storage: <size>Gi
    storageClassName: sfsturbo-subpath-sc
    namespace: <namespace>
    accessModes:
      - ReadWriteMany
    context: external-gy-001
```

Clusters (contexts): `external-gy-001`, `external-gy-006`, `external-wlcb-001`

### Step 2: (Optional) Apply to cluster

```bash
bash go/cmd/converter/pvcrecord/apply-pvc.sh --context external-gy-001
```

Use `--dry-run` to preview without applying:
```bash
bash go/cmd/converter/pvcrecord/apply-pvc.sh --context external-gy-001 --dry-run
```

### Step 3: Commit

Commit `pvc-config.yaml`.

## Verification

After applying, check PVCs on the cluster:

```bash
kubectl --kubeconfig ~/.kube/a-merge-cluster --context external-gy-001 get pvc -n <namespace>
```
