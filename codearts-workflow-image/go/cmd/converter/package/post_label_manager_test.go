package converter

import (
	"testing"

	volcano "github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"
)

func TestAddLabelsFromPodSpec(t *testing.T) {
	tests := []struct {
		name           string
		job            *volcano.Job
		expectedLabels map[string]string
	}{
		{
			name: "extract arm64 arch and NPU resource",
			job: &volcano.Job{
				Metadata: volcano.Metadata{
					Labels: map[string]string{"existing-label": "value"},
				},
				Spec: volcano.JobSpec{
					Tasks: []volcano.TaskSpec{
						{
							Template: volcano.PodTemplateSpec{
								Spec: volcano.PodSpec{
									NodeSelector: map[string]string{
										"kubernetes.io/arch": "arm64",
									},
									Containers: []volcano.Container{
										{
											Resources: volcano.Resources{
												Limits: volcano.ResourceList{
													"cpu":                    "32",
													"huawei.com/ascend-1980": "1",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedLabels: map[string]string{
				"existing-label":         "value",
				"kubernetes.io/arch":     "arm64",
				"huawei.com/ascend-1980": "1",
				"huawei.com/npu":         "true",
			},
		},
		{
			name: "extract arm64 arch and 310P3 NPU resource",
			job: &volcano.Job{
				Metadata: volcano.Metadata{
					Labels: map[string]string{"existing-label": "value"},
				},
				Spec: volcano.JobSpec{
					Tasks: []volcano.TaskSpec{
						{
							Template: volcano.PodTemplateSpec{
								Spec: volcano.PodSpec{
									NodeSelector: map[string]string{
										"kubernetes.io/arch": "arm64",
									},
									Containers: []volcano.Container{
										{
											Resources: volcano.Resources{
												Limits: volcano.ResourceList{
													"cpu":                   "32",
													"huawei.com/ascend-310": "1",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedLabels: map[string]string{
				"existing-label":        "value",
				"kubernetes.io/arch":    "arm64",
				"huawei.com/ascend-310": "1",
				"huawei.com/npu":        "true",
			},
		},
		{
			name: "no nodeSelector or NPU",
			job: &volcano.Job{
				Metadata: volcano.Metadata{},
				Spec: volcano.JobSpec{
					Tasks: []volcano.TaskSpec{
						{
							Template: volcano.PodTemplateSpec{
								Spec: volcano.PodSpec{
									Containers: []volcano.Container{
										{
											Resources: volcano.Resources{
												Limits: volcano.ResourceList{
													"cpu": "4",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedLabels: nil,
		},
		{
			name: "extract from requests",
			job: &volcano.Job{
				Metadata: volcano.Metadata{},
				Spec: volcano.JobSpec{
					Tasks: []volcano.TaskSpec{
						{
							Template: volcano.PodTemplateSpec{
								Spec: volcano.PodSpec{
									NodeSelector: map[string]string{
										"kubernetes.io/arch": "arm64",
									},
									Containers: []volcano.Container{
										{
											Resources: volcano.Resources{
												Requests: volcano.ResourceList{
													"huawei.com/ascend-1980": "2",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			expectedLabels: map[string]string{
				"kubernetes.io/arch":     "arm64",
				"huawei.com/ascend-1980": "2",
				"huawei.com/npu":         "true",
			},
		},
		{
			name:           "nil job",
			job:            nil,
			expectedLabels: nil,
		},
		{
			name: "empty tasks",
			job: &volcano.Job{
				Metadata: volcano.Metadata{},
				Spec: volcano.JobSpec{
					Tasks: []volcano.TaskSpec{},
				},
			},
			expectedLabels: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			AddLabelsFromPodSpec(tt.job)

			if tt.job == nil {
				return
			}

			if len(tt.expectedLabels) == 0 {
				if tt.job.Metadata.Labels != nil && len(tt.job.Metadata.Labels) > 0 {
					t.Errorf("expected no labels, got %v", tt.job.Metadata.Labels)
				}
				return
			}

			for k, v := range tt.expectedLabels {
				if tt.job.Metadata.Labels == nil {
					t.Errorf("expected label %s=%s, got nil labels", k, v)
					continue
				}
				if got, exists := tt.job.Metadata.Labels[k]; !exists || got != v {
					t.Errorf("expected label %s=%s, got %s=%s", k, v, k, got)
				}
			}
		})
	}
}

func TestExtractLabelsFromPodSpec(t *testing.T) {
	tests := []struct {
		name           string
		podSpec        volcano.PodSpec
		expectedLabels map[string]string
	}{
		{
			name: "extract all labels",
			podSpec: volcano.PodSpec{
				NodeSelector: map[string]string{
					"kubernetes.io/arch": "arm64",
				},
				Containers: []volcano.Container{
					{
						Resources: volcano.Resources{
							Limits: volcano.ResourceList{
								"huawei.com/ascend-1980": "1",
							},
						},
					},
				},
			},
			expectedLabels: map[string]string{
				"kubernetes.io/arch":     "arm64",
				"huawei.com/ascend-1980": "1",
				"huawei.com/npu":         "true",
			},
		},
		{
			name: "amd64 arch should not be extracted",
			podSpec: volcano.PodSpec{
				NodeSelector: map[string]string{
					"kubernetes.io/arch": "amd64",
				},
				Containers: []volcano.Container{},
			},
			expectedLabels: map[string]string{
				"kubernetes.io/arch": "amd64",
			},
		},
		{
			name: "nil NodeSelector and nil Resources",
			podSpec: volcano.PodSpec{
				NodeSelector: nil,
				Containers: []volcano.Container{
					{
						Resources: volcano.Resources{
							Limits:   nil,
							Requests: nil,
						},
					},
				},
			},
			expectedLabels: map[string]string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			labels := extractLabelsFromPodSpec(tt.podSpec)

			for k, v := range tt.expectedLabels {
				if got, exists := labels[k]; !exists || got != v {
					t.Errorf("expected label %s=%s, got %s=%s", k, v, k, got)
				}
			}
		})
	}
}

func TestIsNPUResource(t *testing.T) {
	tests := []struct {
		name         string
		resourceName string
		want         bool
	}{
		{
			name:         "ascend-1980 is NPU resource",
			resourceName: "huawei.com/ascend-1980",
			want:         true,
		},
		{
			name:         "ascend-310 is NPU resource",
			resourceName: "huawei.com/ascend-310",
			want:         true,
		},
		{
			name:         "cpu is not NPU resource",
			resourceName: "cpu",
			want:         false,
		},
		{
			name:         "memory is not NPU resource",
			resourceName: "memory",
			want:         false,
		},
		{
			name:         "empty string is not NPU resource",
			resourceName: "",
			want:         false,
		},
		{
			name:         "other ascend resource is not NPU",
			resourceName: "huawei.com/ascend-910",
			want:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNPUResource(tt.resourceName)
			if got != tt.want {
				t.Errorf("isNPUResource() = %v, want %v", got, tt.want)
			}
		})
	}
}
