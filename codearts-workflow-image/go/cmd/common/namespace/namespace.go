package namespace

import (
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	DefaultNamespace       = "argo"
	RagsdkNamespace        = "ragsdk"
	OpPluginNamespace      = "op-plugin"
	RecsdkNamespace        = "recsdk"
	MultimodalsdkNamespace = "multimodalsdk"
	IndexsdkNamespace      = "indexsdk"
)

func GetNamespaceFromRepoName(repoName string) string {
	if repoName == "" {
		return DefaultNamespace
	}

	repoNameLower := strings.ToLower(repoName)

	if strings.Contains(repoNameLower, "ragsdk") ||
		// strings.Contains(repoNameLower, "ascendnpu-ir") ||
		strings.Contains(repoNameLower, "testorg-testrepo-test21") ||
		strings.Contains(repoName, "ascend-text-embeddings-inference") {

		return RagsdkNamespace
	} else if strings.Contains(repoNameLower, "ascend-op-plugin") ||

		strings.Contains(repoName, "ascend-pytorch") {

		return OpPluginNamespace
	} else if strings.Contains(repoNameLower, "ascend-recsdk") {

		return RecsdkNamespace
	} else if strings.Contains(repoNameLower, "ascend-multimodalsdk") {

		return MultimodalsdkNamespace
	} else if strings.Contains(repoNameLower, "ascend-indexsdk") {

		return IndexsdkNamespace
	}

	return DefaultNamespace
}

func GetRepoNameFromWorkflow(workflowPath string) (string, error) {
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		return "", fmt.Errorf("failed to read workflow file: %w", err)
	}

	var job struct {
		Metadata struct {
			Labels map[string]string `yaml:"labels"`
		} `yaml:"metadata"`
	}

	if err := yaml.Unmarshal(data, &job); err != nil {
		return "", fmt.Errorf("failed to parse workflow YAML: %w", err)
	}

	return job.Metadata.Labels["jobRepositoryName"], nil
}

func GetNamespaceFromWorkflow(workflowPath string) (string, error) {
	repoName, err := GetRepoNameFromWorkflow(workflowPath)
	if err != nil {
		return DefaultNamespace, err
	}

	return GetNamespaceFromRepoName(repoName), nil
}
