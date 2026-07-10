package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func handleCopyArtifact(cfg Config, podName string) error {
	fmt.Println("检测到 artifact Volcano Job，准备从 copy-artifact 容器提取 artifacts...")

	mountPath := cfg.CPArtifactsTempFolder
	if mountPath == "" {
		mountPath = "/output"
	}

	fmt.Printf("开始提取 artifacts 到 %s ...\n", cfg.CPWorkspace)
	if err := os.MkdirAll(cfg.CPWorkspace, 0755); err != nil {
		return fmt.Errorf("创建 WORKSPACE 失败: %w", err)
	}

	tempDir, err := os.MkdirTemp(cfg.CPWorkspace, ".cp-download-")
	if err != nil {
		return fmt.Errorf("创建临时下载目录失败: %w", err)
	}
	defer os.RemoveAll(tempDir)

	if err := extractArtifacts(cfg, podName, mountPath, tempDir); err != nil {
		fmt.Fprintln(os.Stderr, "WARN: 提取 artifacts 失败:", err)
	} else {
		if err := moveTempArtifactsIntoWorkspace(cfg.CPWorkspace, tempDir); err != nil {
			return fmt.Errorf("应用 artifacts 到 WORKSPACE 失败: %w", err)
		}
		fmt.Println("Artifacts 提取成功")
	}

	return nil
}

func extractArtifacts(cfg Config, podName, mountPath, tempDir string) error {
	fmt.Printf("使用 kubectl cp 从 copy-artifact 容器提取 artifacts...\n")

	copySource := fmt.Sprintf("%s/%s:%s", cfg.Namespace, podName, mountPath)
	_, err := execKubectl(cfg, "cp", copySource, tempDir, "-c", "copy-artifact")
	if err != nil {
		return fmt.Errorf("kubectl cp 失败: %w", err)
	}

	return nil
}

func moveTempArtifactsIntoWorkspace(workspace, tempDir string) error {
	tempEntries, err := os.ReadDir(tempDir)
	if err != nil {
		return err
	}

	for _, entry := range tempEntries {
		srcPath := filepath.Join(tempDir, entry.Name())
		dstPath := filepath.Join(workspace, entry.Name())
		os.RemoveAll(dstPath)
		if err := os.Rename(srcPath, dstPath); err != nil {
			return err
		}
	}

	return os.Remove(tempDir)
}

func extendCopyArtifactTimer(cfg Config, podName string) error {
	_, err := execKubectl(cfg, "exec", podName, "-n", cfg.Namespace, "-c", "copy-artifact",
		"--", "touch", "/tmp/reset_timer")
	return err
}

func clearDirectoryContents(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if err := os.RemoveAll(filepath.Join(dir, entry.Name())); err != nil {
			return err
		}
	}

	return nil
}
