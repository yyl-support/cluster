package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/opensourceways/codearts-workflow-image-go/cmd/common/namespace"
	"github.com/opensourceways/codearts-workflow-image-go/cmd/kubeconfig/kubectl"
)

type imagePullError struct {
	ImageName string
	PullCount int
}

func (e *imagePullError) Error() string {
	return fmt.Sprintf("镜像拉取失败: image=%s, pull次数=%d", e.ImageName, e.PullCount)
}

func isImagePullError(err error) bool {
	var target *imagePullError
	return errors.As(err, &target)
}

const (
	defaultNamespace      = "argo"
	defaultWorkDir        = "/workspace"
	defaultWorkflowOutput = "./workflowtool/workflow.yaml"
	defaultKubeconfigPath = "/workspace/workflowtool/k8s-cluster-kubeconfig.yaml"
	defaultImageProxyURL  = "harbor-portal.osinfra.cn"

	commandRetryAttempts = 5
	commandRetryDelay    = 10 * time.Second
	logReconnectDelay    = 10 * time.Second
)

type podSelectionMode int

const (
	selectOldest podSelectionMode = iota
	selectNewest
)

type Config struct {
	Namespace             string
	WorkDir               string
	WorkflowOutput        string
	SecretFile            string
	KubeconfigPath        string
	WorkflowNameFile      string
	ImageProxyURL         string
	CPArtifactsTempFolder string
	CPArtifacts           string
	CPWorkspace           string
	MaxPendingChecks      int // Max iterations before checking events (default: 3)
	ImagePullThreshold    int // Number of pull attempts before exit (default: 2, configurable to 6 or any value)
}

type PodList struct {
	Items []PodItem `json:"items"`
}

type PodItem struct {
	Metadata PodMetadata `json:"metadata"`
}

type PodMetadata struct {
	Name              string    `json:"name"`
	CreationTimestamp time.Time `json:"creationTimestamp"`
}

var execKubectlWithContext = func(ctx context.Context, cfg Config, args ...string) ([]byte, error) {
	executor := &kubectl.RealExecutor{Kubeconfig: cfg.KubeconfigPath}
	return kubectl.ExecWithRetry(ctx, executor, args, kubectl.DefaultRetryConfig())
}

func main() {
	cfg, err := loadConfig(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}

	if err := validateConfig(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}

	if err := requireCommands("kubectl", "jq"); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}

	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}

	fmt.Println("工作流处理完成")
}

func run(cfg Config) error {
	if err := os.Chdir(cfg.WorkDir); err != nil {
		return fmt.Errorf("切换到工作目录失败: %w", err)
	}

	secretFile := cfg.SecretFile
	hasSecret := false
	if secretFile == "" {
		secretFile = strings.TrimSuffix(cfg.WorkflowOutput, ".yaml") + "-secret.yaml"
	}
	if _, err := os.Stat(secretFile); err == nil {
		hasSecret = true
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("检查 secret 文件失败: %w", err)
	}

	repoName, err := namespace.GetRepoNameFromWorkflow(cfg.WorkflowOutput)
	if err != nil {
		fmt.Fprintf(os.Stderr, "WARN: Could not read repo name from workflow: %v\n", err)
	} else if repoName != "" {
		mappedNamespace := namespace.GetNamespaceFromRepoName(repoName)
		if mappedNamespace != cfg.Namespace {
			fmt.Printf("Mapping repo '%s' to namespace '%s'\n", repoName, mappedNamespace)
			cfg.Namespace = mappedNamespace
		}
	}

	validatedWorkflow, err := preSubmitValidate(cfg, cfg.WorkflowOutput)
	if err != nil {
		return fmt.Errorf("Pre-submit validation failed: %w", err)
	}

	modifiedWorkflow, err := addClusterDispatchLabels(cfg, validatedWorkflow)
	if err != nil {
		return fmt.Errorf("Failed to add cluster dispatch labels: %w", err)
	}

	submitOutput, err := runCommandOutputRetry(cfg, "kubectl", []string{"create", "-f", modifiedWorkflow, "-n", cfg.Namespace, "-o", "json"}, commandRetryAttempts, commandRetryDelay)
	if err != nil {
		return fmt.Errorf("Volcano Job 提交失败: %w", err)
	}

	jobName, jobUID, err := jobInfoFromSubmitOutput(submitOutput)
	if err != nil {
		return err
	}
	fmt.Println("Volcano Job submitted:", jobName)

	if hasSecret {
		fmt.Println("Applying secret with ownerReference:", secretFile)
		if err := applySecretWithOwner(cfg, secretFile, jobName, jobUID); err != nil {
			return fmt.Errorf("apply secret 失败: %w", err)
		}
	}
	if err := writeWorkflowNameFile(cfg.WorkflowNameFile, jobName); err != nil {
		return err
	}

	fmt.Println("等待主 Pod (main-script) 出现...")
	mainPod, err := waitForPodByLabel(
		cfg,
		jobName,
		selectOldest,
		func(name string) bool {
			return strings.Contains(name, "main-script") || name == jobName
		},
	)
	if err != nil {
		return fmt.Errorf("未找到 main-script pod: %w", err)
	}
	fmt.Printf("Job %s 的主 Pod: %s\n", jobName, mainPod)

	logCtx, cancelLogs := context.WithCancel(context.Background())
	logDone := make(chan struct{})
	go func() {
		defer close(logDone)
		followLogs(logCtx, cfg, jobName)
	}()

	// Wait for pod to leave Pending phase (detect image pull errors early)
	fmt.Printf("等待 Job %s 的 Pod 启动...\n", jobName)
	if err := waitForPodToLeavePending(cfg, mainPod); err != nil {
		gracefulStopTail(cancelLogs, logDone)
		if isImagePullError(err) {
			fmt.Fprintf(os.Stderr, "检测到镜像拉取失败，正在删除 Volcano Job: %s (防止 Pod 重建)\n", jobName)
			// cleanupCopyArtifactContainer(cfg, jobName)
			if deleteErr := deleteJob(cfg, jobName); deleteErr != nil {
				fmt.Fprintf(os.Stderr, "删除 Job 失败: %v\n", deleteErr)
			}
			// Also delete the current pod if it still exists
			// deletePod(cfg, mainPod)
		}
		return err
	}
	fmt.Printf("Job %s 的 Pod 已启动\n", jobName)
	// Cleanup and log handling on all exit paths
	defer postExitHandler(cfg, mainPod, jobName, cancelLogs, logDone, true)

	// Start periodic timer extension for copy-artifact container
	timerCtx, cancelTimer := context.WithCancel(context.Background())
	defer cancelTimer()
	if cfg.CPArtifactsTempFolder != "" {
		go func() {
			ticker := time.NewTicker(5 * time.Minute)
			defer ticker.Stop()
			for {
				select {
				case <-timerCtx.Done():
					return
				case <-ticker.C:
					if err := extendCopyArtifactTimer(cfg, mainPod); err != nil {
						fmt.Fprintf(os.Stderr, "WARN: 延长 copy-artifact 计时器失败: %v\n", err)
					}
				}
			}
		}()
	}

	// Wait for main container (ascend) to terminate
	fmt.Printf("等待 Job %s 的主容器 (ascend) 完成...\n", jobName)
	containerID, _ := getContainerID(cfg, mainPod, "ascend")
	if shortID := shortContainerID(containerID); shortID != "" {
		fmt.Printf("Container ID: %s\n", shortID)
	}
	if err := waitForContainerTermination(cfg, mainPod, "ascend"); err != nil {
		return fmt.Errorf("Job %s 主容器未完成: %w", jobName, err)
	}
	// fmt.Printf("Job %s 的主容器完成\n", jobName)

	// Check main container exit code (not pod phase - copy-artifact keeps pod Running)
	ascendExitCode, err := getContainerExitCode(cfg, mainPod, "ascend")
	if err != nil {
		return fmt.Errorf("Job %s 获取容器退出码失败: %w", jobName, err)
	}

	// Main container failed - cleanup copy-artifact before returning error
	if ascendExitCode != 0 {
		return formatContainerFailureError(cfg, mainPod, jobName, "ascend", ascendExitCode)
	}

	// Success path - extract artifacts (cleanup happens inside handleCopyArtifact too, but defer ensures it)
	if cfg.CPArtifactsTempFolder != "" {
		if err := handleCopyArtifact(cfg, mainPod); err != nil {
			return fmt.Errorf("Job %s 提取 artifacts 失败: %w", jobName, err)
		}
	}

	fmt.Printf("Job %s 成功完成\n", jobName)

	return nil
}

func loadConfig(args []string) (Config, error) {
	imageProxyURL := defaultImageProxyURL
	if envProxy := os.Getenv("CP_image_proxy"); envProxy != "" {
		imageProxyURL = envProxy
	}

	defaults := Config{
		Namespace:      defaultNamespace,
		WorkDir:        defaultWorkDir,
		WorkflowOutput: defaultWorkflowOutput,
		KubeconfigPath: defaultKubeconfigPath,
		ImageProxyURL:  imageProxyURL,
	}

	fs := flag.NewFlagSet("submit", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	fs.StringVar(&defaults.Namespace, "namespace", defaults.Namespace, "Kubernetes namespace")
	fs.StringVar(&defaults.WorkDir, "work-dir", defaults.WorkDir, "Working directory")
	fs.StringVar(&defaults.WorkflowOutput, "workflow-output", defaults.WorkflowOutput, "Workflow YAML path")
	fs.StringVar(&defaults.SecretFile, "secret-file", defaults.SecretFile, "Secret manifest path")
	fs.StringVar(&defaults.KubeconfigPath, "kubeconfig-path", defaults.KubeconfigPath, "Kubeconfig path")
	fs.StringVar(&defaults.WorkflowNameFile, "workflow-name-file", defaults.WorkflowNameFile, "Write workflow name to file")
	fs.StringVar(&defaults.ImageProxyURL, "image-proxy-url", defaults.ImageProxyURL, "Image proxy URL (e.g., harbor-portal.osinfra.cn). Can also be set via CP_image_proxy env var")
	fs.StringVar(&defaults.CPArtifacts, "cp-artifacts", defaults.CPArtifacts, "Artifact copy marker")
	fs.StringVar(&defaults.CPArtifactsTempFolder, "cp-artifacts-temp-folder", defaults.CPArtifactsTempFolder, "Artifacts temp folder in pod")
	fs.StringVar(&defaults.CPWorkspace, "cp-workspace", defaults.CPWorkspace, "Workspace path for copying artifacts")
	fs.IntVar(&defaults.MaxPendingChecks, "max-pending-checks", 3, "Max iterations before checking image pull events")
	fs.IntVar(&defaults.ImagePullThreshold, "image-pull-threshold", 2, "Number of pull attempts before exit (e.g., 6 for 6 attempts)")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	// Set defaults after parsing flags
	if defaults.CPArtifactsTempFolder == "" && defaults.CPArtifacts != "" {
		defaults.CPArtifactsTempFolder = "/output"
	}

	return defaults, nil
}

func validateConfig(cfg Config) error {
	if cfg.CPArtifactsTempFolder != "" && cfg.CPWorkspace == "" {
		return errors.New("cp_workspace 不能为空")
	}
	if cfg.WorkDir == "" {
		return errors.New("submit_workdir 不能为空")
	}
	if cfg.WorkflowOutput == "" {
		return errors.New("workflow_output 不能为空")
	}
	if cfg.KubeconfigPath == "" {
		return errors.New("kubeconfig_path 不能为空")
	}
	return nil
}

func writeWorkflowNameFile(path, workflowName string) error {
	if path == "" {
		return nil
	}
	if err := os.WriteFile(path, []byte(workflowName), 0o644); err != nil {
		return fmt.Errorf("写入 workflow name 文件失败: %w", err)
	}
	return nil
}

func requireCommands(commands ...string) error {
	for _, cmd := range commands {
		if _, err := exec.LookPath(cmd); err != nil {
			return fmt.Errorf("缺少依赖命令: %s", cmd)
		}
	}
	return nil
}

func jobInfoFromSubmitOutput(output []byte) (string, string, error) {
	var job struct {
		Metadata struct {
			Name string `json:"name"`
			UID  string `json:"uid"`
		} `json:"metadata"`
	}

	if err := json.Unmarshal(output, &job); err != nil {
		return "", "", fmt.Errorf("解析 job 输出失败: %w", err)
	}

	if job.Metadata.Name == "" {
		return "", "", errors.New("未能从输出中提取 job name")
	}
	if job.Metadata.UID == "" {
		return "", "", errors.New("未能从输出中提取 job uid")
	}

	return job.Metadata.Name, job.Metadata.UID, nil
}

func applySecretWithOwner(cfg Config, secretFile, jobName, jobUID string) error {
	secretName, err := getSecretNameFromFile(secretFile)
	if err != nil {
		return err
	}

	if _, err := execKubectl(cfg, "apply", "-f", secretFile, "-n", cfg.Namespace); err != nil {
		return err
	}

	if isKarmadaCluster(cfg) {
		fmt.Println("检测到 Karmada 集群，等待 ResourceBinding...")
		targetCluster, err := waitForResourceBinding(cfg, jobName, 120*time.Minute)
		if err != nil {
			return fmt.Errorf("获取 ResourceBinding 失败: %w", err)
		}
		fmt.Printf("ResourceBinding 已就绪，目标集群: %s\n", targetCluster)

		labelPatch := fmt.Sprintf(`{"metadata":{"labels":{"dispatch/%s":"true"}}}`, targetCluster)
		if _, err := execKubectl(cfg, "patch", "secret", secretName, "-n", cfg.Namespace, "-p", labelPatch); err != nil {
			return fmt.Errorf("patch secret label 失败: %w", err)
		}
		fmt.Printf("Secret %s patched with label dispatch/%s=true\n", secretName, targetCluster)
	}

	ownerRef := fmt.Sprintf(`{
		"apiVersion": "batch.volcano.sh/v1alpha1",
		"kind": "Job",
		"name": "%s",
		"uid": "%s",
		"blockOwnerDeletion": true
	}`, jobName, jobUID)

	patchArgs := []string{
		"patch", "secret", secretName,
		"-n", cfg.Namespace,
		"-p", fmt.Sprintf(`{"metadata":{"ownerReferences":[%s]}}`, ownerRef),
	}

	_, err = execKubectl(cfg, patchArgs...)
	return err
}

func getSecretNameFromFile(secretFile string) (string, error) {
	data, err := os.ReadFile(secretFile)
	if err != nil {
		return "", fmt.Errorf("读取 secret 文件失败: %w", err)
	}

	lines := strings.Split(string(data), "\n")
	inMetadata := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "metadata:" {
			inMetadata = true
			continue
		}

		if inMetadata && strings.HasPrefix(trimmed, "name:") {
			name := strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
			return name, nil
		}

		if inMetadata && trimmed != "" && !strings.HasPrefix(trimmed, " ") && !strings.HasPrefix(trimmed, "namespace:") && !strings.HasPrefix(trimmed, "labels:") && !strings.HasPrefix(trimmed, "creationTimestamp:") && trimmed != "metadata:" {
			inMetadata = false
		}
	}

	return "", errors.New("未能从 secret 文件中提取 name")
}

const karmadaResourceBindingCRD = "resourcebindings.work.karmada.io"

func isKarmadaCluster(cfg Config) bool {
	_, err := execKubectl(cfg, "get", "crd", karmadaResourceBindingCRD)
	return err == nil
}

func waitForResourceBinding(cfg Config, jobName string, timeout time.Duration) (string, error) {
	bindingName := jobName + "-job"
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("等待 ResourceBinding %s 超时 (%v)", bindingName, timeout)
		default:
		}

		output, err := execKubectl(cfg, "get", "resourcebinding", bindingName, "-n", cfg.Namespace, "-o", "jsonpath={.spec.clusters[0].name}")
		if err == nil && len(output) > 0 {
			cluster := strings.TrimSpace(string(output))
			if cluster != "" {
				return cluster, nil
			}
		}

		time.Sleep(2 * time.Second)
	}
}

func execKubectl(cfg Config, args ...string) ([]byte, error) {
	return execKubectlWithContext(context.Background(), cfg, args...)
}

func commandEnv(cfg Config) []string {
	env := os.Environ()
	if cfg.KubeconfigPath == "" {
		return env
	}
	filtered := make([]string, 0, len(env)+1)
	for _, entry := range env {
		if !strings.HasPrefix(entry, "KUBECONFIG=") {
			filtered = append(filtered, entry)
		}
	}
	return append(filtered, "KUBECONFIG="+cfg.KubeconfigPath)
}

func fetchFinalLogs(cfg Config, podName string) error {
	cmd := exec.Command("kubectl", "logs", podName, "-n", cfg.Namespace, "-c", "ascend")
	cmd.Env = commandEnv(cfg)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func needsLogFallback(logFile string) bool {
	info, err := os.Stat(logFile)
	if err != nil || info.Size() == 0 {
		return true
	}
	if info.Size() < 100 {
		return true
	}
	content, err := os.ReadFile(logFile)
	if err != nil {
		return true
	}

	lines := strings.Split(string(content), "\n")
	validLogLines := 0
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "time=") && strings.Contains(line, "level=") {
			continue
		}
		validLogLines++
	}

	return validLogLines == 0
}

var runCommandOutputFunc = func(cfg Config, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = commandEnv(cfg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%s %s failed: %w; output=%s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

func runCommandOutput(cfg Config, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Env = commandEnv(cfg)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%s %s 执行失败: %w; output=%s", name, strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return output, nil
}

var runCommandOutputRetryFunc = runCommandOutputRetry

func deletePod(cfg Config, podName string) error {
	_, err := execKubectl(cfg, "delete", "pod", podName, "-n", cfg.Namespace, "--ignore-not-found=true")
	return err
}

func deleteJob(cfg Config, jobName string) error {
	_, err := runCommandOutputRetryFunc(cfg, "kubectl", []string{"delete", "job.batch.volcano.sh", jobName, "-n", cfg.Namespace}, commandRetryAttempts, commandRetryDelay)
	return err
}

func runCommandOutputRetry(cfg Config, name string, args []string, attempts int, delay time.Duration) ([]byte, error) {
	var lastErr error
	var lastOutput []byte

	for attempt := 1; attempt <= attempts; attempt++ {
		output, err := runCommandOutput(cfg, name, args...)
		if err == nil {
			return output, nil
		}

		lastErr = err
		lastOutput = output
		fmt.Fprintf(os.Stderr, "[重试 %d/%d] %s %s 失败\n", attempt, attempts, name, strings.Join(args, " "))

		if attempt < attempts {
			time.Sleep(delay)
		}
	}

	return lastOutput, fmt.Errorf("%s 重试 %d 次后仍然失败: %w", name, attempts, lastErr)
}

func waitForPodByLabel(cfg Config, jobName string, mode podSelectionMode, filter func(string) bool) (string, error) {
	return waitForPodByLabelContextFunc(context.Background(), cfg, jobName, mode, filter)
}

var waitForPodByLabelContextFunc = waitForPodByLabelContext

func waitForPodByLabelContext(ctx context.Context, cfg Config, jobName string, mode podSelectionMode, filter func(string) bool) (string, error) {
	label := fmt.Sprintf("volcano.sh/job-name=%s", jobName)
	lastPrintTime := time.Now().Add(-5 * time.Minute)
	printInterval := 5 * time.Minute

	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
		}

		output, err := execKubectl(cfg, "get", "pods", "-n", cfg.Namespace, "-l", label, "-o", "json")
		if err != nil {
			if time.Since(lastPrintTime) >= printInterval {
				fmt.Fprintln(os.Stderr, "  获取 pods 列表失败，重试...")
				lastPrintTime = time.Now()
			}
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(commandRetryDelay):
			}
			continue
		}

		var podList PodList
		if err := json.Unmarshal(output, &podList); err != nil {
			return "", fmt.Errorf("解析 pod 列表失败: %w", err)
		}

		if len(podList.Items) == 0 {
			if time.Since(lastPrintTime) >= printInterval {
				fmt.Fprintf(os.Stderr, "  未找到任何匹配 label '%s' 的 pod\n", label)
				lastPrintTime = time.Now()
			}
		} else {
			sort.Slice(podList.Items, func(i, j int) bool {
				return podList.Items[i].Metadata.CreationTimestamp.Before(podList.Items[j].Metadata.CreationTimestamp)
			})

			index := 0
			if mode == selectNewest {
				index = len(podList.Items) - 1
			}

			name := podList.Items[index].Metadata.Name
			if filter == nil || filter(name) {
				return name, nil
			}
		}

		if time.Since(lastPrintTime) >= printInterval {
			fmt.Fprintln(os.Stderr, "  等待符合条件的 pod 出现...")
			lastPrintTime = time.Now()
		}
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}
}

func getPodPhase(cfg Config, podName string) (string, error) {

	output, err := execKubectl(cfg, "get", "pod", podName, "-n", cfg.Namespace, "-o", "jsonpath={.status.phase}")
	if err != nil {
		return "", err
	}
	phase := strings.TrimSpace(string(output))
	return phase, nil
}

func getContainerState(cfg Config, podName, containerName string) (string, error) {
	output, err := execKubectl(cfg, "get", "pod", podName, "-n", cfg.Namespace,
		"-o", "jsonpath={.status.containerStatuses[?(@.name==\""+containerName+"\")].state}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func getContainerID(cfg Config, podName, containerName string) (string, error) {
	output, err := execKubectl(cfg, "get", "pod", podName, "-n", cfg.Namespace,
		"-o", "jsonpath={.status.containerStatuses[?(@.name==\""+containerName+"\")].containerID}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func shortContainerID(raw string) string {
	if idx := strings.LastIndex(raw, "://"); idx != -1 {
		raw = raw[idx+3:]
	}
	if len(raw) > 12 {
		raw = raw[:12]
	}
	return raw
}

func waitForPodToLeavePending(cfg Config, podName string) error {
	pendingCount := 0
	threshold := cfg.ImagePullThreshold
	if threshold <= 0 {
		threshold = 6
	}

	maxChecks := cfg.MaxPendingChecks
	if maxChecks <= 0 {
		maxChecks = 3
	}

	for {
		phase, err := getPodPhaseWithRetry(cfg, podName)
		if err != nil {
			if strings.Contains(err.Error(), "NotFound") {
				return fmt.Errorf("Pod %s 在 Pending 状态被删除 (可能 Volcano Job 已终止)", podName)
			}
			return err
		}

		if phase != "Pending" {
			return nil
		}

		pendingCount++

		// Only check events after maxChecks iterations
		if pendingCount >= maxChecks {
			pullInfo, pullErr := getImagePullEventCountFunc(context.Background(), cfg, podName)
			if pullErr == nil && pullInfo.Count >= threshold && pullInfo.ImageName != "" {
				// Exit ONLY when event count reaches threshold (e.g., 6 pull attempts)
				return &imagePullError{ImageName: pullInfo.ImageName, PullCount: pullInfo.Count}
			}
		}

		time.Sleep(10 * time.Second)
	}
}

func extractImageNameFromMessage(message string) string {
	parts := strings.Split(message, "\"")
	for i, part := range parts {
		if strings.Contains(part, "swr.cn-southwest-2.myhuaweicloud.com") || strings.Contains(part, "harbor") {
			if i+1 < len(parts) {
				return part
			}
		}
	}
	return ""
}

func waitForContainerTermination(cfg Config, podName, containerName string) error {
	for {
		state, err := getContainerState(cfg, podName, containerName)
		if err != nil {
			return err
		}

		if strings.Contains(state, "terminated") {
			return nil
		}

		time.Sleep(10 * time.Second)
	}
}

func interruptContainer(cfg Config, podName, containerName string) error {
	_, err := execKubectl(cfg, "exec", podName, "-n", cfg.Namespace, "-c", containerName,
		"--", "sh", "-c", "kill -TERM 1 || true")
	return err
}

func getContainerExitCode(cfg Config, podName, containerName string) (int, error) {
	output, err := execKubectl(cfg, "get", "pod", podName, "-n", cfg.Namespace,
		"-o", "jsonpath={.status.containerStatuses[?(@.name==\""+containerName+"\")].state.terminated.exitCode}")
	if err != nil {
		return -1, err
	}

	if len(output) == 0 {
		return -1, fmt.Errorf("container %s not terminated", containerName)
	}

	exitCodeStr := strings.TrimSpace(string(output))
	exitCode := 0
	if exitCodeStr != "" {
		if _, err := fmt.Sscanf(exitCodeStr, "%d", &exitCode); err != nil {
			return -1, fmt.Errorf("parse exit code failed: %w", err)
		}
	}

	return exitCode, nil
}

func getContainerTerminatedReason(cfg Config, podName, containerName string) (string, error) {
	output, err := execKubectl(cfg, "get", "pod", podName, "-n", cfg.Namespace,
		"-o", "jsonpath={.status.containerStatuses[?(@.name==\""+containerName+"\")].state.terminated.reason}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func getContainerTerminatedMessage(cfg Config, podName, containerName string) (string, error) {
	output, err := execKubectl(cfg, "get", "pod", podName, "-n", cfg.Namespace,
		"-o", "jsonpath={.status.containerStatuses[?(@.name==\""+containerName+"\")].state.terminated.message}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

func formatContainerFailureError(cfg Config, podName, jobName, containerName string, exitCode int) error {
	reason, _ := getContainerTerminatedReason(cfg, podName, containerName)
	message, _ := getContainerTerminatedMessage(cfg, podName, containerName)

	if reason != "" {
		errStr := fmt.Sprintf("Job %s 容器 %s 失败 (exit code: %d, reason: %s", jobName, containerName, exitCode, reason)
		if message != "" {
			errStr += ", message: " + message
		}
		errStr += ")"
		return fmt.Errorf("%s", errStr)
	}

	return fmt.Errorf("Job %s 容器 %s 失败 (exit code: %d)", jobName, containerName, exitCode)
}

var getPodPhaseFunc = getPodPhase

func getPodPhaseWithRetry(cfg Config, podName string) (string, error) {
	retryIntervals := []time.Duration{20 * time.Second, 40 * time.Second, 80 * time.Second}

	phase, err := getPodPhaseFunc(cfg, podName)
	if err != nil {
		return "", err
	}

	for i, interval := range retryIntervals {
		if phase != "" {
			return phase, nil
		}

		fmt.Fprintf(os.Stderr, "[DEBUG] getPodPhaseWithRetry: phase empty (attempt %d/%d), retrying after %v\n", i+1, len(retryIntervals), interval)
		time.Sleep(interval)

		phase, err = getPodPhaseFunc(cfg, podName)
		if err != nil {
			return "", err
		}
	}

	return phase, nil
}

var getImagePullEventCountFunc = getImagePullEventCount

type imagePullInfo struct {
	Count     int    `json:"count"`
	ImageName string `json:"image"`
}

func getEventsFromMemberCluster(ctx context.Context, cfg Config, memberCluster, namespace, podName string) ([]byte, error) {
	rawPath := fmt.Sprintf("/apis/cluster.karmada.io/v1alpha1/clusters/%s/proxy/api/v1/namespaces/%s/events?fieldSelector=involvedObject.name=%s",
		memberCluster, namespace, podName)

	args := []string{
		"get", "--raw", rawPath,
	}

	output, err := execKubectlWithContext(ctx, cfg, args...)
	if err != nil {
		return nil, fmt.Errorf("get events from member cluster %s failed: %w", memberCluster, err)
	}

	return output, nil
}

func extractJobNameFromPodName(podName string) string {
	idx := strings.Index(podName, "-main-script-")
	if idx > 0 {
		return podName[:idx]
	}
	return podName
}

func getImagePullEventCount(ctx context.Context, cfg Config, podName string) (imagePullInfo, error) {
	var output []byte
	var err error

	if isKarmadaCluster(cfg) {
		jobName := extractJobNameFromPodName(podName)
		bindingName := jobName + "-job"

		clusterOutput, clusterErr := execKubectlWithContext(ctx, cfg, "get", "resourcebinding", bindingName, "-n", cfg.Namespace, "-o", "jsonpath={.spec.clusters[0].name}")
		if clusterErr != nil {
			return imagePullInfo{}, fmt.Errorf("get ResourceBinding %s failed: %w", bindingName, clusterErr)
		}

		memberCluster := strings.TrimSpace(string(clusterOutput))
		if memberCluster == "" {
			return imagePullInfo{}, nil
		}

		output, err = getEventsFromMemberCluster(ctx, cfg, memberCluster, cfg.Namespace, podName)
	} else {
		args := []string{
			"get", "events",
			"-n", cfg.Namespace,
			"--field-selector", "involvedObject.name=" + podName,
			"-o", "json",
		}
		output, err = execKubectlWithContext(ctx, cfg, args...)
	}

	if err != nil {
		return imagePullInfo{}, fmt.Errorf("get events failed: %w", err)
	}

	jqArgs := []string{
		`[.items[] | select(.reason == "Pulling" and (.message | type == "string") and (.message | contains("Pulling image"))) | {count, image: (.message | capture("Pulling image \"(?<img>[^\"]+)\"") | .img)}] | .[0] // empty`,
	}

	cmd := exec.CommandContext(ctx, "jq", jqArgs...)
	cmd.Stdin = bytes.NewReader(output)
	jqOutput, err := cmd.Output()
	if err != nil {
		return imagePullInfo{}, fmt.Errorf("jq parse failed: %w", err)
	}

	jqOutputStr := strings.TrimSpace(string(jqOutput))
	if jqOutputStr == "" || jqOutputStr == "null" {
		return imagePullInfo{}, nil
	}

	var result imagePullInfo
	if err := json.Unmarshal([]byte(jqOutputStr), &result); err != nil {
		return imagePullInfo{}, fmt.Errorf("parse result failed: %w", err)
	}

	return result, nil
}

func isPodRunningOrPending(cfg Config, podName string) (bool, error) {
	phase, err := getPodPhase(cfg, podName)
	if err != nil {
		return false, err
	}
	return phase == "Running" || phase == "Pending", nil
}

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

type podEvent struct {
	Reason  string
	Message string
	Count   int
}

func getPodEvents(ctx context.Context, cfg Config, podName string) ([]podEvent, error) {
	args := []string{
		"get", "events",
		"-n", cfg.Namespace,
		"--field-selector", "involvedObject.name=" + podName,
		"-o", "json",
	}

	output, err := execKubectlWithContext(ctx, cfg, args...)
	if err != nil {
		return nil, fmt.Errorf("get events failed: %w", err)
	}

	jqArgs := []string{
		`[.items[] | {reason: .reason, message: .message, count: .count}] | sort_by(.count) | reverse`,
	}

	cmd := exec.CommandContext(ctx, "jq", jqArgs...)
	cmd.Stdin = bytes.NewReader(output)
	jqOutput, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("jq parse failed: %w", err)
	}

	jqOutputStr := strings.TrimSpace(string(jqOutput))
	if jqOutputStr == "" || jqOutputStr == "null" || jqOutputStr == "[]" {
		return nil, nil
	}

	var events []podEvent
	if err := json.Unmarshal([]byte(jqOutputStr), &events); err != nil {
		return nil, fmt.Errorf("parse events failed: %w", err)
	}

	return events, nil
}

func printPendingEvents(cfg Config, podName string) {
	events, err := getPodEventsFunc(context.Background(), cfg, podName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[WARN] Failed to get pod events: %v\n", err)
		return
	}

	if len(events) == 0 {
		fmt.Fprintf(os.Stderr, "[INFO] Pod %s is Pending, no events found\n", podName)
		return
	}

	fmt.Fprintf(os.Stderr, "[INFO] Pod %s is Pending, recent events:\n", podName)
	for _, e := range events {
		fmt.Fprintf(os.Stderr, "  - %s (count: %d): %s\n", e.Reason, e.Count, e.Message)
	}
}

var getPodEventsFunc = getPodEvents

func waitForPodCompletion(cfg Config, podName string) (string, error) {
	phase, err := getPodPhaseWithRetry(cfg, podName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[DEBUG] waitForPodCompletion: initial getPodPhase error: %v\n", err)
		phase = ""
	}

	pendingCount := 0
	lastPrintCheck := 0
	const maxPendingChecks = 3
	const printIntervalChecks = 30

	for phase == "Pending" {
		pendingCount++

		if pendingCount >= maxPendingChecks {
			pullInfo, pullErr := getImagePullEventCountFunc(context.Background(), cfg, podName)
			if pullErr != nil {
				fmt.Fprintf(os.Stderr, "[DEBUG] waitForPodCompletion: getImagePullEventCount error: %v\n", pullErr)
			} else if pullInfo.Count >= 6 && pullInfo.ImageName != "" {
				return "", &imagePullError{ImageName: pullInfo.ImageName, PullCount: pullInfo.Count}
			}

			if lastPrintCheck == 0 || pendingCount-lastPrintCheck >= printIntervalChecks {
				// printPendingEvents(cfg, podName)
				lastPrintCheck = pendingCount
			}
		}

		time.Sleep(10 * time.Second)
		phase, err = getPodPhaseWithRetry(cfg, podName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[DEBUG] waitForPodCompletion: Pending loop getPodPhase error: %v\n", err)
			phase = ""
		}
	}

	for phase == "Running" {
		time.Sleep(10 * time.Second)
		phase, err = getPodPhaseWithRetry(cfg, podName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[DEBUG] waitForPodCompletion: Running loop getPodPhase error: %v\n", err)
			phase = ""
		}
	}

	return phase, nil
}
