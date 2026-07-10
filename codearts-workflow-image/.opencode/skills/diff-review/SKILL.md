---
name: diff-review
description: Review git diff changes and check if they affect existing functions. Use when making code changes to ensure old function behavior is preserved. Run after any edit to verify no unintended side effects on callers or related logic.
---

# Diff Review Skill

Review code changes against existing functions to catch unintended side effects.

## When to Use

- After editing any file in the codebase
- Before committing changes
- When modifying shared functions, maps, or constants

## Process

### Step 1: Get the diff

```bash
git diff
```

### Step 2: For each changed function/variable, check:

1. **Who calls it?** Search for all callers of the changed function
2. **Does the change alter return values or behavior?** If yes, every caller must be updated
3. **Are tests updated?** Every affected test case must reflect the new behavior

### Step 3: Check callers

For each changed function name, search:

```
grep for function name across all .go files
```

Verify each caller still works with the new behavior.

### Step 4: Run all affected tests

```bash
cd go/cmd/converter && go test ./package/...
```

## Example

Changed `arm1980ResourceMap` values from `{1: {24, 189}}` to `{1: {12, 48}}`:

- Search callers: `queue_manager.go` uses `arm1980ResourceMap` in `getCPUCount()`
- `getCPUCount()` returns `res.cpu` from the map
- Old: 1 NPU → 24 cores returned
- New: 1 NPU → 12 cores returned
- This changes queue determination logic!
- Must update queue tests too

## Checklist

- [ ] git diff reviewed for affected functions
- [ ] All callers of changed functions identified
- [ ] Callers verified to work with new behavior
- [ ] Tests updated for changed behavior
- [ ] All tests pass