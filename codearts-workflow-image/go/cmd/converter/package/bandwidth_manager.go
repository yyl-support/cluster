package converter

import (
	"github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"
)

// AddBandwidthAnnotation sets kubernetes.io/ingress-bandwidth on the task's
// pod template metadata annotations when cpBandwidth is non-empty.
func AddBandwidthAnnotation(task *volcano.TaskSpec, cpBandwidth string) {
	if cpBandwidth == "" {
		return
	}

	if task.Template.Metadata.Annotations == nil {
		task.Template.Metadata.Annotations = make(map[string]string)
	}

	task.Template.Metadata.Annotations["kubernetes.io/ingress-bandwidth"] = cpBandwidth
}
