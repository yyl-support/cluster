package main

import (
	"context"
	"fmt"
	"os"
)

func postExitHandler(cfg Config, podName string, jobName string, cancelLogs context.CancelFunc, logDone chan struct{}, shouldCleanup bool) {
	gracefulStopTail(cancelLogs, logDone)

	logFile := tempLogFilePath(jobName)
	if needsLogFallback(logFile) {
		fmt.Fprintln(os.Stderr, "[日志] 日志文件无有效内容，重新获取...")
		if fetchErr := fetchFinalLogs(cfg, podName); fetchErr != nil {
			fmt.Fprintf(os.Stderr, "[日志] 获取最终日志失败: %v\n", fetchErr)
		}
	}

	if shouldCleanup {
		cleanupCopyArtifactContainer(cfg, podName)
	}
}

func cleanupCopyArtifactContainer(cfg Config, podName string) {
	if cfg.CPArtifactsTempFolder == "" {
		return
	}

	fmt.Println("清理 copy-artifact 容器...")
	_, err := execKubectl(cfg,
		"exec", podName, "-n", cfg.Namespace, "-c", "copy-artifact",
		"--", "sh", "-c", "kill -TERM 1 || true")
	if err != nil {
		fmt.Fprintln(os.Stderr, "WARN: 清理 copy-artifact 容器失败:", err)
	}
}