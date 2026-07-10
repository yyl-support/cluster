# Update Submit Tool for Volcano Job Migration

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate `go/cmd/submit/main.go` and `logs.go` from Argo Workflow to Volcano Job submission and management.

**Architecture:** Replace Argo CLI commands (`argo submit`, `argo stop`, `argo logs`) with kubectl commands for Volcano Jobs. Remove artifact handling since CopyPod is not supported in Volcano migration.

**Tech Stack:** Go, kubectl, Volcano Jobs (batch.volcano.sh/v1alpha1)

---

## File Structure

**Modify:**
- `go/cmd/submit/main.go` - Replace Argo commands with kubectl for Volcano Jobs
- `go/cmd/submit/logs.go` - Replace `argo logs` with `kubectl logs`

**Delete:**
- `go/cmd/submit/artifacts.go` - Remove artifact handling (CopyPod not supported)

---

### Task 1: Update main.go - Remove Argo Dependency

**Files:**
- Modify: `go/cmd/submit/main.go:94,125-134,167-169,298-301,330`

- [ ] **Step 1: Remove Argo from required commands**

Change line 94 from:
```go
if err := requireCommands("kubectl", "argo", "jq"); err != nil {
```
To:
```go
if err := requireCommands("kubectl", "jq"); err != nil {
```

- [ ] **Step 2: Replace workflow submission with Volcano Job submission**

Change lines 125-134 from:
```go
submitOutput, err := runCommandOutputRetry(cfg, "argo", []string{"submit", cfg.WorkflowOutput, "-n", cfg.Namespace, "-o", "name"}, commandRetryAttempts, commandRetryDelay)
if err != nil {
	return fmt.Errorf("workflow 提交失败: %w", err)
}

workflowName, err := workflowNameFromSubmitOutput(submitOutput)
if err != nil {
	return err
}
fmt.Println("Workflow submitted:", workflowName)
```
To:
```go
submitOutput, err := runCommandOutputRetry(cfg, "kubectl", []string{"create", "-f", cfg.WorkflowOutput, "-n", cfg.Namespace, "-o", "name"}, commandRetryAttempts, commandRetryDelay)
if err != nil {
	return fmt.Errorf("Volcano Job 提交失败: %w", err)
}

jobName, err := jobNameFromSubmitOutput(submitOutput)
if err != nil {
	return err
}
fmt.Println("Volcano Job submitted:", jobName)
```

- [ ] **Step 3: Update workflowNameFromSubmitOutput to jobNameFromSubmitOutput**

Change lines 250-256 from:
```go
func workflowNameFromSubmitOutput(output []byte) (string, error) {
	workflowName := strings.TrimSpace(filepath.Base(string(output)))
	if workflowName == "" || workflowName == "." {
		return "", errors.New("argo submit 未返回 workflow name")
	}
	return workflowName, nil
}
```
To:
```go
func jobNameFromSubmitOutput(output []byte) (string, error) {
	jobName := strings.TrimSpace(string(output))
	if jobName == "" {
		return "", errors.New("kubectl create 未返回 job name")
	}
	jobName = strings.TrimPrefix(jobName, "job.batch.volcano.sh/")
	return jobName, nil
}
```

- [ ] **Step 4: Update variable names in run() function**

Change line 130 from `workflowName` to `jobName` throughout the run() function:
```go
workflowName, err := jobNameFromSubmitOutput(submitOutput)
```
To:
```go
jobName, err := jobNameFromSubmitOutput(submitOutput)
```

Update all references in run() function (lines 135-157, 161-182):
- `workflowName` → `jobName`

- [ ] **Step 5: Replace stopWorkflow with deleteJob**

Change lines 298-301 from:
```go
func stopWorkflow(cfg Config, workflowName string) error {
	_, err := runCommandOutputRetryFunc(cfg, "argo", []string{"stop", workflowName, "-n", cfg.Namespace}, commandRetryAttempts, commandRetryDelay)
	return err
}
```
To:
```go
func deleteJob(cfg Config, jobName string) error {
	_, err := runCommandOutputRetryFunc(cfg, "kubectl", []string{"delete", "job.batch.volcano.sh", jobName, "-n", cfg.Namespace}, commandRetryAttempts, commandRetryDelay)
	return err
}
```

- [ ] **Step 6: Update stopWorkflow call to deleteJob**

Change lines 167-169 from:
```go
if stopErr := stopWorkflow(cfg, workflowName); stopErr != nil {
	fmt.Fprintf(os.Stderr, "停止 Workflow 失败: %v\n", stopErr)
}
```
To:
```go
if stopErr := deleteJob(cfg, jobName); stopErr != nil {
	fmt.Fprintf(os.Stderr, "删除 Job 失败: %v\n", stopErr)
}
```

- [ ] **Step 7: Update pod label selector for Volcano Jobs**

Change line 330 from:
```go
label := fmt.Sprintf("workflows.argoproj.io/workflow=%s", workflowName)
```
To:
```go
label := fmt.Sprintf("volcano.sh/job-name=%s", jobName)
```

Update parameter name throughout the function (line 329, 341, 356, 381):
```go
func waitForPodByLabelContext(ctx context.Context, cfg Config, jobName string, mode podSelectionMode, filter func(string) bool) (string, error) {
```
And update the label reference.

- [ ] **Step 8: Update error messages and comments**

Update all Chinese error messages to reference "Job" instead of "Workflow":
- Line 127: `return fmt.Errorf("Volcano Job 提交失败: %w", err)`
- Line 149: `return fmt.Errorf("未找到 main-script pod: %w", err)`
- Line 166: `fmt.Fprintf(os.Stderr, "检测到镜像拉取失败，正在删除 Job: %s\n", jobName)`
- Line 168: `fmt.Fprintf(os.Stderr, "删除 Job 失败: %v\n", stopErr)`

- [ ] **Step 9: Run tests to verify compilation**

Run: `cd go/cmd/submit && go build`
Expected: Build succeeds without errors

- [ ] **Step 10: Commit changes**

```bash
git add go/cmd/submit/main.go
git commit -m "feat(submit): replace Argo commands with kubectl for Volcano Jobs"
```

---

### Task 2: Update logs.go - Replace argo logs with kubectl logs

**Files:**
- Modify: `go/cmd/submit/logs.go:14,40,58`

- [ ] **Step 1: Replace argo logs command with kubectl logs**

Change lines 13-19 from:
```go
var (
	runArgoLogsCommand = func(ctx context.Context, cfg Config, workflowName string) error {
		cmd := exec.CommandContext(ctx, "argo", "logs", workflowName, "-n", cfg.Namespace, "--follow", "--no-color")
		cmd.Env = commandEnv(cfg)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
```
To:
```go
var (
	runKubectlLogsCommand = func(ctx context.Context, cfg Config, podName string) error {
		cmd := exec.CommandContext(ctx, "kubectl", "logs", podName, "-n", cfg.Namespace, "-f", "--all-containers")
		cmd.Env = commandEnv(cfg)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
```

- [ ] **Step 2: Update followLogs to use kubectl logs with pod name**

Change function signature and implementation (lines 31-64):
```go
func followLogs(ctx context.Context, cfg Config, jobName string) {
	retryCount := 0
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		fmt.Fprintln(os.Stderr, "[日志] 等待主 Pod 出现...")
		mainPod, err := waitForPodByLabelContext(
			ctx,
			cfg,
			jobName,
			selectOldest,
			func(name string) bool {
				return strings.Contains(name, "main-script") || strings.Contains(name, jobName)
			},
		)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			fmt.Fprintln(os.Stderr, "[日志] 获取 Pod 失败，继续重试...")
			time.Sleep(2 * time.Second)
			continue
		}

		fmt.Fprintf(os.Stderr, "[日志] 连接 kubectl logs %s ...\n", mainPod)
		err = runKubectlLogsCommand(ctx, cfg, mainPod)
		if err == nil || ctx.Err() != nil {
			return
		}

		running, checkErr := isPodRunningOrPending(cfg, mainPod)
		if checkErr != nil {
			if errors.Is(checkErr, context.Canceled) {
				return
			}
			fmt.Fprintln(os.Stderr, "[日志] 获取 pod 状态失败，继续重试...")
		} else if !running {
			fmt.Fprintln(os.Stderr, "[日志] 主 Pod 已结束，停止日志跟随")
			return
		}

		retryCount++
		fmt.Fprintf(os.Stderr, "[日志] kubectl logs 断开，%s 后重试...\n", logReconnectDelay)

		if err := waitForLogReconnectDelay(ctx, retryDelayForAttempt(logReconnectDelay, retryCount)); err != nil {
			return
		}
	}
}
```

- [ ] **Step 3: Add strings import**

Add `"strings"` to imports at line 4.

- [ ] **Step 4: Run tests to verify compilation**

Run: `cd go/cmd/submit && go build`
Expected: Build succeeds without errors

- [ ] **Step 5: Commit changes**

```bash
git add go/cmd/submit/logs.go
git commit -m "feat(submit): replace argo logs with kubectl logs for Volcano Jobs"
```

---

### Task 3: Remove artifacts.go - Delete artifact handling

**Files:**
- Delete: `go/cmd/submit/artifacts.go`
- Modify: `go/cmd/submit/main.go:58-59,179-183`

- [ ] **Step 1: Remove artifact-related Config fields**

Delete lines 58-59 in main.go:
```go
	CPArtifacts           string
	CPArtifactsTempFolder string
```

Delete lines 203-205 in main.go (flag definitions):
```go
	fs.StringVar(&defaults.CPArtifacts, "cp-artifacts", defaults.CPArtifacts, "Artifact copy marker")
	fs.StringVar(&defaults.CPArtifactsTempFolder, "cp-artifacts-temp-folder", defaults.CPArtifactsTempFolder, "Artifact temp folder in pod")
	fs.StringVar(&defaults.CPWorkspace, "cp-workspace", defaults.CPWorkspace, "Workspace path for copying artifacts")
```

- [ ] **Step 2: Remove artifact validation**

Delete lines 216-218 in main.go:
```go
if cfg.CPArtifactsTempFolder != "" && cfg.CPWorkspace == "" {
	return errors.New("cp_workspace 不能为空")
}
```

- [ ] **Step 3: Remove artifact handling in run()**

Delete lines 179-183 in main.go:
```go
if cfg.CPArtifactsTempFolder != "" {
	if err := handleArtifacts(cfg, workflowName); err != nil {
		return err
	}
}
```

- [ ] **Step 4: Remove CPWorkspace field**

Delete line 60:
```go
CPWorkspace           string
```

Delete line 206:
```go
fs.StringVar(&defaults.CPWorkspace, "cp-workspace", defaults.CPWorkspace, "Workspace path for copying artifacts")
```

- [ ] **Step 5: Delete artifacts.go file**

Run: `rm go/cmd/submit/artifacts.go`

- [ ] **Step 6: Run tests to verify compilation**

Run: `cd go/cmd/submit && go build`
Expected: Build succeeds without errors

- [ ] **Step 7: Commit changes**

```bash
git add -A
git commit -m "refactor(submit): remove artifact handling (CopyPod not supported in Volcano)"
```

---

### Task 4: Update main.go - Fix variable references and function calls

**Files:**
- Modify: `go/cmd/submit/main.go`

- [ ] **Step 1: Update followLogs call to match new signature**

Change line 157 from:
```go
followLogs(logCtx, cfg, workflowName)
```
To:
```go
followLogs(logCtx, cfg, jobName)
```

- [ ] **Step 2: Update waitForPodByLabel call in run()**

Change line 140-147 from:
```go
mainPod, err := waitForPodByLabel(
	cfg,
	workflowName,
	selectOldest,
	func(name string) bool {
		return strings.Contains(name, "main-script") || name == workflowName
	},
)
```
To:
```go
mainPod, err := waitForPodByLabel(
	cfg,
	jobName,
	selectOldest,
	func(name string) bool {
		return strings.Contains(name, "main-script") || strings.Contains(name, jobName)
	},
)
```

- [ ] **Step 3: Update writeWorkflowNameFile calls**

Change line 135-137 from:
```go
if err := writeWorkflowNameFile(cfg.WorkflowNameFile, workflowName); err != nil {
	return err
}
```
To:
```go
if err := writeWorkflowNameFile(cfg.WorkflowNameFile, jobName); err != nil {
	return err
}
```

- [ ] **Step 4: Run tests to verify compilation**

Run: `cd go/cmd/submit && go build`
Expected: Build succeeds without errors

- [ ] **Step 5: Commit changes**

```bash
git add go/cmd/submit/main.go
git commit -m "fix(submit): update variable references for Volcano Job migration"
```

---

### Task 5: Update isWorkflowMainPodRunning functions

**Files:**
- Modify: `go/cmd/submit/main.go:446-465`

- [ ] **Step 1: Rename isWorkflowMainPodRunning to isJobMainPodRunning**

Change lines 446-465 from:
```go
func isWorkflowMainPodRunning(cfg Config, workflowName string) (bool, error) {
	return isWorkflowMainPodRunningContext(context.Background(), cfg, workflowName)
}

func isWorkflowMainPodRunningContext(ctx context.Context, cfg Config, workflowName string) (bool, error) {
	mainPod, err := waitForPodByLabelContext(
		ctx,
		cfg,
		workflowName,
		selectOldest,
		func(name string) bool {
			return strings.Contains(name, "main-script") || name == workflowName
		},
	)
	if err != nil {
		return false, err
	}

	return isPodRunningOrPending(cfg, mainPod)
}
```
To:
```go
func isJobMainPodRunning(cfg Config, jobName string) (bool, error) {
	return isJobMainPodRunningContext(context.Background(), cfg, jobName)
}

func isJobMainPodRunningContext(ctx context.Context, cfg Config, jobName string) (bool, error) {
	mainPod, err := waitForPodByLabelContext(
		ctx,
		cfg,
		jobName,
		selectOldest,
		func(name string) bool {
			return strings.Contains(name, "main-script") || strings.Contains(name, jobName)
		},
	)
	if err != nil {
		return false, err
	}

	return isPodRunningOrPending(cfg, mainPod)
}
```

- [ ] **Step 2: Update checkWorkflowMainPodRunning variable in logs.go**

Change line 20 in logs.go from:
```go
checkWorkflowMainPodRunning = isWorkflowMainPodRunningContext
```
To:
```go
checkJobMainPodRunning = isJobMainPodRunningContext
```

Update reference in logs.go function (line 46).

- [ ] **Step 3: Run tests to verify compilation**

Run: `cd go/cmd/submit && go build`
Expected: Build succeeds without errors

- [ ] **Step 4: Commit changes**

```bash
git add go/cmd/submit/main.go go/cmd/submit/logs.go
git commit -m "refactor(submit): rename workflow functions to job functions"
```

---

### Task 6: Final verification and testing

**Files:**
- Test: Manual cluster test

- [ ] **Step 1: Build the submit binary**

Run: `cd go/cmd/submit && go build -o submit`
Expected: Binary created successfully

- [ ] **Step 2: Run unit tests (if any exist)**

Run: `cd go/cmd/submit && go test -v ./...`
Expected: All tests pass (or no tests to run)

- [ ] **Step 3: Test submit with Volcano Job**

Create a test Volcano Job YAML and test submission:
```bash
./submit \
  --namespace default \
  --work-dir /tmp/test \
  --workflow-output /tmp/test/workflow.yaml \
  --kubeconfig-path ~/.kube/config
```
Expected: Job submitted successfully, logs streamed correctly, cleanup works

- [ ] **Step 4: Commit final changes**

```bash
git add -A
git commit -m "test(submit): verify Volcano Job submission works"
```

---

## Summary

**Changes:**
1. Replaced `argo submit` with `kubectl create -f`
2. Replaced `argo stop` with `kubectl delete job.batch.volcano.sh`
3. Replaced `argo logs` with `kubectl logs -f`
4. Updated pod label selector from `workflows.argoproj.io/workflow` to `volcano.sh/job-name`
5. Removed artifact handling (artifacts.go deleted)
6. Renamed all workflow-related functions and variables to job-related names

**Breaking Changes:**
- Removed `--cp-artifacts`, `--cp-artifacts-temp-folder`, `--cp-workspace` flags
- Volcano Jobs do not support abort/stop without deletion (must delete)

**Testing:**
- Build verification after each task
- Manual cluster test with real Volcano Job submission