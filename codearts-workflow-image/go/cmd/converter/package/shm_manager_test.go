package converter

import (
	"testing"

	"github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"
)

func TestAddShmVolume(t *testing.T) {
	tests := []struct {
		name              string
		shmSize           string
		expectVolume      bool
		expectMount       bool
		expectedSizeLimit string
	}{
		{
			name:              "empty_shm_size",
			shmSize:           "",
			expectVolume:      false,
			expectMount:       false,
			expectedSizeLimit: "",
		},
		{
			name:              "shm_size_8G",
			shmSize:           "8G",
			expectVolume:      true,
			expectMount:       true,
			expectedSizeLimit: "8Gi",
		},
		{
			name:              "shm_size_512Mi",
			shmSize:           "512Mi",
			expectVolume:      true,
			expectMount:       true,
			expectedSizeLimit: "512Mi",
		},
		{
			name:              "shm_size_1Gi",
			shmSize:           "1Gi",
			expectVolume:      true,
			expectMount:       true,
			expectedSizeLimit: "1Gi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := &volcano.Container{}
			task := &volcano.TaskSpec{
				Template: volcano.PodTemplateSpec{
					Spec: volcano.PodSpec{},
				},
			}

			AddShmVolume(container, task, tt.shmSize)

			if tt.expectVolume {
				if len(task.Template.Spec.Volumes) != 1 {
					t.Errorf("expected 1 volume, got %d", len(task.Template.Spec.Volumes))
				} else {
					vol := task.Template.Spec.Volumes[0]
					if vol.Name != "shm" {
						t.Errorf("expected volume name shm, got %s", vol.Name)
					}
					if vol.EmptyDir == nil {
						t.Error("expected EmptyDir to be set")
					} else {
						if vol.EmptyDir.Medium != "Memory" {
							t.Errorf("expected Medium Memory, got %s", vol.EmptyDir.Medium)
						}
						if vol.EmptyDir.SizeLimit != tt.expectedSizeLimit {
							t.Errorf("expected SizeLimit %s, got %s", tt.expectedSizeLimit, vol.EmptyDir.SizeLimit)
						}
					}
				}
			} else {
				if len(task.Template.Spec.Volumes) != 0 {
					t.Errorf("expected 0 volumes, got %d", len(task.Template.Spec.Volumes))
				}
			}

			if tt.expectMount {
				if len(container.VolumeMounts) != 1 {
					t.Errorf("expected 1 volume mount, got %d", len(container.VolumeMounts))
				} else {
					mount := container.VolumeMounts[0]
					if mount.Name != "shm" {
						t.Errorf("expected volume mount name shm, got %s", mount.Name)
					}
					if mount.MountPath != "/dev/shm" {
						t.Errorf("expected mount path /dev/shm, got %s", mount.MountPath)
					}
				}
			} else {
				if len(container.VolumeMounts) != 0 {
					t.Errorf("expected 0 volume mounts, got %d", len(container.VolumeMounts))
				}
			}
		})
	}
}

func TestAddShmVolume_AppendsToExisting(t *testing.T) {
	container := &volcano.Container{
		VolumeMounts: []volcano.VolumeMount{
			{Name: "existing-mount", MountPath: "/existing"},
		},
	}
	task := &volcano.TaskSpec{
		Template: volcano.PodTemplateSpec{
			Spec: volcano.PodSpec{
				Volumes: []volcano.Volume{
					{Name: "existing-volume"},
				},
			},
		},
	}

	AddShmVolume(container, task, "8G")

	if len(task.Template.Spec.Volumes) != 2 {
		t.Errorf("expected 2 volumes, got %d", len(task.Template.Spec.Volumes))
	}
	if len(container.VolumeMounts) != 2 {
		t.Errorf("expected 2 volume mounts, got %d", len(container.VolumeMounts))
	}

	shmVolume := task.Template.Spec.Volumes[1]
	if shmVolume.Name != "shm" {
		t.Errorf("expected second volume name shm, got %s", shmVolume.Name)
	}

	shmMount := container.VolumeMounts[1]
	if shmMount.Name != "shm" {
		t.Errorf("expected second mount name shm, got %s", shmMount.Name)
	}
}