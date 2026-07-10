package kubectl

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type mockExecutor struct {
	callCount int
	errors    []error
	output    []byte
}

func (m *mockExecutor) Exec(ctx context.Context, args ...string) ([]byte, error) {
	if m.callCount >= len(m.errors) {
		return m.output, nil
	}
	err := m.errors[m.callCount]
	m.callCount++
	if err != nil {
		return nil, err
	}
	return m.output, nil
}

func TestExecWithRetry_RetryableError(t *testing.T) {
	mock := &mockExecutor{
		errors: []error{
			errors.New("stream error: stream ID1; INTERNAL_ERROR; received from peer"),
			errors.New("stream error: stream ID1; INTERNAL_ERROR; received from peer"),
			nil,
		},
		output: []byte("success"),
	}
	config := RetryConfig{
		MaxRetries: 3,
		Backoff:    []time.Duration{1 * time.Millisecond, 1 * time.Millisecond},
		IsRetryable: func(err error) bool {
			return err != nil && strings.Contains(err.Error(), "stream error")
		},
	}

	out, err := ExecWithRetry(context.Background(), mock, []string{"get", "pods"}, config)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != "success" {
		t.Errorf("expected output 'success', got %q", out)
	}
	if mock.callCount != 3 {
		t.Errorf("expected 3 attempts, got %d", mock.callCount)
	}
}

func TestExecWithRetry_NonRetryableError(t *testing.T) {
	mock := &mockExecutor{
		errors: []error{
			errors.New("invalid argument: unknown command"),
		},
		output: nil,
	}
	config := RetryConfig{
		MaxRetries: 3,
		Backoff:    []time.Duration{1 * time.Millisecond},
		IsRetryable: func(err error) bool {
			return strings.Contains(err.Error(), "stream error")
		},
	}

	out, err := ExecWithRetry(context.Background(), mock, []string{"invalid"}, config)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if string(out) != "" {
		t.Errorf("expected empty output, got %q", out)
	}
	if mock.callCount != 1 {
		t.Errorf("expected 1 attempt, got %d", mock.callCount)
	}
}

func TestExecWithRetry_ContextCanceled(t *testing.T) {
	mock := &mockExecutor{
		errors: []error{
			errors.New("stream error"),
			errors.New("stream error"),
		},
		output: nil,
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	config := RetryConfig{
		MaxRetries:  3,
		Backoff:     []time.Duration{1 * time.Second},
		IsRetryable: func(err error) bool { return true },
	}

	_, err := ExecWithRetry(ctx, mock, []string{"get", "pods"}, config)
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
	if mock.callCount != 0 {
		t.Errorf("expected 0 attempts, got %d", mock.callCount)
	}
}

func TestExecWithRetry_MaxRetriesExceeded(t *testing.T) {
	mock := &mockExecutor{
		errors: []error{
			errors.New("stream error"),
			errors.New("stream error"),
			errors.New("stream error"),
		},
		output: nil,
	}
	config := RetryConfig{
		MaxRetries:  3,
		Backoff:     []time.Duration{1 * time.Millisecond},
		IsRetryable: func(err error) bool { return true },
	}

	_, err := ExecWithRetry(context.Background(), mock, []string{"get", "pods"}, config)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if mock.callCount != 3 {
		t.Errorf("expected 3 attempts, got %d", mock.callCount)
	}
}
