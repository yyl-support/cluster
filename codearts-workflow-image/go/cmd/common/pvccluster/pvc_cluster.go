package pvccluster

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"go.yaml.in/yaml/v3"
)

const (
	karmadaResourceBindingCRD = "resourcebindings.work.karmada.io"
	resourceBindingTimeout    = 2 * time.Minute
)

type KubectlExecutor interface {
	Exec(args ...string) ([]byte, error)
	ExecWithContext(ctx context.Context, args ...string) ([]byte, error)
}

type PVCClusterManager struct {
	executor     KubectlExecutor
	namespace    string
	kubeconfig   string
	isKarmada    bool
	karmadaCheck bool
}

func NewPVCClusterManager(executor KubectlExecutor, namespace, kubeconfig string) *PVCClusterManager {
	return &PVCClusterManager{
		executor:   executor,
		namespace:  namespace,
		kubeconfig: kubeconfig,
	}
}

func (m *PVCClusterManager) IsKarmadaCluster() bool {
	if m.karmadaCheck {
		return m.isKarmada
	}
	_, err := m.executor.Exec("get", "crd", karmadaResourceBindingCRD)
	m.isKarmada = err == nil
	m.karmadaCheck = true
	return m.isKarmada
}

type VolcanoJob struct {
	APIVersion string     `yaml:"apiVersion"`
	Kind       string     `yaml:"kind"`
	Metadata   JobMetadata `yaml:"metadata"`
	Spec       JobSpec    `yaml:"spec"`
}

type JobMetadata struct {
	Name         string            `yaml:"name,omitempty"`
	GenerateName string            `yaml:"generateName,omitempty"`
	Labels       map[string]string `yaml:"labels,omitempty"`
}

type JobSpec struct {
	Tasks []TaskSpec `yaml:"tasks"`
}

type TaskSpec struct {
	Name     string        `yaml:"name"`
	Template PodTemplateSpec `yaml:"template"`
}

type PodTemplateSpec struct {
	Spec PodSpec `yaml:"spec"`
}

type PodSpec struct {
	Volumes []Volume `yaml:"volumes,omitempty"`
}

type Volume struct {
	Name                  string                       `yaml:"name"`
	PersistentVolumeClaim *PersistentVolumeClaimVolume `yaml:"persistentVolumeClaim,omitempty"`
}

type PersistentVolumeClaimVolume struct {
	ClaimName string `yaml:"claimName"`
}

func ExtractPVCClaimNames(workflowYamlPath string) ([]string, error) {
	data, err := os.ReadFile(workflowYamlPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow yaml: %w", err)
	}

	var job VolcanoJob
	if err := yaml.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("failed to parse workflow yaml: %w", err)
	}

	if job.Kind != "Job" && job.Kind != "VolcanoJob" {
		return nil, fmt.Errorf("unsupported workflow kind: %s (expected Job/VolcanoJob)", job.Kind)
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

func (m *PVCClusterManager) GetPVCClusterMember(pvcName string) (string, error) {
	if !m.IsKarmadaCluster() {
		return "", nil
	}

	bindingName := pvcName
	ctx, cancel := context.WithTimeout(context.Background(), resourceBindingTimeout)
	defer cancel()

	for {
		select {
		case <-ctx.Done():
			return "", fmt.Errorf("timeout waiting for ResourceBinding %s", bindingName)
		default:
		}

		output, err := m.executor.ExecWithContext(ctx,
			"get", "resourcebinding", bindingName,
			"-n", m.namespace,
			"-o", "jsonpath={.spec.clusters[0].name}")
		if err == nil && len(output) > 0 {
			cluster := strings.TrimSpace(string(output))
			if cluster != "" {
				return cluster, nil
			}
		}

		time.Sleep(2 * time.Second)
	}
}

func (m *PVCClusterManager) GetPVCClusterMemberImmediate(pvcName string) (string, error) {
	if !m.IsKarmadaCluster() {
		return "", nil
	}

	bindingName := pvcName
	output, err := m.executor.Exec(
		"get", "resourcebinding", bindingName,
		"-n", m.namespace,
		"-o", "jsonpath={.spec.clusters[0].name}")
	if err != nil {
		return "", fmt.Errorf("failed to get ResourceBinding for PVC %s: %w", pvcName, err)
	}
	cluster := strings.TrimSpace(string(output))
	if cluster == "" {
		return "", fmt.Errorf("no cluster found in ResourceBinding for PVC %s", pvcName)
	}
	return cluster, nil
}

func (m *PVCClusterManager) GetMemberClusters() ([]string, error) {
	output, err := m.executor.Exec("get", "clusters", "-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return nil, fmt.Errorf("failed to get member clusters: %w", err)
	}

	clusterStr := strings.TrimSpace(string(output))
	if clusterStr == "" {
		return nil, nil
	}

	return strings.Split(clusterStr, " "), nil
}

func (m *PVCClusterManager) FindPVCInMemberClusters(pvcName string) (string, error) {
	if !m.IsKarmadaCluster() {
		return "", nil
	}

	clusters, err := m.GetMemberClusters()
	if err != nil {
		return "", fmt.Errorf("failed to get member clusters: %w", err)
	}

	for _, cluster := range clusters {
		rawPath := fmt.Sprintf("/apis/cluster.karmada.io/v1alpha1/clusters/%s/proxy/api/v1/namespaces/%s/persistentvolumeclaims/%s",
			cluster, m.namespace, pvcName)
		output, err := m.executor.Exec("get", "--raw", rawPath)
		if err == nil && len(output) > 0 {
			return cluster, nil
		}
	}

	return "", fmt.Errorf("PVC %s not found in any member cluster", pvcName)
}

func (m *PVCClusterManager) GetPVCClusterMembers(pvcNames []string) (map[string]string, error) {
	if !m.IsKarmadaCluster() {
		return nil, nil
	}

	result := make(map[string]string)
	for _, pvcName := range pvcNames {
		cluster, err := m.FindPVCInMemberClusters(pvcName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "WARN: %v\n", err)
			continue
		}
		if cluster != "" {
			result[pvcName] = cluster
		}
	}
	return result, nil
}

func BuildClusterLabelPatch(clusters map[string]string) string {
	labels := make(map[string]string)
	for _, cluster := range clusters {
		key := fmt.Sprintf("dispatch/%s", cluster)
		labels[key] = "true"
	}

	if len(labels) == 0 {
		return ""
	}

	labelPatch := `"metadata":{"labels":{`
	first := true
	for key, value := range labels {
		if !first {
			labelPatch += ","
		}
		labelPatch += fmt.Sprintf(`"%s":"%s"`, key, value)
		first = false
	}
	labelPatch += `}}`

	return fmt.Sprintf(`{%s}`, labelPatch)
}

func (m *PVCClusterManager) PatchWorkflowWithClusterLabels(workflowPath string) (map[string]string, error) {
	if !m.IsKarmadaCluster() {
		return nil, nil
	}

	pvcNames, err := ExtractPVCClaimNames(workflowPath)
	if err != nil {
		return nil, fmt.Errorf("failed to extract PVC claim names: %w", err)
	}

	if len(pvcNames) == 0 {
		return nil, nil
	}

	return m.PatchJobWithClusterLabels(workflowPath, pvcNames)
}

func (m *PVCClusterManager) PatchJobWithClusterLabels(workflowPath string, pvcNames []string) (map[string]string, error) {
	if !m.IsKarmadaCluster() {
		return nil, nil
	}

	if len(pvcNames) == 0 {
		return nil, nil
	}

	clusters, err := m.GetPVCClusterMembers(pvcNames)
	if err != nil {
		return nil, fmt.Errorf("failed to get PVC cluster members: %w", err)
	}

	if len(clusters) == 0 {
		return nil, nil
	}

	patch := BuildClusterLabelPatch(clusters)
	if patch == "" {
		return nil, nil
	}

	var job VolcanoJob
	data, err := os.ReadFile(workflowPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read workflow yaml: %w", err)
	}
	if err := yaml.Unmarshal(data, &job); err != nil {
		return nil, fmt.Errorf("failed to parse workflow yaml: %w", err)
	}

	jobName := job.Metadata.Name
	if jobName == "" && job.Metadata.GenerateName != "" {
		jobName = strings.TrimSuffix(job.Metadata.GenerateName, "-")
	}

	if jobName == "" {
		return nil, fmt.Errorf("cannot determine workflow name for patching")
	}

	_, err = m.executor.Exec(
		"patch", "job", jobName,
		"-n", m.namespace,
		"--type", "merge",
		"-p", patch)
	if err != nil {
		return nil, fmt.Errorf("failed to patch workflow with cluster labels: %w", err)
	}

	return clusters, nil
}

func (m *PVCClusterManager) PatchJobByNameWithClusterLabels(jobName string, pvcNames []string) (map[string]string, error) {
	if !m.IsKarmadaCluster() {
		return nil, nil
	}

	if len(pvcNames) == 0 {
		return nil, nil
	}

	clusters, err := m.GetPVCClusterMembers(pvcNames)
	if err != nil {
		return nil, fmt.Errorf("failed to get PVC cluster members: %w", err)
	}

	if len(clusters) == 0 {
		return nil, nil
	}

	patch := BuildClusterLabelPatch(clusters)
	if patch == "" {
		return nil, nil
	}

	_, err = m.executor.Exec(
		"patch", "jobs.batch.volcano.sh", jobName,
		"-n", m.namespace,
		"--type", "merge",
		"-p", patch)
	if err != nil {
		return nil, fmt.Errorf("failed to patch job %s with cluster labels: %w", jobName, err)
	}

	return clusters, nil
}