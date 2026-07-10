# kubectl cp Command Differences

## Source Path Variations

### 1. With Trailing Slash: `/output/artifact/`

```bash
kubectl cp argo/${WORKFLOW_NAME}-copy-pod:/output/artifact/ ./go/cmd/output/artifact/
```

| Aspect | Behavior |
|--------|----------|
| Meaning | Contents of the directory |
| Result | Copies the **contents** inside `/output/artifact/`, not the directory itself |
| Prerequisite | Destination directory must exist |

**Example:**
```
/output/artifact/
├── test.txt
└── test22222.txt

# Result after copy:
./go/cmd/output/artifact/
├── test.txt
└── test22222.txt
```

---

### 2. Without Trailing Slash: `/output/artifact`

```bash
kubectl cp argo/${WORKFLOW_NAME}-copy-pod:/output/artifact ./go/cmd/output/
```

| Aspect | Behavior |
|--------|----------|
| Meaning | The directory itself |
| Result | Copies the **directory** with all its contents into destination |
| Prerequisite | Destination directory should exist (creates subdirectory) |

**Example:**
```
/output/artifact/
├── test.txt
└── test22222.txt

# Result after copy:
./go/cmd/output/
artifact/           <-- new directory created
├── test.txt
└── test22222.txt
```

---

### 3. Specific File: `/output/artifact/test.txt`

```bash
kubectl cp argo/${WORKFLOW_NAME}-copy-pod:/output/artifact/test.txt ./go/cmd/output/artifact/test.txt
```

| Aspect | Behavior |
|--------|----------|
| Meaning | A single file |
| Result | Copies only the specified file |
| Prerequisite | None (creates parent directories if needed) |

**Example:**
```
# Result after copy:
./go/cmd/output/artifact/test.txt   <-- only this file
```

---

### 4. Directory to File-Style Path: `/output/artifact ./go/cmd/output/artifact`

```bash
kubectl cp argo/${WORKFLOW_NAME}-copy-pod:/output/artifact ./go/cmd/output/artifact
```

| Aspect | Behavior |
|--------|----------|
| Meaning | Ambiguous - directory to path without trailing slash |
| Result | Creates `artifact` as a subdirectory in destination |
| Note | This is essentially same as case #2 |

**Example:**
```
# Result after copy:
./go/cmd/output/artifact/
├── test.txt
└── test22222.txt
```

---

## Summary Table

| Command | Source Meaning | Dest Behavior | Creates Subdir? |
|---------|---------------|---------------|-----------------|
| `...:/artifact/` | Contents only | Copies contents into existing dir | No |
| `...:/artifact` | Directory itself | Copies dir into dest | Yes |
| `...:/artifact/file.txt` | Single file | Copies file to specific path | No (file only) |

---

## Common Use Cases

### Copy contents only (recommended for most cases)
```bash
kubectl cp argo/${WORKFLOW_NAME}-copy-pod:/output/artifact/ ./go/cmd/output/artifact/
```
Requires `./go/cmd/output/artifact/` to exist.

### Copy entire directory
```bash
kubectl cp argo/${WORKFLOW_NAME}-copy-pod:/output/artifact ./go/cmd/output/
```
Creates `./go/cmd/output/artifact/` automatically.

### Copy single file
```bash
kubectl cp argo/${WORKFLOW_NAME}-copy-pod:/output/artifact/test.txt ./go/cmd/output/test.txt
```
Most precise - only copies the specific file.

---

## Notes

- Trailing slash (`/`) matters in `kubectl cp` - it distinguishes between copying directory contents vs the directory itself
- Always ensure destination has enough storage space
- For large files, consider using `pv` or direct volume mounting instead of `kubectl cp`
