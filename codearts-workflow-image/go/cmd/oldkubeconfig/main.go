package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	workspace := os.Getenv("WORKSPACE")
	if workspace == "" {
		fmt.Fprintln(os.Stderr, "错误：环境变量 WORKSPACE 未设置")
		os.Exit(1)
	}

	kubeconfigKeyFile, err := findKubeconfigKeyFile(workspace)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	// 只输出路径到 stdout
	fmt.Println(kubeconfigKeyFile)
}

// findKubeconfigKeyFile 查找 kubeconfig 密钥文件
func findKubeconfigKeyFile(workspace string) (string, error) {
	// 优先检查 WORKSPACE/kubeconfig.key
	defaultPath := filepath.Join(workspace, "kubeconfig.key")
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath, nil
	}

	// 若不存在，则查找 WORKSPACE 下匹配 kubeconfig_*.key 的文件
	pattern := filepath.Join(workspace, "kubeconfig_*.key")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return "", fmt.Errorf("查找文件失败: %w", err)
	}
	if len(matches) == 0 {
		return "", fmt.Errorf("错误：未找到任何 kubeconfig_*.key 文件")
	}

	// 取第一个匹配的文件
	return matches[0], nil
}
