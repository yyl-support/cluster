# AGENTS.md

## IMPORTANT - Read First

**Before making changes, read `docs/mistakenotebook/` to avoid repeating past mistakes.**

## Project Overview

Converts files + env vars → Volcano Job CRD + K8s secrets

## Input/Output

| Input | Output |
|-------|--------|
| shell.sh | workflow.yaml (Volcano Job CRD) |
| env.sh | workflow-secret.yaml (if secrets) |
| workflow_templatev2.yaml | |

## Testing (TDD)

### End-to-End Tests
- Test: `go/cmd/converter/convertv2_to_yaml_test.go`
- Cases: `go/cmd/converter/case/newtest/*/`
- Each case: env.sh, expected.yaml, shell.sh, workflow_templatev2.yaml

### Unit Tests
- Every new function: `*_test.go` in same package
- Path coverage > 90%

### Volcano Job Tests
- Command: `skill submit-test -k <kubeconfig> -t all`

## Running Tests

```bash
# Unit tests
cd go/cmd/converter && go test -cover ./...

# E2E tests
cd go/cmd/converter && go test -v -run Test_main

# Volcano Job tests
skill submit-test -k <kubeconfig> -t all

# CI checks (typos, golangci-lint)
.ci/typos.sh
.ci/golangci-lint.sh
```

## Environment Variables

- Required/Filtered: see `isConfigEnv()` and `isSystemEnv()` in source
