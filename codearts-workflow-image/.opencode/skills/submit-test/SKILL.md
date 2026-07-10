---
name: submit-test
description: A comprehensive test skill for Argo workflows that avoids absolute paths, supports multi-workflow submission, and provides comprehensive output evaluation.
license: MIT
---

# Submit Test Skill

A comprehensive test skill for Argo workflows that avoids absolute paths, supports multi-workflow submission, and provides comprehensive output evaluation.

## Description

This skill orchestrates the testing of Argo Workflows by:
1. Parsing test case configurations
2. Running Go unit tests (optional)
3. Submitting all expected.yaml files as separate workflows (async)
4. Monitoring workflow completion
5. Validating workflow outputs including status, artifacts, and secret yaml changes

## Usage

### Basic Usage

```bash
# Run in interactive mode
skill submit-test

# Run with specific kubeconfig
skill submit-test -k /path/to/kubeconfig

# Run specific test cases
skill submit-test -k /path/to/kubeconfig -t simple,with-secrets

# Run all tests
skill submit-test -k /path/to/kubeconfig -t all

# Skip Go tests
skill submit-test -k /path/to/kubeconfig -t all --skip-go-tests

# Skip workflow submission
skill submit-test -k /path/to/kubeconfig -t all --skip-submit

# Skip result validation
skill submit-test -k /path/to/kubeconfig -t all --skip-validate

# Dry run (preview without submission)
skill submit-test -k /path/to/kubeconfig -t all --dry-run

# List available test cases
skill submit-test --list-tests
```

## Options

- `--kubeconfig, -k`: Path to kubeconfig file (optional, will prompt if not provided)
- `--tests, -t`: Test cases to run (comma-separated, or 'all')
- `--skip-submit`: Skip workflow submission
- `--skip-validate`: Skip result validation
- `--skip-go-tests`: Skip Go unit tests
- `--dry-run`: Preview mode (implies --skip-submit and --skip-validate)
- `--list-tests`: List available test cases
- `--help, -h`: Show help message

## Test Case Configuration

Test cases are defined in `test-cases.json`:

```json
{
    "test-name": {
        "test-dir": "path/to/test/directory",
        "env": "path/to/env/file",
        "shell-script": "path/to/shell/script",
        "expected-yaml": "path/to/expected/yaml",
        "expected-secret": "path/to/expected/secret/yaml",
        "expected-artifact-pvc": "path/to/expected/pvc/yaml",
        "expected-copy-pod": "path/to/copy/pod/yaml",
        "validation": {
            "script": "eval.sh",
            "params": {
                "WORKFLOW_NAME": "${WORKFLOW_NAME}",
                "NAMESPACE": "argo",
                "KUBECONFIG": "${KUBECONFIG}"
            }
        }
    }
}
```

## Features

### Relative Path Architecture
All paths are calculated relative to the script location, avoiding absolute path dependencies.

### Multi-Workflow Submission
Submits all specified expected.yaml files as separate Argo workflows asynchronously and tracks their individual completion status.

### Go Tests Integration
Runs `go test ./cmd/converter/...` to verify conversion logic before submitting workflow tests.

### Copy Pod Support
Retrieves artifacts from completed workflows via temporary copy pods that mount PVCs and copy artifacts to the local workspace.

### Artifact PVC Support
Applies PersistentVolumeClaims with ownerReferences to workflows for automatic cleanup.

### Comprehensive Evaluation
- **Status Validation**: Verifies workflow completion status (Succeeded/Failed)
- **Artifact Validation**: Compares generated artifacts with expected outputs
- **Secret YAML Validation**: Validates secret configurations and changes
- **Custom Eval Scripts**: Runs user-defined validation scripts per test case

### Dry-run Mode
Preview mode that shows what would be processed without actually submitting workflows or running validation.

## Requirements

- `argo` CLI installed and configured
- Valid kubeconfig file with cluster access
- Test cases configured in `test-cases.json`
- Go test files with expected outputs

## Directory Structure

```
submit-test/
├── scripts/
│   ├── main.sh              # Main orchestrator
│   ├── lib/
│   │   ├── utils.sh         # Logging and utilities
│   │   ├── argo.sh          # Argo workflow operations
│   │   └── test-cases.sh    # Test case management
│   ├── test-cases.json      # Test case configuration
│   └── test-suite.sh
├── test-cases.json
└── EVAL_GENERATOR_GUIDE.md
```

## Commands

### argo.sh
Handles Argo workflow operations:
- `argo_submit`: Submits a single workflow
- `argo_wait`: Waits for workflow completion with timeout
- `argo_get_uid`: Gets workflow UID for PVC ownerReferences
- `apply_secret`: Applies Kubernetes secrets
- `apply_pvc`: Applies PersistentVolumeClaims with workflow ownerReferences
- `start_copy_pod`: Starts a temporary pod for artifact retrieval
- `wait_copy_pod`: Waits for copy pod completion
- `run_eval`: Executes custom evaluation scripts

### test-cases.sh
Manages test case parsing and selection:
- `parse_test_cases`: Parses test cases from JSON
- `get_test_case_info`: Gets specific field from test case config
- `select_tests`: Selects tests by name or 'all'
- `list_tests`: Lists available test cases
- `get_cp_artifacts_temp_folder`: Extracts CP_artifacts_temp_folder path from env.sh
- `get_workspace`: Extracts WORKSPACE path from env.sh

### utils.sh
Provides logging and utility functions:
- `log_info`, `log_success`, `log_error`, `log_step`, `log_test`, `log_eval`: Colored logging
- `get_timestamp`: Returns UTC timestamp
- `validate_kubeconfig`: Validates kubeconfig file exists

## Examples

### Interactive Mode
```bash
skill submit-test
# Prompts for kubeconfig path and test selection
```

### Single Test
```bash
skill submit-test -k ~/.kube/config -t simple
```

### Multiple Tests
```bash
skill submit-test -k ~/.kube/config -t "simple,with-secrets,custom-resources"
```

### All Tests
```bash
skill submit-test -k ~/.kube/config -t all
```

### Skip Go Tests
```bash
skill submit-test -k ~/.kube/config -t all --skip-go-tests
```

### Dry Run
```bash
skill submit-test -k ~/.kube/config -t all --dry-run
```

### Validation Only
```bash
skill submit-test -k ~/.kube/config -t all --skip-submit
```

## Troubleshooting

### Common Issues

1. **Kubeconfig Not Found**: Ensure the kubeconfig path is correct
2. **Test Case Missing**: Verify test cases are properly configured in `test-cases.json`
3. **Workflow Submission Failed**: Check Argo server connectivity
4. **Go Tests Failed**: Run `cd go && go test ./cmd/converter/...` to debug
5. **Copy Pod Failed**: Check PVC and artifact paths

### Debugging

- Use `--dry-run` to preview without submission
- Use `--skip-submit` to test without workflow submission
- Use `--skip-validate` to test without validation
- Use `--skip-go-tests` to skip Go unit tests
- Check logs for detailed error messages
- Verify test case configuration in `test-cases.json`
