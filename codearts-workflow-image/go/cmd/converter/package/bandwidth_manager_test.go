package converter

import (
	"testing"

	"github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"
)

func TestAddBandwidthAnnotation(t *testing.T) {
	tests := []struct {
		name                string
		cpBandwidth         string
		existingAnnotations map[string]string
		wantAnnotations     map[string]string
	}{
		{
			name:            "empty bandwidth - no annotation added",
			cpBandwidth:     "",
			wantAnnotations: nil,
		},
		{
			name:        "150M bandwidth - annotation added",
			cpBandwidth: "150M",
			wantAnnotations: map[string]string{
				"kubernetes.io/ingress-bandwidth": "150M",
			},
		},
		{
			name:        "1G bandwidth - annotation added",
			cpBandwidth: "1G",
			wantAnnotations: map[string]string{
				"kubernetes.io/ingress-bandwidth": "1G",
			},
		},
		{
			name:        "500M bandwidth - annotation added",
			cpBandwidth: "500M",
			wantAnnotations: map[string]string{
				"kubernetes.io/ingress-bandwidth": "500M",
			},
		},
		{
			name:        "existing annotations preserved",
			cpBandwidth: "150M",
			existingAnnotations: map[string]string{
				"existing-key": "existing-value",
			},
			wantAnnotations: map[string]string{
				"existing-key":                    "existing-value",
				"kubernetes.io/ingress-bandwidth": "150M",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			task := volcano.TaskSpec{}
			if tt.existingAnnotations != nil {
				task.Template.Metadata.Annotations = tt.existingAnnotations
			}

			AddBandwidthAnnotation(&task, tt.cpBandwidth)

			if tt.wantAnnotations == nil {
				if task.Template.Metadata.Annotations != nil {
					t.Errorf("expected nil annotations, got %v", task.Template.Metadata.Annotations)
				}
				return
			}

			if task.Template.Metadata.Annotations == nil {
				t.Fatalf("expected annotations %v, got nil", tt.wantAnnotations)
			}

			for k, v := range tt.wantAnnotations {
				got, ok := task.Template.Metadata.Annotations[k]
				if !ok {
					t.Errorf("expected annotation key %q not found", k)
					continue
				}
				if got != v {
					t.Errorf("annotation %q = %q, want %q", k, got, v)
				}
			}

			if len(task.Template.Metadata.Annotations) != len(tt.wantAnnotations) {
				t.Errorf("annotation count = %d, want %d", len(task.Template.Metadata.Annotations), len(tt.wantAnnotations))
			}
		})
	}
}
