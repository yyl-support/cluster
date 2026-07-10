package converter

import (
	"strings"

	volcano "github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"
)

const (
	archLabelKey          = "kubernetes.io/arch"
	ascend1980LabelPrefix = "huawei.com/ascend-1980"
	ascend310LabelPrefix  = "huawei.com/ascend-310"
	npuLabelKey           = "huawei.com/npu"
)

func isNPUResource(name string) bool {
	return strings.HasPrefix(name, ascend1980LabelPrefix) ||
		strings.HasPrefix(name, ascend310LabelPrefix)
}

func AddLabelsFromPodSpec(job *volcano.Job) {
	if job == nil || len(job.Spec.Tasks) == 0 {
		return
	}

	for i := range job.Spec.Tasks {
		task := &job.Spec.Tasks[i]
		podSpec := &task.Template.Spec

		labels := extractLabelsFromPodSpec(*podSpec)
		if len(labels) > 0 {
			if job.Metadata.Labels == nil {
				job.Metadata.Labels = make(map[string]string)
			}
			for k, v := range labels {
				job.Metadata.Labels[k] = v
			}
		}
	}
}

func extractLabelsFromPodSpec(podSpec volcano.PodSpec) map[string]string {
	labels := make(map[string]string)

	if arch, exists := podSpec.NodeSelector[archLabelKey]; exists {
		labels[archLabelKey] = arch
	}

	for _, container := range podSpec.Containers {
		npuLabels, hasNPU := extractNPULabelsFromResources(container.Resources)
		for k, v := range npuLabels {
			labels[k] = v
		}
		if hasNPU {
			labels[npuLabelKey] = "true"
		}
	}

	return labels
}

func extractNPULabelsFromResources(resources volcano.Resources) (map[string]string, bool) {
	labels := make(map[string]string)
	hasNPU := false

	for name, value := range resources.Limits {
		if isNPUResource(name) {
			labels[name] = value
			hasNPU = true
		}
	}

	for name, value := range resources.Requests {
		if isNPUResource(name) {
			labels[name] = value
			hasNPU = true
		}
	}

	return labels, hasNPU
}
