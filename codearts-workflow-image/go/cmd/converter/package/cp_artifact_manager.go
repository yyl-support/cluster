package converter

import "github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"

const (
	ArtifactVolumeName = "output"
)

func NeedsArtifactMultiTask(cpArtifacts, cpArtifactsTempFolder string) bool {
	return cpArtifactsTempFolder != ""
}

func GetArtifactMountPath(cpArtifactsTempFolder string) string {
	if cpArtifactsTempFolder == "" {
		return "/output"
	}
	return cpArtifactsTempFolder
}

func GetCopyArtifactTaskName() string {
	return "copy-artifact"
}

func addArtifactVolumeAndSidecar(task *volcano.TaskSpec, mountPath string, nodeSelector map[string]string, imagePullSecrets []volcano.LocalObjectReference, activeDeadlineSeconds int64) {
	if len(task.Template.Spec.Containers) == 0 {
		return
	}

	mainContainer := &task.Template.Spec.Containers[0]
	mainContainer.VolumeMounts = append(mainContainer.VolumeMounts, volcano.VolumeMount{
		Name:      ArtifactVolumeName,
		MountPath: mountPath,
	})

	if len(mainContainer.Args) > 0 {
		doneFile := mountPath + "/.ascend-done"
		mainContainer.Args[0] = "(" + mainContainer.Args[0] + "); _ec=$?; echo $_ec > " + doneFile + "; exit $_ec"
	}

	emptyDirVolume := volcano.Volume{
		Name:     ArtifactVolumeName,
		EmptyDir: &volcano.EmptyDir{SizeLimit: "10Gi"},
	}
	task.Template.Spec.Volumes = append(task.Template.Spec.Volumes, emptyDirVolume)

	sidecarContainer := volcano.Container{
		Name:    GetCopyArtifactTaskName(),
		Image:   "swr.cn-southwest-2.myhuaweicloud.com/modelfoundry/alpine:3.23.3",
		Command: []string{"sh"},
		Args: []string{"-c", `trap 'echo "Received TERM, exiting."; exit 0' TERM
TIMER_FILE="/tmp/reset_timer"
TIMEOUT_SEC=600
touch "$TIMER_FILE"
COUNT=0
while true; do
  if [ -f "$DONE_FILE" ]; then
    _ascend_exit=$(cat "$DONE_FILE" 2>/dev/null || echo "1")
    echo "ascend container finished (exit $_ascend_exit)."
    if [ "$_ascend_exit" != "0" ]; then
      echo "ascend failed, exiting sidecar."
      exit 0
    fi
  fi
  LAST=$(stat -c %Y "$TIMER_FILE" 2>/dev/null || stat -f %m "$TIMER_FILE")
  NOW=$(date +%s)
  ELAPSED=$((NOW - LAST))
  if [ "$ELAPSED" -ge "$TIMEOUT_SEC" ]; then
    echo "Timer expired. Exiting."
    exit 0
  fi
  COUNT=$((COUNT + 1))
  if [ $((COUNT % 5)) -eq 0 ]; then
    REMAINING=$((TIMEOUT_SEC - ELAPSED))
    echo "Timer remaining: ${REMAINING}s"
  fi
  sleep 1
done`},
		Resources: volcano.Resources{
			Limits:   volcano.ResourceList{"cpu": "2", "memory": "1Gi"},
			Requests: volcano.ResourceList{"cpu": "200m", "memory": "200Mi"},
		},
		VolumeMounts: []volcano.VolumeMount{
			{Name: ArtifactVolumeName, MountPath: mountPath},
		},
		Env: []volcano.EnvVar{
			{Name: "DONE_FILE", Value: mountPath + "/.ascend-done"},
			{Name: "WORKSPACE", Value: "/workspace"},
		},
	}
	task.Template.Spec.Containers = append(task.Template.Spec.Containers, sidecarContainer)
}
