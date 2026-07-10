package kubectl

import (
	"context"
	"strings"
	"time"
)

type RetryConfig struct {
	MaxRetries  int
	Backoff     []time.Duration
	IsRetryable func(error) bool
}

func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries: 10,
		Backoff:    []time.Duration{10 * time.Second, 20 * time.Second, 40 * time.Second},
		IsRetryable: func(err error) bool {
			if err == nil {
				return false
			}
			errStr := err.Error()
			return strings.Contains(errStr, "stream error") ||
				strings.Contains(errStr, "INTERNAL_ERROR") ||
				strings.Contains(errStr, "closed connection") ||
				strings.Contains(errStr, "EOF") ||
				strings.Contains(errStr, "connection reset") ||
				strings.Contains(errStr, "connection lost")
		},
	}
}

func ExecWithRetry(ctx context.Context, executor Executor, args []string, config RetryConfig) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt < config.MaxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		out, err := executor.Exec(ctx, args...)
		if err == nil {
			return out, nil
		}

		lastErr = err
		if !config.IsRetryable(err) {
			break
		}

		if attempt < config.MaxRetries-1 {
			idx := attempt
			if idx >= len(config.Backoff) {
				idx = len(config.Backoff) - 1
			}
			wait := config.Backoff[idx]
			timer := time.NewTimer(wait)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return nil, lastErr
}
