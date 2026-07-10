package main

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

type mockKubectl struct {
	callCount   int32
	responses   []mockResponse
	responseIdx int32
}

type mockResponse struct {
	output []byte
	err    error
}

func (m *mockKubectl) exec(ctx context.Context, cfg Config, args ...string) ([]byte, error) {
	atomic.AddInt32(&m.callCount, 1)
	idx := atomic.LoadInt32(&m.responseIdx)
	if int(idx) >= len(m.responses) {
		return nil, errors.New("no more mock responses")
	}
	resp := m.responses[idx]
	atomic.AddInt32(&m.responseIdx, 1)
	return resp.output, resp.err
}

func (m *mockKubectl) getCallCount() int {
	return int(atomic.LoadInt32(&m.callCount))
}

func setupMockKubectl(mock *mockKubectl) func() {
	original := execKubectlWithContext
	execKubectlWithContext = mock.exec
	return func() {
		execKubectlWithContext = original
	}
}

func TestWaitForResourceBinding(t *testing.T) {
	tests := []struct {
		name           string
		mockResponses  []mockResponse
		timeout        time.Duration
		wantErr        bool
		wantErrContain string
		wantCluster    string
		wantMinCalls   int
		wantMaxCalls   int
	}{
		{
			name: "timeout_on_not_found",
			mockResponses: []mockResponse{
				{err: errors.New("resourcebinding not found")},
				{err: errors.New("resourcebinding not found")},
				{err: errors.New("resourcebinding not found")},
			},
			timeout:        5 * time.Second,
			wantErr:        true,
			wantErrContain: "超时",
			wantMinCalls:   2,
			wantMaxCalls:   5,
		},
		{
			name: "timeout_on_empty_cluster",
			mockResponses: []mockResponse{
				{output: []byte("")},
				{output: []byte("")},
				{output: []byte("")},
			},
			timeout:        5 * time.Second,
			wantErr:        true,
			wantErrContain: "超时",
			wantMinCalls:   2,
			wantMaxCalls:   5,
		},
		{
			name: "success_on_first_call",
			mockResponses: []mockResponse{
				{output: []byte("gy-001")},
			},
			timeout:     10 * time.Second,
			wantErr:     false,
			wantCluster: "gy-001",
			wantMinCalls: 1,
			wantMaxCalls: 1,
		},
		{
			name: "success_after_retries",
			mockResponses: []mockResponse{
				{err: errors.New("resourcebinding not found")},
				{output: []byte("")},
				{output: []byte("wlcb-001")},
			},
			timeout:     10 * time.Second,
			wantErr:     false,
			wantCluster: "wlcb-001",
			wantMinCalls: 3,
			wantMaxCalls: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockKubectl{responses: tt.mockResponses}
			cleanup := setupMockKubectl(mock)
			defer cleanup()

			cfg := Config{Namespace: "argo"}
			start := time.Now()

			cluster, err := waitForResourceBinding(cfg, "test-job", tt.timeout)

			elapsed := time.Since(start)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErrContain) {
					t.Fatalf("expected error containing '%s', got: %v", tt.wantErrContain, err)
				}
				if elapsed < tt.timeout-1*time.Second {
					t.Fatalf("expected to wait at least %v, but only waited %v", tt.timeout-1*time.Second, elapsed)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				if cluster != tt.wantCluster {
					t.Fatalf("expected cluster '%s', got: %v", tt.wantCluster, cluster)
				}
			}

			calls := mock.getCallCount()
			if calls < tt.wantMinCalls {
				t.Fatalf("expected at least %d calls, got %d", tt.wantMinCalls, calls)
			}
			if calls > tt.wantMaxCalls {
				t.Fatalf("expected at most %d calls, got %d", tt.wantMaxCalls, calls)
			}

			t.Logf("test '%s': elapsed=%v, calls=%d, cluster=%s, err=%v", tt.name, elapsed, calls, cluster, err)
		})
	}
}

func TestWaitForResourceBindingContextCancellation(t *testing.T) {
	mock := &mockKubectl{
		responses: []mockResponse{
			{err: errors.New("resourcebinding not found")},
			{err: errors.New("resourcebinding not found")},
		},
	}
	cleanup := setupMockKubectl(mock)
	defer cleanup()

	cfg := Config{Namespace: "argo"}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	start := time.Now()

	done := make(chan struct{})
	var err error
	go func() {
		defer close(done)
		_, err = waitForResourceBinding(cfg, "test-job", 10*time.Second)
	}()

	select {
	case <-ctx.Done():
		t.Logf("context cancelled after %v", time.Since(start))
	case <-done:
		t.Logf("waitForResourceBinding completed after %v", time.Since(start))
	}

	if err != nil {
		if !strings.Contains(err.Error(), "超时") {
			t.Fatalf("expected timeout error, got: %v", err)
		}
	}

	calls := mock.getCallCount()
	t.Logf("calls=%d", calls)
}

func TestWaitForResourceBindingSchedulingFailure(t *testing.T) {
	mock := &mockKubectl{
		responses: []mockResponse{
			{output: []byte("")},
			{output: []byte("")},
			{output: []byte("")},
		},
	}
	cleanup := setupMockKubectl(mock)
	defer cleanup()

	cfg := Config{Namespace: "argo"}
	timeout := 5 * time.Second
	start := time.Now()

	_, err := waitForResourceBinding(cfg, "test-job", timeout)

	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error when scheduling fails (empty cluster)")
	}
	if !strings.Contains(err.Error(), "超时") {
		t.Fatalf("expected timeout error message, got: %v", err)
	}

	calls := mock.getCallCount()
	if calls < 2 {
		t.Fatalf("expected at least 2 calls for scheduling failure, got %d", calls)
	}

	t.Logf("scheduling failure detected: elapsed=%v, calls=%d, err=%v", elapsed, calls, err)
}