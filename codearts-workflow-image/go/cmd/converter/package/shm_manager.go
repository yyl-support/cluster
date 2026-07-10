package converter

import (
	"github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"
)

func AddShmVolume(container *volcano.Container, task *volcano.TaskSpec, shmSize string) {
	if shmSize == "" {
		return
	}

	normalizedSize := normalizeShmSize(shmSize)

	container.VolumeMounts = append(container.VolumeMounts, volcano.VolumeMount{
		Name:      "shm",
		MountPath: "/dev/shm",
	})

	task.Template.Spec.Volumes = append(task.Template.Spec.Volumes, volcano.Volume{
		Name: "shm",
		EmptyDir: &volcano.EmptyDir{
			Medium:    "Memory",
			SizeLimit: normalizedSize,
		},
	})
}