# CP_delay_exit Parameter Design

**Date:** 2026-07-02
**Author:** OpenCode
**Status:** Design Approved

## Overview

Add support for a `CP_delay_exit` environment variable that controls how long a failing job's container sleeps before exiting, giving operators a window to inspect the pod (logs, exec in, etc.) before Volcano tears it down. The delay is injected as a `trap ... EXIT` at the top of every generated container script, always active with a default of 10 seconds.

## Problem Statement

When the user's script exits non-zero, the pod terminates immediately and can disappear before anyone gets a chance to `kubectl exec` into it or capture live state. There's currently no mechanism to hold the pod open briefly on failure. Some workloads may also want a longer (or shorter) delay than the default.

## Requirements

1. Every generated script gets a `trap` on `EXIT` that sleeps N seconds only when the script's exit code is non-zero.
2. Default N = 10 seconds when `CP_delay_exit` is unset or invalid.
3. `CP_delay_exit` (integer seconds) overrides the default when set to a valid non-negative integer.
4. The trap is prepended before the git-clone block, so it covers the entire generated script (clone + user script + artifact copy).
5. `CP_delay_exit` must be excluded from generic pass-through env vars (added to `isConfigEnv()`), matching the pattern used for other `CP_*` config vars.

## Architecture

### Parameter Handling

**File:** `go/cmd/converter/package/cp_config.go`

Add `CP_delay_exit` parsing, following the same permissive-parse-with-default pattern as `CP_timeout`:

```go
func GetCPConfig() (..., cpDelayExitSeconds int) {
    ...
    cpDelayExitStr := filterCPEnv("CP_delay_exit")
    cpDelayExitSeconds = 10 // default
    if cpDelayExitStr != "" {
        if v, err := strconv.Atoi(cpDelayExitStr); err == nil && v >= 0 {
            cpDelayExitSeconds = v
        }
    }
    ...
}
```

Unlike `CP_timeout` (which calls `os.Exit(1)` on a parse failure because timeout is safety-critical), `CP_delay_exit` silently falls back to the default of 10 on empty/invalid/negative input, since it's a minor operational convenience, not a hard requirement.

### Script Injection

**File:** `go/cmd/converter/package/script_handler.go`

`handlerScript()` gains a new `delayExitSeconds int` parameter and prepends the trap line before the git-clone block:

```go
func handlerScript(scriptContent string, req gitRequest, artifactsReq artifactsRequest, delayExitSeconds int) string {
    var sb strings.Builder

    sb.WriteString(buildDelayExitTrap(delayExitSeconds))
    sb.WriteString(req.GenerateGitCloneScript())
    sb.WriteString(scriptContent)

    artifactsScript := artifactsReq.GenerateArtifactsCopyScript()
    if artifactsScript != "" {
        if !strings.HasSuffix(scriptContent, "\n") {
            sb.WriteString("\n")
        }
        sb.WriteString(artifactsScript)
    }

    return sb.String()
}

func buildDelayExitTrap(delayExitSeconds int) string {
    return fmt.Sprintf("trap 'EXIT_CODE=$?; [ \"$EXIT_CODE\" -ne 0 ] && sleep %d' EXIT\n", delayExitSeconds)
}
```

### Integration

**File:** `go/cmd/converter/package/convert_script_to_volcano.go`

`ConvertScriptToVolcano(...)` gains a `cpDelayExitSeconds int` parameter, threaded into the `handlerScript(...)` call:

```go
func ConvertScriptToVolcano(
    ...,
    cpDelayExitSeconds int,
) VolcanoConversionResult {
    ...
    script := handlerScript(
        scriptContent,
        gitRequest{...},
        artifactsReq,
        cpDelayExitSeconds,
    )
    ...
}
```

**File:** `go/cmd/converter/convertv2_to_yaml.go`

Capture the new return value from `GetCPConfig()` and pass it into `ConvertScriptToVolcano(...)`.

### isConfigEnv Whitelist

**File:** `go/cmd/converter/convertv2_to_yaml.go`

Add `"CP_delay_exit"` to the `isConfigEnv()` whitelist map so it's excluded from the generic pass-through env var list (same treatment as `CP_shm`, `CP_bandwidth`, etc.).

## Data Flow

1. User optionally sets `CP_delay_exit=30` in env.sh.
2. `GetCPConfig()` parses and returns `cpDelayExitSeconds` (30, or 10 if unset/invalid).
3. `convertv2_to_yaml.go` passes it to `ConvertScriptToVolcano(...)`.
4. `ConvertScriptToVolcano` passes it to `handlerScript(...)`.
5. `handlerScript` prepends `trap 'EXIT_CODE=$?; [ "$EXIT_CODE" -ne 0 ] && sleep 30' EXIT\n` before the git-clone/user-script/artifact-copy content.
6. The full script becomes `container.Args[0]`.

## Generated Script Example

Given `shell.sh`:
```bash
#!/bin/bash
echo "Hello from test script"
```

Generated `container.Args[0]` (default delay, no `CP_delay_exit` set):
```bash
trap 'EXIT_CODE=$?; [ "$EXIT_CODE" -ne 0 ] && sleep 10' EXIT
#!/bin/bash
echo "Hello from test script"
```

## Testing

### Unit Tests

**File:** `go/cmd/converter/package/cp_config_test.go`
- `CP_delay_exit` unset -> defaults to 10
- `CP_delay_exit="30"` -> returns 30
- `CP_delay_exit="-5"` -> defaults to 10 (negative rejected)
- `CP_delay_exit="abc"` -> defaults to 10 (parse failure)

**File:** `go/cmd/converter/package/script_handler_test.go`
- `handlerScript` output starts with the trap line for a given `delayExitSeconds`
- Trap line correctly reflects a non-default value (e.g. 30)
- Trap is placed before the git-clone script and before the user script content

### E2E Test Case

**Test Directory:** `go/cmd/converter/case/newtest/test37-delay-exit/`

**env.sh:**
```bash
export CP_runs_on="amd64-cpu-1-mem-1G"
export CP_docker_image="swr.cn-southwest-2.myhuaweicloud.com/base_image/python:3.11"
export CP_pipeline_run_id="test-delay-exit-123"
export CP_merge_id="37"
export CP_repo_url="https://github.com/testorg/testrepo-test37.git"
export CP_delay_exit="20"
```

**shell.sh:**
```bash
#!/bin/bash
echo "delay exit test"
```

**expected.yaml:** `args` block begins with
```yaml
args:
  - |
    trap 'EXIT_CODE=$?; [ "$EXIT_CODE" -ne 0 ] && sleep 20' EXIT
    ...
```

Add a second case (or extend an existing simple case, e.g. `test1-simple`) implicitly covering the default-10 path since no existing case sets `CP_delay_exit` — after this change, `test1-simple/expected.yaml`'s `args` block will need the default trap line prepended too. All 36 existing `expected.yaml` files with script content must be regenerated (`go test -run Test_main -regenerate`) and reviewed since this trap is unconditional for every generated script.

Add the new test to the `tests` table in `convertv2_to_yaml_test.go`, and add a corresponding entry to `test-cases.json` per the `add-new-test-case` skill.

## Edge Cases

1. **`CP_delay_exit` unset:** default 10s trap still injected (feature is always-on).
2. **`CP_delay_exit="0"`:** valid, disables the sleep (trap fires but sleeps 0s).
3. **Negative or non-numeric value:** falls back to default of 10, no error/exit.
4. **Script succeeds (exit 0):** trap's condition is false, no sleep occurs, no behavior change.

## Design Decisions

1. **Always-on trap, override via env var:** matches the user's explicit request; ensures the safety net applies to every job without requiring users to opt in.
2. **Silent fallback on invalid input:** consistent with wanting this to be a low-risk operational aid, not a blocking validation like `CP_runs_on`.
3. **Injected via `handlerScript`, not a separate manager file:** unlike volume/annotation features (`AddShmVolume`, `AddBandwidthAnnotation`), this only affects the script text, so it belongs in the existing script-composition function rather than a new manager.
4. **Trap placed first, before git-clone:** ensures failures during git clone (not just the user's script) are also covered by the delay.

## Implementation Complexity

- **Low complexity:** one new small helper function, one new config field, parameter threading through 3 existing functions.
- **Estimated LOC:** ~20 lines of new code + regeneration of ~30 `expected.yaml` fixtures.
- **Breaking change for fixtures only:** every existing E2E test's `expected.yaml` script content changes (gains the trap line), but no behavior/schema change for consumers beyond the script text itself.

## Success Criteria

1. `CP_delay_exit` parsed correctly, defaulting to 10 on unset/invalid input.
2. Generated script always begins with the trap line, sleep duration reflecting the configured value.
3. `isConfigEnv()` excludes `CP_delay_exit` from pass-through env vars.
4. New E2E test case passes; all existing E2E test fixtures regenerated and passing.
5. Unit test coverage for `cp_config.go` parsing and `script_handler.go` trap injection.
