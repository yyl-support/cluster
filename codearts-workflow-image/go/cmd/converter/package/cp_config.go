package converter

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func GetCPConfig() (runsOn, dockerImage, pipelineRunID, mergeID, repoURL, targetBranch, cpArtifacts, cpArtifactsTempFolder, cpDataset, cpImageProxy, cpShm, cpBandwidth, cpImagePullPolicy string, cpTimeoutSeconds, cpDelayExitSeconds int) {
	runsOn = os.Getenv("CP_runs_on")
	if runsOn == "" {
		fmt.Println("错误：环境变量 CP_runs_on 未设置")
		os.Exit(1)
	}
	dockerImage = filterCPEnv("CP_docker_image")
	pipelineRunID = filterCPEnv("CP_pipeline_run_id")
	mergeID = filterCPEnv("CP_merge_id")
	repoURL = filterCPEnv("CP_repo_url")
	targetBranch = filterCPEnv("CP_target_branch")
	cpArtifacts = filterCPEnv("CP_artifacts")
	cpArtifactsTempFolder = filterCPEnv("CP_artifacts_temp_folder")
	cpDataset = filterCPEnv("CP_dataset")
	cpImageProxy = filterCPEnv("CP_image_proxy")
	cpShm = filterCPEnv("CP_shm")
	cpBandwidth = filterCPEnv("CP_bandwidth")
	cpImagePullPolicy = normalizeImagePullPolicy(filterCPEnv("CP_image_pull_policy"))

	cpTimeoutHours := filterCPEnv("CP_timeout")
	if cpTimeoutHours != "" {
		hours, err := strconv.Atoi(cpTimeoutHours)
		if err != nil {
			fmt.Printf("错误：CP_timeout 解析失败: %v\n", err)
			os.Exit(1)
		}
		cpTimeoutSeconds = hours * 3600
	} else {
		cpTimeoutSeconds = 14400 // default 4 hours
	}

	if cpArtifacts != "" && cpArtifactsTempFolder == "" {
		cpArtifactsTempFolder = "/output"
	}
	if cpImageProxy != "" {
		SetDefaultImageProxyURL(cpImageProxy)
	}

	cpDelayExitSeconds = defaultDelayExitSeconds
	if cpDelayExitStr := filterCPEnv("CP_delay_exit"); cpDelayExitStr != "" {
		if v, err := strconv.Atoi(cpDelayExitStr); err == nil && v >= 0 {
			cpDelayExitSeconds = v
		}
	}
	return
}

// defaultDelayExitSeconds is how long the container sleeps on a non-zero
// exit code when CP_delay_exit is unset or invalid, giving operators time
// to inspect the pod before it terminates.
const defaultDelayExitSeconds = 10

func filterCPEnv(key string) string {
	v := os.Getenv(key)
	if strings.HasPrefix(v, "$") {
		return ""
	}
	return v
}

// normalizeShmSize converts size format by adding "i" suffix if missing
// Supports formats: "8G" → "8Gi", "512M" → "512Mi", "8Gi" → "8Gi" (unchanged)
func normalizeShmSize(size string) string {
	if size == "" {
		return ""
	}
	if strings.HasSuffix(size, "i") {
		return size
	}
	return size + "i"
}

// normalizeImagePullPolicy canonicalizes the imagePullPolicy value to the
// Kubernetes-accepted casing. Unrecognized values are silently dropped (return "")
// to avoid submitting an invalid policy to the API server.
// Supported: ifnotpresent → IfNotPresent, always → Always, never → Never
func normalizeImagePullPolicy(policy string) string {
	switch strings.ToLower(policy) {
	case "ifnotpresent":
		return "IfNotPresent"
	case "always":
		return "Always"
	case "never":
		return "Never"
	default:
		return ""
	}
}
