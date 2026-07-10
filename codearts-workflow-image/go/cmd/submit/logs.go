package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

var (
	runKubectlLogsToFile = func(ctx context.Context, cfg Config, podName, logFile string) error {
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		defer func() {
			if closeErr := f.Close(); closeErr != nil {
				fmt.Fprintf(os.Stderr, "[日志] 关闭日志文件失败: %v\n", closeErr)
			}
		}()

		cmd := exec.CommandContext(ctx, "kubectl", "logs", podName, "-n", cfg.Namespace, "--follow", "-c", "ascend")
		cmd.Env = commandEnv(cfg)
		cmd.Stdout = f
		cmd.Stderr = f
		return cmd.Run()
	}
	tailFile = func(ctx context.Context, logFile string) error {
		cmd := exec.CommandContext(ctx, "tail", "-F", logFile)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}
	checkJobMainPodRunning   = isJobMainPodRunningContext
	waitForLogReconnectDelay = func(ctx context.Context, delay time.Duration) error {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
			return nil
		}
	}
	tempLogFilePath = func(jobName string) string {
		return filepath.Join(os.TempDir(), fmt.Sprintf("%s.log", jobName))
	}
)

func gracefulStopTail(cancel context.CancelFunc, done chan struct{}) {
	time.Sleep(5 * time.Second)
	cancel()
	<-done
}

func followLogs(ctx context.Context, cfg Config, jobName string) {
	mainPod, err := waitForPodByLabelContextFunc(
		ctx,
		cfg,
		jobName,
		selectOldest,
		func(name string) bool {
			return strings.Contains(name, "main-script") || name == jobName
		},
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[日志] 查找主 Pod 失败: %v\n", err)
		return
	}

	logFile := tempLogFilePath(jobName)
	f, err := os.OpenFile(logFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[日志] 创建日志文件失败: %v\n", err)
	}
	f.Close()

	tailCtx, tailCancel := context.WithCancel(ctx)
	defer tailCancel()
	tailDone := make(chan struct{})
	go func() {
		defer close(tailDone)
		if err := tailFile(tailCtx, logFile); err != nil && ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "[日志] tail 错误: %v\n", err)
		}
	}()

	fmt.Fprintf(os.Stderr, "[日志] 等待 Pod %s 开始运行...\n", mainPod)
	for {
		select {
		case <-ctx.Done():
			gracefulStopTail(tailCancel, tailDone)
			return
		default:
		}

		phase, phaseErr := getPodPhaseFunc(cfg, mainPod)
		if phaseErr != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		if phase == "Running" || phase == "Succeeded" || phase == "Failed" {
			fmt.Fprintf(os.Stderr, "[日志] Pod 状态: %s，开始连接日志\n", phase)
			break
		}

		time.Sleep(2 * time.Second)
	}

	retryCount := 0
	for {
		select {
		case <-ctx.Done():
			gracefulStopTail(tailCancel, tailDone)
			return
		default:
		}

		fmt.Fprintf(os.Stderr, "[日志] 连接 kubectl logs %s ...\n", mainPod)
		err := runKubectlLogsToFile(ctx, cfg, mainPod, logFile)
		if err == nil || ctx.Err() != nil {
			gracefulStopTail(tailCancel, tailDone)
			return
		}

		phase, phaseErr := getPodPhaseFunc(cfg, mainPod)
		if phaseErr != nil {
			fmt.Fprintln(os.Stderr, "[日志] 获取 Pod 状态失败，继续重试...")
		} else if phase == "Pending" {
			fmt.Fprintf(os.Stderr, "[日志] Pod 还在启动 (%s)，等待 10s 后重试...\n", phase)
			if err := waitForLogReconnectDelay(ctx, 10*time.Second); err != nil {
				gracefulStopTail(tailCancel, tailDone)
				return
			}
			retryCount++
			continue
		} else if phase == "Succeeded" || phase == "Failed" {
			fmt.Fprintf(os.Stderr, "[日志] Pod 已结束 (%s)，停止日志跟随\n", phase)
			gracefulStopTail(tailCancel, tailDone)
			return
		}

		running, checkErr := checkJobMainPodRunning(ctx, cfg, jobName)
		if checkErr != nil {
			if errors.Is(checkErr, context.Canceled) {
				gracefulStopTail(tailCancel, tailDone)
				return
			}
			fmt.Fprintln(os.Stderr, "[日志] 获取 pod 状态失败，继续重试...")
		} else if !running {
			fmt.Fprintln(os.Stderr, "[日志] 主 Pod 已结束，停止日志跟随")
			gracefulStopTail(tailCancel, tailDone)
			return
		}

		retryCount++
		fmt.Fprintf(os.Stderr, "[日志] kubectl logs 断开，%s 后重试...\n", logReconnectDelay)

		if err := waitForLogReconnectDelay(ctx, retryDelayForAttempt(logReconnectDelay, retryCount)); err != nil {
			gracefulStopTail(tailCancel, tailDone)
			return
		}
	}
}
