package converter

import (
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
)

const (
	DefaultNamespace = "argo"
)

func BuildSecretName(pipelineRunID, uniqueID string) string {
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(pipelineRunID+"-"+uniqueID)))
	shortHash := hash[:16]
	name := fmt.Sprintf("pipeline-secret-%s-%s", pipelineRunID, shortHash)
	if len(name) > 63 {
		name = name[:63]
	}
	return name
}

func BuildSecretManifest(secretName, namespace, pipelineRunID string, data map[string]string) string {
	var dataLines []string
	for k, v := range data {
		encodedValue := base64.StdEncoding.EncodeToString([]byte(v))
		dataLines = append(dataLines, fmt.Sprintf(`  %s: %s`, k, encodedValue))
	}

	var namespaceLine string
	if namespace != "" {
		namespaceLine = fmt.Sprintf("  namespace: %s\n", namespace)
	}

	return fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
%s  labels:
    pipeline/run-id: %s
type: Opaque
data:
%s
`, secretName, namespaceLine, pipelineRunID, strings.Join(dataLines, "\n"))
}

func BuildDeleteSecretManifest(secretName, namespace string) string {
	var namespaceLine string
	if namespace != "" {
		namespaceLine = fmt.Sprintf("  namespace: %s\n", namespace)
	}

	return fmt.Sprintf(`apiVersion: v1
kind: Secret
metadata:
  name: %s
%s`, secretName, namespaceLine)
}
