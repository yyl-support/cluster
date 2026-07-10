package converter

import (
	"github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"
)

func convertEnv(env map[string]string) []volcano.EnvVar {
	vars := make([]volcano.EnvVar, 0, len(env))
	for k, v := range env {
		vars = append(vars, volcano.EnvVar{Name: k, Value: v})

	}
	return vars
}

func ConvertToSecretEnv(secretName string, env map[string]string) []volcano.EnvVar {
	vars := make([]volcano.EnvVar, 0, len(env))
	for k := range env {
		vars = append(vars, volcano.EnvVar{
			Name: k,
			ValueFrom: &volcano.EnvVarSource{
				SecretKeyRef: &volcano.SecretKeySelector{
					Name: secretName,
					Key:  k,
				},
			},
		})
	}
	return vars
}
