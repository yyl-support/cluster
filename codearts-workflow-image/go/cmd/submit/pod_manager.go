package main

import (
	"fmt"
)

func getFirstContainerReason(cfg Config, podName string) (string, error) {
	args := []string{
		"get", "pod", podName, "-n", cfg.Namespace,
		"-o", "jsonpath={.status.containerStatuses[0].state.terminated.reason}",
	}
	output, err := execKubectl(cfg, args...)
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func getFirstContainerExitCode(cfg Config, podName string) (string, error) {
	args := []string{
		"get", "pod", podName, "-n", cfg.Namespace,
		"-o", "jsonpath={.status.containerStatuses[0].state.terminated.exitCode}",
	}
	output, err := execKubectl(cfg, args...)
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func getFirstContainerMessage(cfg Config, podName string) (string, error) {
	args := []string{
		"get", "pod", podName, "-n", cfg.Namespace,
		"-o", "jsonpath={.status.containerStatuses[0].state.terminated.message}",
	}
	output, err := execKubectl(cfg, args...)
	if err != nil {
		return "", err
	}
	return string(output), nil
}

func getPodFailureReason(cfg Config, podName string) string {
	reason, err := getFirstContainerReason(cfg, podName)
	if err != nil || reason == "" {
		return ""
	}

	exitCode, _ := getFirstContainerExitCode(cfg, podName)
	message, _ := getFirstContainerMessage(cfg, podName)

	result := reason
	if exitCode != "" {
		result += ", exitCode=" + exitCode
	}
	if message != "" {
		result += ", message=" + message
	}

	return result
}

func formatPodFailureError(cfg Config, podName, jobName, phase string, originalErr error) error {
	reason := getPodFailureReason(cfg, podName)

	errStr := ""
	if originalErr != nil {
		errStr = originalErr.Error()
	}

	if reason != "" {
		return fmt.Errorf("主 Pod %s 失败，状态: %s, 原因: %s, 错误: %s", jobName, phase, reason, errStr)
	}

	if errStr != "" {
		return fmt.Errorf("主 Pod %s 失败，状态: %s, 错误: %s", jobName, phase, errStr)
	}

	return fmt.Errorf("主 Pod %s 失败，状态: %s", jobName, phase)
}
