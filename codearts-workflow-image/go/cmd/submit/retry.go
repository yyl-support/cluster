package main

import "time"

func retryDelayForAttempt(base time.Duration, attempt int) time.Duration {
	if attempt < 1 {
		return base
	}
	return base * time.Duration(attempt)
}
