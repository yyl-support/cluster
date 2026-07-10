package main

import (
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

func addClusterDispatchLabels(cfg Config, workflowPath string) (string, error) {
	if !isKarmadaCluster(cfg) {
		return workflowPath, nil
	}

	pvcNames, err := extractPVCClaimNamesFromWorkflow(workflowPath)
	if err != nil {
		return workflowPath, fmt.Errorf("failed to extract PVC names: %w", err)
	}

	chipName, err := extractNodeSelectorChipNameFromWorkflow(workflowPath)
	if err != nil {
		return workflowPath, fmt.Errorf("failed to extract nodeSelector chip name: %w", err)
	}

	if len(pvcNames) == 0 && chipName == "" {
		return workflowPath, nil
	}

	var pvcCluster string
	var chipCluster string

	if len(pvcNames) > 0 {
		fmt.Printf("检测到 PVC mounts: %v\n", pvcNames)
		pvcClusters, err := getPVCClusterMembers(cfg, pvcNames)
		if err != nil {
			return workflowPath, err
		}
		fmt.Printf("PVC 所在集群: %v\n", pvcClusters)

		clusterSet := make(map[string]bool)
		for _, c := range pvcClusters {
			if c != "" {
				clusterSet[c] = true
			}
		}
		if len(clusterSet) == 1 {
			for c := range clusterSet {
				pvcCluster = c
			}
		}
	}

	if chipName != "" {
		fmt.Printf("检测到 nodeSelector chip name: %s\n", chipName)
		chipCluster, err = getNodeChipClusterMember(cfg, chipName)
		if err != nil {
			return workflowPath, err
		}
		if chipCluster != "" {
			fmt.Printf("节点标签所在集群: %s\n", chipCluster)
		}
	}

	if pvcCluster != "" && chipCluster != "" && pvcCluster != chipCluster {
		return workflowPath, fmt.Errorf("dispatch conflict: PVC in cluster %s, but nodeSelector chip %s in cluster %s", pvcCluster, chipName, chipCluster)
	}

	dispatchCluster := pvcCluster
	if dispatchCluster == "" {
		dispatchCluster = chipCluster
	}

	dispatchLabels := make(map[string]string)
	if len(pvcNames) > 0 {
		for _, pvcName := range pvcNames {
			if dispatchCluster != "" {
				dispatchLabels[pvcName] = dispatchCluster
			}
		}
	}
	if chipName != "" && dispatchCluster != "" {
		dispatchLabels[chipName] = dispatchCluster
	}

	if len(dispatchLabels) == 0 {
		return workflowPath, nil
	}

	return addDispatchLabelsToWorkflow(workflowPath, dispatchLabels, "dispatch")
}

func addDispatchLabelsToWorkflow(workflowPath string, dispatchLabels map[string]string, source string) (string, error) {
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		return workflowPath, fmt.Errorf("failed to read workflow: %w", err)
	}

	var job map[string]interface{}
	if err := yaml.Unmarshal(data, &job); err != nil {
		return workflowPath, fmt.Errorf("failed to parse workflow: %w", err)
	}

	metadata, ok := job["metadata"].(map[string]interface{})
	if !ok {
		metadata = make(map[string]interface{})
		job["metadata"] = metadata
	}

	labels, ok := metadata["labels"].(map[string]interface{})
	if !ok {
		labels = make(map[string]interface{})
		metadata["labels"] = labels
	}

	for key, cluster := range dispatchLabels {
		labelKey := fmt.Sprintf("dispatch/%s", cluster)
		labels[labelKey] = "true"
		fmt.Printf("Added label %s=true for %s %s\n", labelKey, source, key)
	}

	modifiedData, err := yaml.Marshal(job)
	if err != nil {
		return workflowPath, fmt.Errorf("failed to marshal workflow: %w", err)
	}

	labeledPath := workflowPath + ".labeled"
	if err := os.WriteFile(labeledPath, modifiedData, 0644); err != nil {
		return workflowPath, fmt.Errorf("failed to write labeled workflow: %w", err)
	}

	fmt.Printf("Created labeled workflow: %s\n", labeledPath)
	return labeledPath, nil
}

func extractPVCClaimNamesFromWorkflow(workflowPath string) ([]string, error) {
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow: %w", err)
	}

	var job struct {
		Kind string `yaml:"kind"`
		Spec struct {
			Tasks []struct {
				Template struct {
					Spec struct {
						Volumes []struct {
							PersistentVolumeClaim *struct {
								ClaimName string `yaml:"claimName"`
							} `yaml:"persistentVolumeClaim,omitempty"`
						} `yaml:"volumes,omitempty"`
					} `yaml:"spec"`
				} `yaml:"template"`
			} `yaml:"tasks"`
		} `yaml:"spec"`
	}

	if err := yaml.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("failed to parse workflow: %w", err)
	}

	if job.Kind != "Job" {
		return nil, fmt.Errorf("unsupported workflow kind: %s", job.Kind)
	}

	claimNames := make(map[string]bool)
	for _, task := range job.Spec.Tasks {
		for _, volume := range task.Template.Spec.Volumes {
			if volume.PersistentVolumeClaim != nil && volume.PersistentVolumeClaim.ClaimName != "" {
				claimNames[volume.PersistentVolumeClaim.ClaimName] = true
			}
		}
	}

	result := make([]string, 0, len(claimNames))
	for name := range claimNames {
		result = append(result, name)
	}
	return result, nil
}

func getPVCClusterMembers(cfg Config, pvcNames []string) (map[string]string, error) {
	clusters, err := getKarmadaMemberClusters(cfg)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, pvcName := range pvcNames {
		foundClusters := []string{}
		for _, cluster := range clusters {
			rawPath := fmt.Sprintf("/apis/cluster.karmada.io/v1alpha1/clusters/%s/proxy/api/v1/namespaces/%s/persistentvolumeclaims/%s",
				cluster, cfg.Namespace, pvcName)
			output, err := execKubectl(cfg, "get", "--raw", rawPath)
			if err == nil && len(output) > 0 {
				foundClusters = append(foundClusters, cluster)
			}
		}
		if len(foundClusters) == 1 {
			result[pvcName] = foundClusters[0]
		} else if len(foundClusters) > 1 {
			result[pvcName] = ""
		} else {
			return nil, fmt.Errorf("PVC %s not found in any cluster in namespace %s, please contact manager", pvcName, cfg.Namespace)
		}
	}

	clusterSet := make(map[string]bool)
	for _, cluster := range result {
		if cluster != "" {
			clusterSet[cluster] = true
		}
	}
	if len(clusterSet) > 1 {
		return nil, fmt.Errorf("PVCs are located in different clusters: %v", result)
	}

	return result, nil
}

func getKarmadaMemberClusters(cfg Config) ([]string, error) {
	output, err := execKubectl(cfg, "get", "clusters", "-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return nil, fmt.Errorf("failed to get member clusters: %w", err)
	}

	clusterStr := strings.TrimSpace(string(output))
	if clusterStr == "" {
		return nil, nil
	}

	return strings.Split(clusterStr, " "), nil
}

func extractNodeSelectorChipNameFromWorkflow(workflowPath string) (string, error) {
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		return "", fmt.Errorf("failed to read workflow: %w", err)
	}

	var job struct {
		Kind string `yaml:"kind"`
		Spec struct {
			Tasks []struct {
				Template struct {
					Spec struct {
						NodeSelector map[string]string `yaml:"nodeSelector,omitempty"`
					} `yaml:"spec"`
				} `yaml:"template"`
			} `yaml:"tasks"`
		} `yaml:"spec"`
	}

	if err := yaml.Unmarshal(data, &job); err != nil {
		return "", fmt.Errorf("failed to parse workflow: %w", err)
	}

	if job.Kind != "Job" {
		return "", fmt.Errorf("unsupported workflow kind: %s", job.Kind)
	}

	const chipNameLabel = "node.kubernetes.io/npu.chip.name"
	for _, task := range job.Spec.Tasks {
		if task.Template.Spec.NodeSelector != nil {
			if chipName, exists := task.Template.Spec.NodeSelector[chipNameLabel]; exists {
				return chipName, nil
			}
		}
	}

	return "", nil
}

func getNodeChipClusterMember(cfg Config, chipName string) (string, error) {
	clusters, err := getKarmadaMemberClusters(cfg)
	if err != nil {
		return "", err
	}

	foundClusters := []string{}
	for _, cluster := range clusters {
		rawPath := fmt.Sprintf("/apis/cluster.karmada.io/v1alpha1/clusters/%s/proxy/api/v1/nodes?labelSelector=node.kubernetes.io/npu.chip.name%%3D%s",
			cluster, chipName)
		output, err := execKubectl(cfg, "get", "--raw", rawPath)
		if err == nil && hasNonEmptyItems(output) {
			foundClusters = append(foundClusters, cluster)
		}
	}

	if len(foundClusters) == 1 {
		return foundClusters[0], nil
	} else if len(foundClusters) > 1 {
		return "", nil
	} else {
		return "", fmt.Errorf("nodes with chip name %s not found in any cluster, please contact manager", chipName)
	}
}

func hasNonEmptyItems(output []byte) bool {
	var nodeList struct {
		Items []interface{} `yaml:"items"`
	}
	if err := yaml.Unmarshal(output, &nodeList); err != nil {
		return false
	}
	return len(nodeList.Items) > 0
}
