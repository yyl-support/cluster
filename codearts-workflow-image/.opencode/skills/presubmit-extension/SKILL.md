---
name: presubmit-extension
description: Guide for extending presubmit validation in go/cmd/submit/presubmit.go. Use this skill when adding new validation checks (StorageClass, PVC, Queue, Chip, etc.) to the presubmit workflow validation system. Distinguishes between Karmada (member clusters) and normal K8s cluster validation approaches.
---

# Presubmit Extension Pattern

## Core Pattern

When adding new validation checks to `go/cmd/submit/presubmit.go`, follow this pattern:

1. **Query**: Extract value from job, check if it exists
2. **Karmada vs Normal**: Determine if validation needs member cluster check (Karmada) or single cluster check (Normal)
3. **Replace**: Modify job if validation fails (outside Karmada check)

## Validation Types

### Type 1: Same Function for Both (Queue example)
**Use when**: Resource exists in same location for both Karmada and normal clusters (control plane or single cluster)

```go
queueName := extractQueueName(job)
if queueName != "" {
    if queueExists(cfg, queueName) {
        fmt.Printf("[OK] Queue '%s' exists in cluster\n", queueName)
    } else {
        fmt.Printf("[WARN] Queue '%s' not found, replacing with 'default'\n", queueName)
        job = replaceQueue(job, defaultQueue)
    }
} else {
    fmt.Println("[SKIP] No queue specified in workflow")
}
```

**No `isKarmadaCluster` check needed** - same validation function works for both.

### Type 2: Different Functions (StorageClass example)
**Use when**: Resource location differs - Karmada needs member cluster check, normal needs single cluster check

```go
// StorageClass validation
if hasEphemeralVolume(job) {
    fmt.Printf("[INFO] Checking StorageClasses\n")
    
    scExistsFunc := storageClassExists
    if isKarmadaCluster(cfg) {
        scExistsFunc = storageClassExistsInMemberClusters
    }
    
    if scExistsFunc(cfg, storageClassSFSTurbo) {
        fmt.Printf("[OK] StorageClass '%s' exists\n", storageClassSFSTurbo)
    } else if scExistsFunc(cfg, storageClassCSINAS) {
        fmt.Printf("[WARN] StorageClass '%s' not found, using '%s' instead\n", storageClassSFSTurbo, storageClassCSINAS)
        job = replaceStorageClass(job, storageClassSFSTurbo, storageClassCSINAS)
    } else {
        fmt.Printf("[SKIP] Neither '%s' nor '%s' StorageClass found, keeping original\n", storageClassSFSTurbo, storageClassCSINAS)
    }
} else {
    fmt.Println("[SKIP] No ephemeral volumes in workflow")
}
```

**Pattern**: Set function pointer based on `isKarmadaCluster`, then call same logic.

### Type 3: Different Logic (Chip/PVC example)
**Use when**: Validation approach fundamentally differs between Karmada and normal clusters

```go
chipName := extractChipNameFromJob(job)
if chipName != "" {
    if isKarmadaCluster(cfg) {
        fmt.Printf("[INFO] Checking chip '%s' across member clusters\n", chipName)
        chipCluster, err := getNodeChipClusterMember(cfg, chipName)
        if err != nil {
            return fmt.Errorf("[FAIL] Chip validation failed: %w", err)
        }
        if chipCluster == "" {
            fmt.Printf("[WARN] Chip '%s' found in multiple clusters, will rely on dispatch labels\n", chipName)
        } else {
            fmt.Printf("[OK] Chip '%s' found in cluster '%s'\n", chipName, chipCluster)
        }
    } else {
        if chipNodesExist(cfg, chipName) {
            fmt.Printf("[OK] Nodes with chip '%s' found in cluster\n", chipName)
        } else {
            return fmt.Errorf("[FAIL] No nodes found with chip label '%s', cannot submit workflow", chipName)
        }
    }
} else {
    fmt.Println("[SKIP] No chip-specific nodeSelector in workflow")
}
```

**Different functions**: `getNodeChipClusterMember` for Karmada vs `chipNodesExist` for normal, different error handling.

## Helper Functions Required

### For Type 1 (Same function)
- `extractFieldNameFromJob(job map[string]interface{}) string`
- `fieldExists(cfg Config, fieldName string) bool`
- `replaceField(job map[string]interface{}, oldValue, newValue string) map[string]interface{}`

### For Type 2 (Different functions)
- Normal: `fieldExists(cfg Config, fieldName string) bool`
- Karmada: `fieldExistsInMemberClusters(cfg Config, fieldName string) bool`
- Use `getKarmadaMemberClusters(cfg)` to iterate member clusters
- Use proxy path: `--raw /apis/cluster.karmada.io/v1alpha1/clusters/<cluster>/proxy/apis/...`

### For Type 3 (Different logic)
- Normal: Simple existence check
- Karmada: Complex multi-cluster check with cluster mapping

## Karmada Member Cluster Check Pattern

```go
func fieldExistsInMemberClusters(cfg Config, fieldName string) bool {
    clusters, err := getKarmadaMemberClusters(cfg)
    if err != nil {
        return false
    }

    for _, cluster := range clusters {
        rawPath := fmt.Sprintf("/apis/cluster.karmada.io/v1alpha1/clusters/%s/proxy/apis/<group>/v1/<resource>/<name>", cluster, fieldName)
        output, err := execKubectl(cfg, "get", "--raw", rawPath)
        if err == nil && len(output) > 0 {
            return true
        }
    }

    return false
}
```

## Testing

After adding validation:

1. Run unit tests: `cd go/cmd/submit && go test -v -run TestYourValidation`
2. Run E2E tests: `cd go/cmd/converter && go test -v -run Test_main`
3. Test with both Karmada (`~/.kube/karmada-search-proxy.yaml`) and normal cluster kubeconfigs