# Secret Auto-Detection and K8s Secret Management

## Overview

This feature automatically detects sensitive environment variables (passwords, tokens, credentials, etc.) during `convert_to_yaml_v2`, stores them in a Kubernetes Secret, injects them into the Argo Workflow via `secretKeyRef`, and cleans up the Secret after the workflow completes via an on-exit handler.

## Motivation

Currently, sensitive values in `job.env` and `job.steps[].run` scripts are rendered directly into the pipeline YAML as plaintext. This is a security risk. This feature intercepts sensitive values, stores them in a short-lived K8s Secret, and references them securely.

## Sensitive Pattern Matching

### Patterns

Env variable names are matched **case-insensitively** against these substring patterns:

- `password`
- `passwd`
- `token`
- `access`
- `secret`
- `key`
- `credential`

A variable name like `MY_DB_PASSWORD`, `api_token`, `ACCESS_KEY_ID` would all be matched.

### Scan Sources

Sensitive values are collected from three sources:

1. **`job.env` map** — keys matching sensitive patterns have their key-value pairs extracted
2. **`export KEY=VALUE` or `KEY=VALUE` assignments in `job.steps[].run`** — regex scans script bodies for shell variable assignments where the variable name matches a sensitive pattern
3. **`${SENSITIVE_VAR}` references in `job.steps[].run`** — regex scans for `${VAR}` patterns where VAR matches a sensitive pattern; the actual value is resolved from `os.Getenv(VAR)`

## K8s Secret Management

### Secret Creation

- A single K8s Secret is created per job conversion, containing all detected sensitive key-value pairs
- Secret name format: `pipeline-secret-<pipeline_run_id>-<job_name_hash>` (truncated to 63 chars for K8s name limit)
- The **original key names are preserved** as Secret data keys (not renamed)
- The Secret is labeled with:
  ```yaml
  labels:
    pipeline/run-id: <pipeline_run_id>
  ```
- The Secret is created using `k8s.io/client-go` in the same namespace (`argo`)
- The kubeconfig is read from the existing path: `/workspace/workflowtool/k8s-cluster-kubeconfig.yaml` (or from `kubectlfile` env base64-decoded)
- **Secret creation is deferred**: a cleanup function is registered via `defer` to handle errors, but the Secret is created before workflow submission. The `defer` ensures that if `ConverJobToArgo` fails partway, any created Secret is cleaned up.

### Secret Injection

For each sensitive env var extracted, instead of rendering:
```yaml
env:
  - name: MY_PASSWORD
    value: "plaintext-value"
```

The output becomes:
```yaml
env:
  - name: MY_PASSWORD
    valueFrom:
      secretKeyRef:
        name: pipeline-secret-<pipeline_run_id>-<hash>
        key: MY_PASSWORD
```

These `secretKeyRef` env vars are **appended** to the existing `spec.templates.<name>.script.env` list, alongside any non-sensitive env vars that remain as plain `value` entries.

### Script Body Replacement

For sensitive values detected in script bodies (`export KEY=VALUE` or inline `${VAR}`):
- `export KEY=VALUE` lines are removed from the script source
- The value is added to the K8s Secret under key `KEY`
- A `secretKeyRef` env var is injected into the template, making `$KEY` available in the script environment automatically
- `${SENSITIVE_VAR}` references in scripts remain as-is since they'll resolve from the injected env

## On-Exit Cleanup

### Workflow Template Changes

A new on-exit handler is added to the Argo Workflow spec to delete the Secret after workflow completion (success or failure).

#### Argo Struct Changes

Add to `WorkflowSpec`:
```go
OnExit string `yaml:"onExit,omitempty"`
```

Add a new `Resource` type for resource-based templates:
```go
type ResourceTemplate struct {
    Action           string `yaml:"action"`
    Manifest         string `yaml:"manifest"`
    SuccessCondition string `yaml:"successCondition,omitempty"`
    FailureCondition string `yaml:"failureCondition,omitempty"`
}
```

Add to `Template`:
```go
Resource *ResourceTemplate `yaml:"resource,omitempty"`
```

#### On-Exit Template

A new template is appended to `spec.templates`:
```yaml
- name: cleanup-secret
  resource:
    action: delete
    manifest: |
      apiVersion: v1
      kind: Secret
      metadata:
        name: pipeline-secret-<pipeline_run_id>-<hash>
        namespace: argo
```

And `spec.onExit` is set:
```yaml
spec:
  onExit: cleanup-secret
```

This uses Argo's built-in resource template to delete the Secret, requiring appropriate RBAC (ServiceAccount must have `delete` permission on `secrets` in the `argo` namespace).

## Implementation Details

### New Files

| File | Purpose |
|------|---------|
| `package/secret_filter.go` | Detect sensitive env vars from job.env, step scripts |
| `package/secret_manager.go` | K8s Secret CRUD via client-go |

### Modified Files

| File | Changes |
|------|---------|
| `package/convert_job_to_argo.go` | Integrate secret filtering, inject secretKeyRef envs, add on-exit template |
| `package/env_convert.go` | Split env vars into sensitive vs plain |
| `dto/argo/argo_workflow_yaml.go` | Add `OnExit`, `Resource` template fields |
| `go.mod` | Add `k8s.io/client-go` dependency |

### Key Functions

```go
// secret_filter.go
func IsSensitiveEnvName(name string) bool
func FilterSensitiveEnv(env map[string]string) (sensitive, plain map[string]string)
func ExtractSensitiveFromScript(script string) (sensitive map[string]string, cleanedScript string)

// secret_manager.go
func CreatePipelineSecret(secretName, namespace, pipelineRunID string, data map[string]string) error
func DeletePipelineSecret(secretName, namespace string) error
func BuildSecretName(pipelineRunID, jobName string) string
```

### Flow in ConverJobToArgo

```
1. Parse job.env → FilterSensitiveEnv → sensitive map + plain map
2. Build script from steps → ExtractSensitiveFromScript → more sensitive pairs + cleaned script
3. Merge all sensitive pairs
4. If sensitive pairs exist:
   a. Generate secret name: BuildSecretName(pipelineRunID, jobName)
   b. defer CreatePipelineSecret(...)  // deferred execution
   c. Convert sensitive map → []EnvVar with secretKeyRef
   d. Convert plain map → []EnvVar with plain value
   e. Append both to template.Script.Env
   f. Add cleanup-secret template to spec.templates
   g. Set spec.onExit = "cleanup-secret"
5. If no sensitive pairs: behave as before (backward compatible)
```

## RBAC Requirements

The Argo workflow ServiceAccount needs:
```yaml
rules:
  - apiGroups: [""]
    resources: ["secrets"]
    verbs: ["create", "delete", "get"]
```

## Testing

### Test Cases

| Test Name | Description |
|-----------|-------------|
| test-secret-filter-env | job.env with mixed sensitive/plain vars |
| test-secret-filter-script-export | step.run with `export TOKEN=xxx` |
| test-secret-filter-script-ref | step.run with `${PASSWORD}` references |
| test-secret-cleanup-on-exit | on-exit template is generated |
| test-no-secret-passthrough | no sensitive vars → no secret, no on-exit |

## Compatibility

- Backward compatible: workflows without sensitive env vars behave identically
- Requires `client-go` dependency (new)
- Requires RBAC for secret management on the Argo ServiceAccount
