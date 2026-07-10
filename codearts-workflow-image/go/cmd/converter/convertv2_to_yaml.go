package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	converter "github.com/opensourceways/codearts-workflow-image-go/cmd/converter/package"
	"go.yaml.in/yaml/v3"
)

var (
	templatePath string
	outputPath   string
)

func init() {
	flag.StringVar(&templatePath, "t", "case/workflow_templatev2.yaml", "Template YAML path")
	flag.StringVar(&outputPath, "o", "./workflow_trans.yaml", "Output workflow YAML path")
}

func getYamlPaths() (templateYamlPath, targetYamlPath string) {
	templateYamlPath = "case/workflow_templatev2.yaml"
	targetYamlPath = "./workflow_trans.yaml"

	if templatePath != "" {
		templateYamlPath = templatePath
	}
	if outputPath != "" {
		targetYamlPath = outputPath
	}

	return
}

func main() {
	flag.Parse()
	templateYamlPath, targetYamlPath := getYamlPaths()

	workspace := os.Getenv("WORKSPACE")
	if workspace == "" {
		fmt.Println("错误：环境变量 WORKSPACE 未设置")
		os.Exit(1)
	}

	runsOn, dockerImage, pipelineRunID, mergeID, repoURL, targetBranch, cpArtifacts, cpArtifactsTempFolder, cpDataset, _, cpShm, cpBandwidth, cpImagePullPolicy, cpTimeoutSeconds, cpDelayExitSeconds := converter.GetCPConfig()

	jobID := os.Getenv("JOB_ID")
	if jobID == "" {
		fmt.Println("错误：环境变量 JOB_ID 未设置")
		os.Exit(1)
	}

	buildNumber := os.Getenv("BUILDNUMBER")
	if buildNumber == "" {
		fmt.Println("错误：环境变量 BUILDNUMBER 未设置")
		os.Exit(1)
	}

	timestamp := os.Getenv("CP_timestamp")
	if timestamp == "" {
		timestamp = fmt.Sprintf("%d", time.Now().UnixMilli())
	}

	uniqueID := jobID + buildNumber + "-" + timestamp

	root, err := os.OpenRoot(workspace)
	if err != nil {
		fmt.Printf("错误：读取 shell.sh 失败: %v\n", err)
		os.Exit(1) //nolint:gocritic
	}
	defer root.Close() //nolint:errcheck,gocritic

	shellScriptPath := "shell.sh"
	f, err := root.Open(shellScriptPath)
	if err != nil {
		fmt.Printf("错误：读取 shell.sh 失败: %v\n", err)
		os.Exit(1) //nolint:gocritic
	}
	defer f.Close() //nolint:errcheck,gocritic
	scriptContent, err := io.ReadAll(f)
	if err != nil {
		fmt.Printf("错误：读取 shell.sh 失败: %v\n", err)
		os.Exit(1) //nolint:gocritic
	}

	envVars := getAllEnvVars()

	fmt.Printf("读取 shell.sh: %s\n", shellScriptPath)
	fmt.Printf("CP_runs_on: %s\n", runsOn)
	fmt.Printf("CP_docker_image: %s\n", dockerImage)
	fmt.Printf("CP_pipeline_run_id: %s\n", pipelineRunID)
	fmt.Printf("CP_merge_id: %s\n", mergeID)
	fmt.Printf("CP_repo_url: %s\n", repoURL)
	fmt.Printf("环境变量数量: %d\n", len(envVars))

	result := converter.ConvertScriptToVolcano(
		string(scriptContent),
		runsOn,
		dockerImage,
		envVars,
		pipelineRunID,
		mergeID,
		repoURL,
		targetBranch,
		uniqueID,
		templateYamlPath,
		cpDataset,
		cpArtifacts,
		cpArtifactsTempFolder,
		cpTimeoutSeconds,
		cpShm,
		cpBandwidth,
		cpImagePullPolicy,
		cpDelayExitSeconds,
	)

	jobYAML, err := yaml.Marshal(result.Job)
	if err != nil {
		fmt.Println("错误：序列化 YAML 失败")
		os.Exit(1)
	}
	err = os.WriteFile(targetYamlPath, jobYAML, 0644)
	if err != nil {
		fmt.Printf("错误：写入 YAML 文件失败: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("成功生成 YAML 文件: %s\n", targetYamlPath)

	if result.SecretManifest != "" {
		secretFile := strings.TrimSuffix(targetYamlPath, ".yaml") + "-secret.yaml"
		err = os.WriteFile(secretFile, []byte(result.SecretManifest), 0644)
		if err != nil {
			fmt.Printf("错误：写入 Secret YAML 文件失败: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("成功生成 Secret YAML 文件: %s\n", secretFile)
	}
}

func getAllEnvVars() map[string]string {
	envVars := make(map[string]string)
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			key := parts[0]
			value := parts[1]

			if isSystemEnv(key) {
				continue
			}

			if isConfigEnv(key) {
				continue
			}

			envVars[key] = value
		}
	}
	return envVars
}

func isConfigEnv(key string) bool {
	configEnvs := []string{
		"WORKSPACE",
		"workspace",
		"CP_runs_on",
		"CP_docker_image",
		"CP_pipeline_run_id",
		"CP_merge_id",
		"CP_repo_url",
		"CP_target_branch",
		"CP_artifacts",
		"CP_artifacts_temp_folder",
		"CP_dataset",
		"CP_image_proxy",
		"CP_shm",
		"CP_bandwidth",
		"CP_image_pull_policy",
		"CP_delay_exit",
		"workflow_template",
	}

	for _, configEnv := range configEnvs {
		if key == configEnv {
			return true
		}
	}

	return false
}

var (
	exactSystemEnv = map[string]struct{}{
		"SHELL": {}, "LANG": {}, "DISPLAY": {}, "ALL_PROXY": {}, "OPENCODE": {},
		"XDG_SESSION_DESKTOP": {}, "MOZ_ENABLE_WAYLAND": {}, "XDG_RUNTIME_DIR": {},
		"XDG_SESSION_PATH": {}, "XDG_SESSION_ID": {}, "TERM": {}, "DEBUGINFOD_URLS": {},
		"HYPRCURSOR_SIZE": {}, "HOME": {}, "_": {}, "PWD": {}, "HYPRLAND_INSTANCE_SIGNATURE": {},
		"DESKTOP_SESSION": {}, "_JAVA_AWT_WM_NONREPARENTING": {}, "SHLVL": {},
		"XDG_CURRENT_DESKTOP": {}, "XDG_BACKEND": {}, "ALACRITTY_LOG": {}, "PATH": {},
		"MAIL": {}, "XDG_SEAT_PATH": {}, "XDG_VTNR": {}, "XDG_SEAT": {}, "XDG_DATA_DIRS": {},
		"WINDOWID": {}, "PAM_KWALLET5_LOGIN": {}, "ALACRITTY_SOCKET": {}, "COLORTERM": {},
		"DBUS_SESSION_BUS_ADDRESS": {}, "AGENT": {}, "WAYLAND_DISPLAY": {}, "USER": {},
		"XCURSOR_SIZE": {}, "LOGNAME": {}, "ALACRITTY_WINDOW_ID": {}, "HTTP_PROXY": {},
		"all_proxy": {}, "XDG_SESSION_TYPE": {}, "HYPRLAND_CMD": {}, "OLDPWD": {},
		"http_proxy": {}, "https_proxy": {}, "HTTPS_PROXY": {}, "HL_INITIAL_WORKSPACE_TOKEN": {},
		"MOTD_SHOWN": {}, "XDG_SESSION_CLASS": {}, "GOCOVERDIR": {}, "OPENCODE_PID": {},
		"GOPATH": {}, "HISTSIZE": {}, "HISTTIMEFORMAT": {}, "LESSCLOSE": {}, "LESSOPEN": {},
		"LS_COLORS": {}, "SSH_CLIENT": {}, "SSH_CONNECTION": {}, "SSH_TTY": {}, "SSH_ORIGINAL_COMMAND": {},
		"PYTHONPATH": {}, "NODE_PATH": {}, "PERL5LIB": {}, "JAVA_HOME": {}, "Maven_HOME": {},
		"GRADLE_HOME": {}, "CARGO_HOME": {}, "RUSTUP_HOME": {}, "GOROOT": {}, "GOPROXY": {},
		"ENVMAN_LOAD": {}, "HAXE_STD_PATH": {}, "VSSCRIPT_PATH": {},
		"KITTY_PID": {}, "KITTY_WINDOW_ID": {}, "KITTY_INSTALLATION_DIR": {}, "KITTY_PUBLIC_KEY": {},
		"INVOCATION_ID": {}, "JOURNAL_STREAM": {}, "MANAGERPID": {}, "MANAGERPIDFDID": {},
		"KGLOBALACCELD_PLATFORM": {}, "ICEAUTHORITY": {}, "SESSION_MANAGER": {}, "QT_WAYLAND_RECONNECT": {},
		"XAUTHORITY": {}, "KUBECONFIG": {},
		"GTK2_RC_FILES": {}, "NVCC_CCBIN": {}, "TERMINFO": {},
	}

	cusSystemEnv = map[string]struct{}{
		"EXPECTED_EXIT_CODE": {},
		"CP_timestamp":       {},
	}
	systemEnvPrefixes = []string{
		"KDE_", "GTK_", "NVM_", "ALACRITTY_", "XDG_", "CUDA_",
		"MEMORY_PRESSURE_", "SYSTEMD_", "VSCODE_", "GDK_", "ELECTRON_",
		"CHROME_", "GOMODCACHE", "GOTELEMETRY_", "FC_", "NO_AT_",
		"OPENCODE_",
	}
)

func isSystemEnv(key string) bool {
	if _, ok := exactSystemEnv[key]; ok {
		return true
	}
	if _, ok := cusSystemEnv[key]; ok {
		return true
	}

	for _, prefix := range systemEnvPrefixes {
		if strings.HasPrefix(key, prefix) {
			return true
		}
	}
	return false
}
