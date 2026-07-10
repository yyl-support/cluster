# Pipeline Run ID Label Feature

## Overview

This feature adds the ability to label Argo Workflows with a `pipeline/run-id` label during the YAML conversion process in `convert_to_yaml_v2`. This enables better tracking and identification of workflows associated with specific pipeline runs.

## Feature Specification

### Environment Variable

| Variable | Required | Description |
|----------|----------|-------------|
| `pipeline_run_id` | No | The unique identifier for the pipeline run |

### Behavior

- When `pipeline_run_id` environment variable is set, the converted Argo Workflow will include a label `pipeline/run-id` in its metadata
- When `pipeline_run_id` is not set or empty, no labels are added to the workflow metadata

### Label Format

```yaml
metadata:
  labels:
    pipeline/run-id: <value_from_pipeline_run_id_env>
```

## Implementation Details

### Modified Files

1. **dto/argo/argo_workflow_yaml.go**
   - Added `Labels` field to `Metadata` struct to support workflow labels

2. **package/convert_job_to_argo.go**
   - Updated `ConverJobToArgo` function signature to accept `pipelineRunID` parameter
   - Added logic to set `pipeline/run-id` label when `pipelineRunID` is provided

3. **convertv2_to_yaml.go**
   - Added reading of `pipeline_run_id` environment variable
   - Passed `pipelineRunID` to `ConverJobToArgo` function

## Usage Example

### Environment Variables

```bash
export pipeline_run_id="pipeline-12345"
export yaml_job_name="build-job"
export yaml_stage_name="stage1"
export yaml_path="./path/to/workflow.yaml"
export ci_repo_url="https://gitcode.com/org/repo"
```

### Expected Output

The converted Argo Workflow will include:

```yaml
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: my-workflow-
  labels:
    pipeline/run-id: pipeline-12345
spec:
  # ... rest of the workflow
```

## Testing

### Test Case

| Test Name | Env File | Expected Output |
|-----------|----------|-----------------|
| test7-pipeline-run-id | case/pipeline-run-id-env.sh | case/pipeline-run-id.yaml |

### Run Tests

```bash
cd go/cmd/convertorv2
go test -v -run "test7-pipeline-run-id"
```

## Compatibility

- This feature is backward compatible
- Workflows without `pipeline_run_id` set will continue to work as before
- Existing tests pass without modification
