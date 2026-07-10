package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestFollowLogsRetriesWhileMainPodRunning(t *testing.T) {
	originalRun := runKubectlLogsToFile
	originalCheck := checkJobMainPodRunning
	originalWait := waitForLogReconnectDelay
	originalWaitPod := waitForPodByLabelContextFunc
	originalPhase := getPodPhaseFunc
	originalTail := tailFile
	originalTempPath := tempLogFilePath
	defer func() {
		runKubectlLogsToFile = originalRun
		checkJobMainPodRunning = originalCheck
		waitForLogReconnectDelay = originalWait
		waitForPodByLabelContextFunc = originalWaitPod
		getPodPhaseFunc = originalPhase
		tailFile = originalTail
		tempLogFilePath = originalTempPath
	}()

	var runCalls int32
	var waits []time.Duration
	var tailCalls int32
	waitForPodByLabelContextFunc = func(ctx context.Context, cfg Config, jobName string, mode podSelectionMode, filter func(string) bool) (string, error) {
		return "main-script-pod", nil
	}
	getPodPhaseFunc = func(cfg Config, podName string) (string, error) {
		return "Running", nil
	}
	runKubectlLogsToFile = func(ctx context.Context, cfg Config, podName, logFile string) error {
		call := atomic.AddInt32(&runCalls, 1)
		if call == 1 {
			return errors.New("log stream dropped")
		}
		return nil
	}
	checkJobMainPodRunning = func(ctx context.Context, cfg Config, jobName string) (bool, error) {
		return true, nil
	}
	waitForLogReconnectDelay = func(ctx context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		return nil
	}
	tailFile = func(ctx context.Context, logFile string) error {
		atomic.AddInt32(&tailCalls, 1)
		<-ctx.Done()
		return nil
	}
	tempLogFilePath = func(jobName string) string {
		return "/tmp/test-wf-1.log"
	}

	followLogs(context.Background(), Config{Namespace: "argo"}, "wf-1")

	if got := atomic.LoadInt32(&runCalls); got != 2 {
		t.Fatalf("expected 2 log attempts, got %d", got)
	}
	if len(waits) != 1 || waits[0] != 10*time.Second {
		t.Fatalf("expected first wait to be 10s, got %#v", waits)
	}
	if got := atomic.LoadInt32(&tailCalls); got != 1 {
		t.Fatalf("expected 1 tail call, got %d", got)
	}
}

func TestFollowLogsExitsOnCancelDuringRetryCheck(t *testing.T) {
	originalRun := runKubectlLogsToFile
	originalCheck := checkJobMainPodRunning
	originalWait := waitForLogReconnectDelay
	originalWaitPod := waitForPodByLabelContextFunc
	originalPhase := getPodPhaseFunc
	originalTail := tailFile
	originalTempPath := tempLogFilePath
	defer func() {
		runKubectlLogsToFile = originalRun
		checkJobMainPodRunning = originalCheck
		waitForLogReconnectDelay = originalWait
		waitForPodByLabelContextFunc = originalWaitPod
		getPodPhaseFunc = originalPhase
		tailFile = originalTail
		tempLogFilePath = originalTempPath
	}()

	waitForPodByLabelContextFunc = func(ctx context.Context, cfg Config, jobName string, mode podSelectionMode, filter func(string) bool) (string, error) {
		return "main-script-pod", nil
	}
	getPodPhaseFunc = func(cfg Config, podName string) (string, error) {
		return "Running", nil
	}
	runKubectlLogsToFile = func(ctx context.Context, cfg Config, podName, logFile string) error {
		return errors.New("log stream dropped")
	}

	enteredCheck := make(chan struct{})
	checkJobMainPodRunning = func(ctx context.Context, cfg Config, jobName string) (bool, error) {
		close(enteredCheck)
		<-ctx.Done()
		return false, ctx.Err()
	}
	waitForLogReconnectDelay = func(ctx context.Context, delay time.Duration) error {
		return nil
	}
	tailFile = func(ctx context.Context, logFile string) error {
		<-ctx.Done()
		return nil
	}
	tempLogFilePath = func(jobName string) string {
		return "/tmp/test-wf-1.log"
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		followLogs(ctx, Config{Namespace: "argo"}, "wf-1")
	}()

	select {
	case <-enteredCheck:
	case <-time.After(2 * time.Second):
		t.Fatal("followLogs did not reach retry check")
	}

	cancel()

	select {
	case <-done:
	case <-time.After(7 * time.Second):
		t.Fatal("followLogs did not exit after cancel")
	}
}

func TestFollowLogsSnippetDoesNotStallAfterCancel(t *testing.T) {
	originalRun := runKubectlLogsToFile
	originalCheck := checkJobMainPodRunning
	originalWait := waitForLogReconnectDelay
	originalWaitPod := waitForPodByLabelContextFunc
	originalPhase := getPodPhaseFunc
	originalTail := tailFile
	originalTempPath := tempLogFilePath
	defer func() {
		runKubectlLogsToFile = originalRun
		checkJobMainPodRunning = originalCheck
		waitForLogReconnectDelay = originalWait
		waitForPodByLabelContextFunc = originalWaitPod
		getPodPhaseFunc = originalPhase
		tailFile = originalTail
		tempLogFilePath = originalTempPath
	}()

	waitForPodByLabelContextFunc = func(ctx context.Context, cfg Config, jobName string, mode podSelectionMode, filter func(string) bool) (string, error) {
		return "main-script-pod", nil
	}
	getPodPhaseFunc = func(cfg Config, podName string) (string, error) {
		return "Running", nil
	}
	runKubectlLogsToFile = func(ctx context.Context, cfg Config, podName, logFile string) error {
		return errors.New("log stream dropped")
	}
	checkJobMainPodRunning = func(ctx context.Context, cfg Config, jobName string) (bool, error) {
		<-ctx.Done()
		return false, ctx.Err()
	}
	waitForLogReconnectDelay = func(ctx context.Context, delay time.Duration) error {
		return nil
	}
	tailFile = func(ctx context.Context, logFile string) error {
		<-ctx.Done()
		return nil
	}
	tempLogFilePath = func(jobName string) string {
		return "/tmp/test-wf-1.log"
	}

	logCtx, cancelLogs := context.WithCancel(context.Background())
	logDone := make(chan struct{})
	go func() {
		defer close(logDone)
		followLogs(logCtx, Config{Namespace: "argo"}, "wf-1")
	}()

	phase := "Succeeded"
	var err error
	if phase != "Succeeded" {
		t.Fatal("setup failed")
	}

	cancelLogs()

	select {
	case <-logDone:
	case <-time.After(7 * time.Second):
		t.Fatal("<-logDone would stall after cancelLogs")
	}

	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
}

func TestFollowLogsStopsRetryWhenMainPodFinished(t *testing.T) {
	originalRun := runKubectlLogsToFile
	originalCheck := checkJobMainPodRunning
	originalWait := waitForLogReconnectDelay
	originalWaitPod := waitForPodByLabelContextFunc
	originalPhase := getPodPhaseFunc
	originalTail := tailFile
	originalTempPath := tempLogFilePath
	defer func() {
		runKubectlLogsToFile = originalRun
		checkJobMainPodRunning = originalCheck
		waitForLogReconnectDelay = originalWait
		waitForPodByLabelContextFunc = originalWaitPod
		getPodPhaseFunc = originalPhase
		tailFile = originalTail
		tempLogFilePath = originalTempPath
	}()

	var runCalls int32
	var checkCalls int32
	var waitCalls int32

	waitForPodByLabelContextFunc = func(ctx context.Context, cfg Config, jobName string, mode podSelectionMode, filter func(string) bool) (string, error) {
		return "main-script-pod", nil
	}
	getPodPhaseFunc = func(cfg Config, podName string) (string, error) {
		return "Running", nil
	}
	runKubectlLogsToFile = func(ctx context.Context, cfg Config, podName, logFile string) error {
		atomic.AddInt32(&runCalls, 1)
		return errors.New("log stream dropped")
	}
	checkJobMainPodRunning = func(ctx context.Context, cfg Config, jobName string) (bool, error) {
		atomic.AddInt32(&checkCalls, 1)
		return false, nil
	}
	waitForLogReconnectDelay = func(ctx context.Context, delay time.Duration) error {
		atomic.AddInt32(&waitCalls, 1)
		return nil
	}
	tailFile = func(ctx context.Context, logFile string) error {
		<-ctx.Done()
		return nil
	}
	tempLogFilePath = func(jobName string) string {
		return "/tmp/test-wf-1.log"
	}

	followLogs(context.Background(), Config{Namespace: "argo"}, "wf-1")

	if got := atomic.LoadInt32(&runCalls); got != 1 {
		t.Fatalf("expected 1 log attempt, got %d", got)
	}
	if got := atomic.LoadInt32(&checkCalls); got != 1 {
		t.Fatalf("expected 1 main pod check, got %d", got)
	}
	if got := atomic.LoadInt32(&waitCalls); got != 0 {
		t.Fatalf("expected no reconnect wait, got %d", got)
	}
}

func TestFollowLogsIncreasesWaitEachRetry(t *testing.T) {
	originalRun := runKubectlLogsToFile
	originalCheck := checkJobMainPodRunning
	originalWait := waitForLogReconnectDelay
	originalWaitPod := waitForPodByLabelContextFunc
	originalPhase := getPodPhaseFunc
	originalTail := tailFile
	originalTempPath := tempLogFilePath
	defer func() {
		runKubectlLogsToFile = originalRun
		checkJobMainPodRunning = originalCheck
		waitForLogReconnectDelay = originalWait
		waitForPodByLabelContextFunc = originalWaitPod
		getPodPhaseFunc = originalPhase
		tailFile = originalTail
		tempLogFilePath = originalTempPath
	}()

	var runCalls int32
	var waits []time.Duration
	waitForPodByLabelContextFunc = func(ctx context.Context, cfg Config, jobName string, mode podSelectionMode, filter func(string) bool) (string, error) {
		return "main-script-pod", nil
	}
	getPodPhaseFunc = func(cfg Config, podName string) (string, error) {
		return "Running", nil
	}
	runKubectlLogsToFile = func(ctx context.Context, cfg Config, podName, logFile string) error {
		call := atomic.AddInt32(&runCalls, 1)
		if call <= 2 {
			return errors.New("log stream dropped")
		}
		return nil
	}
	checkJobMainPodRunning = func(ctx context.Context, cfg Config, jobName string) (bool, error) {
		return true, nil
	}
	waitForLogReconnectDelay = func(ctx context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		return nil
	}
	tailFile = func(ctx context.Context, logFile string) error {
		<-ctx.Done()
		return nil
	}
	tempLogFilePath = func(jobName string) string {
		return "/tmp/test-wf-1.log"
	}

	followLogs(context.Background(), Config{Namespace: "argo"}, "wf-1")

	if got := atomic.LoadInt32(&runCalls); got != 3 {
		t.Fatalf("expected 3 log attempts, got %d", got)
	}
	if len(waits) != 2 {
		t.Fatalf("expected 2 waits, got %d", len(waits))
	}
	if waits[0] != 10*time.Second || waits[1] != 20*time.Second {
		t.Fatalf("expected waits [10s 20s], got %#v", waits)
	}
}

func TestNeedsLogFallbackEmptyFile(t *testing.T) {
	tmpFile := filepath.Join(os.TempDir(), "test-empty.log")
	os.WriteFile(tmpFile, []byte(""), 0644)
	defer os.Remove(tmpFile)

	if !needsLogFallback(tmpFile) {
		t.Fatal("expected fallback for empty file")
	}
}

func TestNeedsLogFallbackSmallFile(t *testing.T) {
	tmpFile := filepath.Join(os.TempDir(), "test-small.log")
	os.WriteFile(tmpFile, []byte("short"), 0644)
	defer os.Remove(tmpFile)

	if !needsLogFallback(tmpFile) {
		t.Fatal("expected fallback for small file (<100 bytes)")
	}
}

func TestNeedsLogFallbackOnlyArgoInternal(t *testing.T) {
	tmpFile := filepath.Join(os.TempDir(), "test-argo-internal.log")
	content := `time="2026-05-14T13:21:34.796Z" level=info msg="Re-establishing pod watch" namespace=argo workflow=test
time="2026-05-14T13:21:34.832Z" level=warning msg="watch object was not a pod" error="too old resource version" namespace=argo workflow=test
`
	os.WriteFile(tmpFile, []byte(content), 0644)
	defer os.Remove(tmpFile)

	if !needsLogFallback(tmpFile) {
		t.Fatal("expected fallback for file with only argo internal messages")
	}
}

func TestNeedsLogFallbackWithValidOutput(t *testing.T) {
	tmpFile := filepath.Join(os.TempDir(), "test-valid.log")
	content := `time="2026-05-14T13:21:34.796Z" level=info msg="Re-establishing pod watch" namespace=argo workflow=test
test-pod: Hello from workflow
test-pod: Build completed
`
	os.WriteFile(tmpFile, []byte(content), 0644)
	defer os.Remove(tmpFile)

	if needsLogFallback(tmpFile) {
		t.Fatal("expected NO fallback for file with valid workflow output")
	}
}

func TestNeedsLogFallbackMissingFile(t *testing.T) {
	if !needsLogFallback("/nonexistent/file.log") {
		t.Fatal("expected fallback for missing file")
	}
}
