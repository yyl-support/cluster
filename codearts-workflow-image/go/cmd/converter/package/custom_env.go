package converter

import (
	"github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"
)

func AddCustomEnv(envVars map[string]string, resources volcano.Resources) map[string]string {
	if cpu, ok := resources.Requests["cpu"]; ok {
		if envVars == nil {
			envVars = make(map[string]string)
		}
		envVars["MAX_JOBS"] = cpu
		envVars["CMAKE_BUILD_PARALLEL_LEVEL"] = cpu
	}
	return envVars
}
