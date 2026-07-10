package converter

import (
	"testing"

	"github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"
)

func TestNeedsArtifactMultiTask(t *testing.T) {
	tests := []struct {
		name                  string
		cpArtifacts           string
		cpArtifactsTempFolder string
		want                  bool
	}{
		{"both empty", "", "", false},
		{"temp folder only", "", "/output/artifact", true},
		{"both specified", "*.txt;*.log", "/output/artifact", true},
		{"artifacts only", "*.txt", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NeedsArtifactMultiTask(tt.cpArtifacts, tt.cpArtifactsTempFolder); got != tt.want {
				t.Errorf("NeedsArtifactMultiTask() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetArtifactMountPath(t *testing.T) {
	tests := []struct {
		name                  string
		cpArtifactsTempFolder string
		want                  string
	}{
		{"empty defaults to /output", "", "/output"},
		{"specified path", "/output/artifact", "/output/artifact"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetArtifactMountPath(tt.cpArtifactsTempFolder); got != tt.want {
				t.Errorf("GetArtifactMountPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetCopyArtifactTaskName(t *testing.T) {
	if got := GetCopyArtifactTaskName(); got != "copy-artifact" {
		t.Errorf("GetCopyArtifactTaskName() = %v, want %v", got, "copy-artifact")
	}
}

func TestAddArtifactVolumeAndSidecar(t *testing.T) {
	mainContainer := volcano.Container{Name: "ascend"}
	task := &volcano.TaskSpec{
		Template: volcano.PodTemplateSpec{
			Spec: volcano.PodSpec{
				Containers: []volcano.Container{mainContainer},
			},
		},
	}
	mountPath := "/output/artifact"
	nodeSelector := map[string]string{"kubernetes.io/arch": "amd64"}
	imagePullSecrets := []volcano.LocalObjectReference{{Name: "test-secret"}}
	activeDeadlineSeconds := int64(14400)

	addArtifactVolumeAndSidecar(task, mountPath, nodeSelector, imagePullSecrets, activeDeadlineSeconds)

	if len(task.Template.Spec.Containers) != 2 {
		t.Fatalf("expected 2 containers (main + sidecar), got %d", len(task.Template.Spec.Containers))
	}

	main := task.Template.Spec.Containers[0]
	if len(main.VolumeMounts) != 1 {
		t.Fatalf("expected 1 volumeMount on main container, got %d", len(main.VolumeMounts))
	}
	if main.VolumeMounts[0].Name != ArtifactVolumeName {
		t.Errorf("expected main volumeMount name %s, got %s", ArtifactVolumeName, main.VolumeMounts[0].Name)
	}
	if main.VolumeMounts[0].MountPath != mountPath {
		t.Errorf("expected main mountPath %s, got %s", mountPath, main.VolumeMounts[0].MountPath)
	}

	sidecar := task.Template.Spec.Containers[1]
	if sidecar.Name != GetCopyArtifactTaskName() {
		t.Errorf("expected sidecar name %s, got %s", GetCopyArtifactTaskName(), sidecar.Name)
	}
	if len(sidecar.VolumeMounts) != 1 {
		t.Fatalf("expected 1 volumeMount on sidecar, got %d", len(sidecar.VolumeMounts))
	}
	if sidecar.VolumeMounts[0].Name != ArtifactVolumeName {
		t.Errorf("expected sidecar volumeMount name %s, got %s", ArtifactVolumeName, sidecar.VolumeMounts[0].Name)
	}
	if sidecar.VolumeMounts[0].MountPath != mountPath {
		t.Errorf("expected sidecar mountPath %s, got %s", mountPath, sidecar.VolumeMounts[0].MountPath)
	}

	if len(task.Template.Spec.Volumes) != 1 {
		t.Fatalf("expected 1 Pod volume, got %d", len(task.Template.Spec.Volumes))
	}
	if task.Template.Spec.Volumes[0].Name != ArtifactVolumeName {
		t.Errorf("expected volume name %s, got %s", ArtifactVolumeName, task.Template.Spec.Volumes[0].Name)
	}
	if task.Template.Spec.Volumes[0].EmptyDir == nil {
		t.Fatal("expected EmptyDir volume to be set")
	}
	if task.Template.Spec.Volumes[0].Ephemeral != nil {
		t.Error("expected Ephemeral volume to be nil (replaced by EmptyDir)")
	}
}
