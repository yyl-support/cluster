package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGetImagePullEventCount(t *testing.T) {
	originalExec := execKubectlWithContext
	defer func() { execKubectlWithContext = originalExec }()

	execKubectlWithContext = func(ctx context.Context, cfg Config, args ...string) ([]byte, error) {
		return []byte(`{"items":[{"reason":"Pulling","message":"Pulling image \"test-image:v1\"","count":3}]}`), nil
	}

	cfg := Config{Namespace: "argo"}
	info, err := getImagePullEventCount(context.Background(), cfg, "test-pod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Count != 3 {
		t.Fatalf("expected count 3, got %d", info.Count)
	}
	if info.ImageName != "test-image:v1" {
		t.Fatalf("expected image name 'test-image:v1', got %q", info.ImageName)
	}
}

func TestGetImagePullEventCountJqError(t *testing.T) {
	originalExec := execKubectlWithContext
	defer func() { execKubectlWithContext = originalExec }()

	execKubectlWithContext = func(ctx context.Context, cfg Config, args ...string) ([]byte, error) {
		return nil, errors.New("kubectl failed")
	}

	cfg := Config{Namespace: "argo"}
	_, err := getImagePullEventCount(context.Background(), cfg, "test-pod")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestGetImagePullEventCountNoPullingEvents(t *testing.T) {
	originalExec := execKubectlWithContext
	defer func() { execKubectlWithContext = originalExec }()

	execKubectlWithContext = func(ctx context.Context, cfg Config, args ...string) ([]byte, error) {
		return []byte(`{"items":[{"reason":"Scheduled","message":"pod scheduled","count":1}]}`), nil
	}

	cfg := Config{Namespace: "argo"}
	info, err := getImagePullEventCount(context.Background(), cfg, "test-pod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if info.Count != 0 {
		t.Fatalf("expected count 0, got %d", info.Count)
	}
	if info.ImageName != "" {
		t.Fatalf("expected empty image name, got %q", info.ImageName)
	}
}

func TestIsImagePullError(t *testing.T) {
	err := &imagePullError{ImageName: "test-image:v1", PullCount: 3}
	if !isImagePullError(err) {
		t.Fatal("expected isImagePullError to return true")
	}

	regularErr := errors.New("some error")
	if isImagePullError(regularErr) {
		t.Fatal("expected isImagePullError to return false for regular error")
	}
}

func TestWaitForPodCompletionImagePullFailure(t *testing.T) {
	origPhase := getPodPhaseFunc
	origPull := getImagePullEventCountFunc
	defer func() {
		getPodPhaseFunc = origPhase
		getImagePullEventCountFunc = origPull
	}()

	callCount := 0
	getPodPhaseFunc = func(cfg Config, podName string) (string, error) {
		callCount++
		return "Pending", nil
	}
	getImagePullEventCountFunc = func(ctx context.Context, cfg Config, podName string) (imagePullInfo, error) {
		return imagePullInfo{Count: 6, ImageName: "test-image:v1"}, nil
	}

	cfg := Config{Namespace: "argo"}
	_, err := waitForPodCompletion(cfg, "test-pod")
	if err == nil {
		t.Fatal("expected image pull error")
	}
	if !isImagePullError(err) {
		t.Fatalf("expected imagePullError, got %T", err)
	}
	pullErr := err.(*imagePullError)
	if pullErr.ImageName != "test-image:v1" {
		t.Fatalf("expected image name 'test-image:v1', got %q", pullErr.ImageName)
	}
}

func TestWaitForPodCompletionPendingButLowPullCount(t *testing.T) {
	origPhase := getPodPhaseFunc
	origPull := getImagePullEventCountFunc
	origEvents := getPodEventsFunc
	defer func() {
		getPodPhaseFunc = origPhase
		getImagePullEventCountFunc = origPull
		getPodEventsFunc = origEvents
	}()

	callCount := 0
	getPodPhaseFunc = func(cfg Config, podName string) (string, error) {
		callCount++
		if callCount <= 4 {
			return "Pending", nil
		}
		return "Succeeded", nil
	}
	getImagePullEventCountFunc = func(ctx context.Context, cfg Config, podName string) (imagePullInfo, error) {
		return imagePullInfo{Count: 1, ImageName: ""}, nil
	}
	getPodEventsFunc = func(ctx context.Context, cfg Config, podName string) ([]podEvent, error) {
		return []podEvent{{Reason: "Scheduled", Message: "pod scheduled", Count: 1}}, nil
	}

	cfg := Config{Namespace: "argo"}
	phase, err := waitForPodCompletion(cfg, "test-pod")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if phase != "Succeeded" {
		t.Fatalf("expected Succeeded, got %s", phase)
	}
}

func TestDeleteJob(t *testing.T) {
	origRun := runCommandOutputRetryFunc
	defer func() { runCommandOutputRetryFunc = origRun }()

	var calledName string
	var calledArgs []string
	runCommandOutputRetryFunc = func(cfg Config, name string, args []string, attempts int, delay time.Duration) ([]byte, error) {
		calledName = name
		calledArgs = args
		return []byte("job deleted"), nil
	}

	cfg := Config{Namespace: "test-ns"}
	err := deleteJob(cfg, "test-job")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if calledName != "kubectl" {
		t.Fatalf("expected 'kubectl', got %q", calledName)
	}
	expected := []string{"delete", "job.batch.volcano.sh", "test-job", "-n", "test-ns"}
	if len(calledArgs) != len(expected) {
		t.Fatalf("expected %d args, got %d", len(expected), len(calledArgs))
	}
	for i, arg := range expected {
		if calledArgs[i] != arg {
			t.Fatalf("arg[%d]: expected %q, got %q", i, arg, calledArgs[i])
		}
	}
}

func TestShortContainerID(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"containerd with prefix", "containerd://d0ff003f42e827e9982933a76c93849cd566f459caee60c38d8fd4396f6474fd", "d0ff003f42e8"},
		{"docker with prefix", "docker://cf19c51517708961579a3e7b3f193c1f351e8437c2f13b214288", "cf19c5151770"},
		{"no prefix", "d0ff003f42e827e9982933a76c93849cd566f45", "d0ff003f42e8"},
		{"short no prefix", "abc123", "abc123"},
		{"empty", "", ""},
		{"exactly 12 chars", "abcdefghijkl", "abcdefghijkl"},
		{"prefix only", "containerd://abc", "abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := shortContainerID(tt.input)
			if got != tt.expected {
				t.Fatalf("shortContainerID(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}
