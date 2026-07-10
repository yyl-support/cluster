---
name: parsedspec-guide
description: Guide for using RunOnSpec methods correctly. Use when writing code that handles NPU chip selection, resource mapping, or any logic involving parsedSpec/RunOnSpec. Prevents common mistakes like redundant string checks inside IsNPUEmpty() blocks or mixing string comparisons with method calls.
---

# ParsedSpec (RunOnSpec) Usage Guide

## Key Methods on RunOnSpec

| Method | Condition | Meaning |
|--------|-----------|---------|
| `IsNPUEmpty()` | `NPUChipName == "" && NPUCount == 0` | No NPU specified at all |
| `IsGenericNPU()` | `NPUChipName == "npu"` | Generic NPU placeholder (no specific chip) |
| `Is310P3()` | `NPUChipName == "310P3"` | 310P3 chip specifically |
| `IsCPUEmpty()` | `CPUCoreCount == 0` | No CPU specified |
| `IsMEMEmpty()` | `MemorySize == 0 && MemorySizeUnit == ""` | No memory specified |
| `IsArm64()` | `Arch == "arm64"` | ARM64 architecture |
| `IsAmd64()` | `Arch == "amd64"` | AMD64 architecture |

## Resource Map Selection

Inside `!IsNPUEmpty()` block, the parser ALWAYS sets `NPUChipName` when parsing NPU count. Therefore `NPUChipName` is never empty inside this block.

```
if !parsedSpec.IsNPUEmpty() {
    if parsedSpec.Is310P3() {
        // arm310P3ResourceMap
    } else {
        // arm1980ResourceMap (covers 910 series AND generic "npu")
    }
}
```

## NPU Resource Name

`getNPUResourceName()`:
- Is310P3() → "huawei.com/ascend-310"
- Everything else → "huawei.com/ascend-1980"

## Rules

1. Inside `!IsNPUEmpty()`, `parsedSpec.NPUChipName != ""` is REDUNDANT - the parser always sets chip name
2. Use methods (`Is310P3()`, `IsGenericNPU()`) instead of raw string comparisons like `NPUChipName != "npu"`
3. The else branch after `Is310P3()` covers ALL non-310P3 cases (910A, 910B1-4, generic "npu") - no need for `!IsGenericNPU()` check since resource values are the same