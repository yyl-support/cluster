package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestExtendCopyArtifactTimer(t *testing.T) {
	originalExec := execKubectlWithContext
	defer func() { execKubectlWithContext = originalExec }()

	var capturedArgs []string
	execKubectlWithContext = func(ctx context.Context, cfg Config, args ...string) ([]byte, error) {
		capturedArgs = args
		return nil, nil
	}

	cfg := Config{Namespace: "argo"}
	err := extendCopyArtifactTimer(cfg, "test-pod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the kubectl exec command was constructed correctly
	joined := strings.Join(capturedArgs, " ")
	if !strings.Contains(joined, "exec") {
		t.Errorf("expected 'exec' in args, got %v", capturedArgs)
	}
	if !strings.Contains(joined, "copy-artifact") {
		t.Errorf("expected 'copy-artifact' container in args, got %v", capturedArgs)
	}
	if !strings.Contains(joined, "touch") {
		t.Errorf("expected 'touch' in args, got %v", capturedArgs)
	}
	if !strings.Contains(joined, "/tmp/reset_timer") {
		t.Errorf("expected '/tmp/reset_timer' in args, got %v", capturedArgs)
	}
}

func TestExtendCopyArtifactTimerError(t *testing.T) {
	originalExec := execKubectlWithContext
	defer func() { execKubectlWithContext = originalExec }()

	execKubectlWithContext = func(ctx context.Context, cfg Config, args ...string) ([]byte, error) {
		return nil, errors.New("kubectl exec failed")
	}

	cfg := Config{Namespace: "argo"}
	err := extendCopyArtifactTimer(cfg, "test-pod")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "kubectl exec failed") {
		t.Errorf("expected error to contain 'kubectl exec failed', got %v", err)
	}
}
