# Security Context Feature

## Overview

This feature adds the ability to set pod-level security context with `runAsUser=0` in the converted Argo Workflows. This enables workflows to run containers with root user permissions when needed.

## Feature Specification

### Behavior

- All converted Argo Workflows will include a pod-level `securityContext` with `runAsUser: 0`
- This allows containers to run as root user, which may be required for certain build or installation operations

### Security Context Format

```yaml
spec:
  securityContext:
    runAsUser: 0
```

## Implementation Details

### Modified Files

1. **dto/argo/argo_workflow_yaml.go**
   - Added `SecurityContext` field to `WorkflowSpec` struct
   - Added `SecurityContext` struct with `RunAsUser` field

2. **package/convert_job_to_argo.go**
   - Updated `ConverJobToArgo` function to set `SecurityContext.RunAsUser = 0`
   - Security context is applied to the workflow spec after setting active deadline seconds

## Usage Example

### Expected Output

The converted Argo Workflow will include:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: my-workflow-
spec:
  templates:
    - # template details
  securityContext:
    runAsUser: 0
  # ... rest of the workflow
```

## Testing

### Test Case

| Test Name | Env File | Expected Output |
|-----------|----------|-----------------|
| test8-security-context | case/security-context-env.sh | case/security-context.yaml |

### Run Tests

```bash
cd go/cmd/convertorv2
go test -v -run "test8-security-context"
```

## Security Considerations

- Running containers with `runAsUser: 0` grants root privileges
- Use this feature only when root access is required for the workflow operations
- Consider using more restrictive security contexts when possible
