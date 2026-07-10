package converter

import (
	runonparser "github.com/opensourceways/codearts-workflow-image-go/cmd/common"
	"github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"
)

const (
	nodeChipNameLabel = "node.kubernetes.io/npu.chip.name"
	chipToExclude     = "310P3"
)

func AddNPUAffinity(podSpec *volcano.PodSpec, runsOn string) {
	if podSpec == nil {
		return
	}

	parsedSpec, err := runonparser.Parse(runsOn)
	if err != nil || parsedSpec == nil || parsedSpec.IsNPUEmpty() || !parsedSpec.IsGenericNPU() {
		return
	}

	if podSpec.Affinity == nil {
		podSpec.Affinity = &volcano.Affinity{}
	}

	if podSpec.Affinity.NodeAffinity == nil {
		podSpec.Affinity.NodeAffinity = &volcano.NodeAffinity{}
	}

	if podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &volcano.NodeSelector{
			NodeSelectorTerms: []volcano.NodeSelectorTerm{},
		}
	}

	terms := podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
	if len(terms) == 0 {
		terms = append(terms, volcano.NodeSelectorTerm{})
	}

	terms[0].MatchExpressions = append(terms[0].MatchExpressions, volcano.NodeSelectorRequirement{
		Key:      nodeChipNameLabel,
		Operator: "NotIn",
		Values:   []string{chipToExclude},
	})

	podSpec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = terms
}
