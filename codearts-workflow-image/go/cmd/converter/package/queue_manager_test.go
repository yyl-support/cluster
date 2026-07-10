package converter

import "testing"

func TestDetermineQueue(t *testing.T) {
	tests := []struct {
		name     string
		runsOn   string
		expected string
	}{
		{
			name:     "cpu_less_than_64_returns_flexible_queue",
			runsOn:   "arm64-cpu-32-mem-64G",
			expected: QueueSharedFlexible,
		},
		{
			name:     "cpu_equal_64_returns_large_queue",
			runsOn:   "arm64-cpu-64-mem-128G",
			expected: QueueLargeTaskShared,
		},
		{
			name:     "cpu_greater_than_64_returns_large_queue",
			runsOn:   "arm64-cpu-128-mem-256G",
			expected: QueueLargeTaskShared,
		},
		{
			name:     "arm64_without_npu_returns_flexible_queue",
			runsOn:   "arm64-cpu-32-mem-64G",
			expected: QueueSharedFlexible,
		},
		{
			name:     "arm64_npu_8_returns_large_queue_64_cpu",
			runsOn:   "arm64-910b1-8-mem-512G",
			expected: QueueLargeTaskShared,
		},
		{
			name:     "arm64_npu_4_returns_flexible_queue_36_cpu",
			runsOn:   "arm64-910b2-4-mem-144G",
			expected: QueueSharedFlexible,
		},
		{
			name:     "arm64_npu_1_returns_flexible_queue_12_cpu",
			runsOn:   "arm64-910a-1-mem-48G",
			expected: QueueSharedFlexible,
		},
		{
			name:     "invalid_runs_on_returns_flexible_queue",
			runsOn:   "invalid-spec",
			expected: QueueSharedFlexible,
		},
		{
			name:     "empty_runs_on_returns_flexible_queue",
			runsOn:   "",
			expected: QueueSharedFlexible,
		},
		{
			name:     "cpu_only_less_than_64",
			runsOn:   "arm64-cpu-16",
			expected: QueueSharedFlexible,
		},
		{
			name:     "cpu_only_equal_64",
			runsOn:   "arm64-cpu-64",
			expected: QueueLargeTaskShared,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := DetermineQueue(tt.runsOn)
			if result != tt.expected {
				t.Errorf("DetermineQueue(%q) = %q, want %q", tt.runsOn, result, tt.expected)
			}
		})
	}
}
