package main

import (
	"fmt"
	"os"
	"strings"

	"go.yaml.in/yaml/v3"
)

const (
	harborNamespace   = "harbor"
	gitCacheNamespace = "git-cache"
	registryPodLabel  = "component=registry"
	defaultQueue      = "default"
	chipNameLabel     = "node.kubernetes.io/npu.chip.name"
	storageClassSFSTurbo = "sfsturbo-subpath-sc"
	storageClassCSINAS  = "csi-nas"
)

func preSubmitValidate(cfg Config, workflowPath string) (string, error) {
	fmt.Println("=== Pre-submit validation ===")

	data, err := os.ReadFile(workflowPath)
	if err != nil {
		return workflowPath, fmt.Errorf("failed to read workflow: %w", err)
	}

	var job map[string]interface{}
	if err := yaml.Unmarshal(data, &job); err != nil {
		return workflowPath, fmt.Errorf("failed to parse workflow: %w", err)
	}

	if harborExists(cfg) {
		fmt.Println("[OK] Harbor registry found, applying image proxy replacement")
		job = applyImageProxy(cfg, job)
	} else {
		fmt.Println("[SKIP] Harbor not found, keeping original image URLs")
	}

	if gitCacheExists(cfg) {
		fmt.Println("[OK] Git-cache found, injecting git proxy config")
		job = applyGitProxy(job)
	} else {
		fmt.Println("[SKIP] Git-cache not found, keeping original script")
	}

	queueName := extractQueueName(job)
	if queueName != "" {
		if queueExists(cfg, queueName) {
			fmt.Printf("[OK] Queue '%s' exists in cluster\n", queueName)
		} else {
			fmt.Printf("[WARN] Queue '%s' not found, replacing with 'default'\n", queueName)
			job = replaceQueue(job, defaultQueue)
		}
	} else {
		fmt.Println("[SKIP] No queue specified in workflow")
	}

	if err := validateResources(cfg, job); err != nil {
		return workflowPath, err
	}

	modifiedData, err := yaml.Marshal(job)
	if err != nil {
		return workflowPath, fmt.Errorf("failed to marshal workflow: %w", err)
	}

	validatedPath := workflowPath + ".validated"
	if err := os.WriteFile(validatedPath, modifiedData, 0644); err != nil {
		return workflowPath, fmt.Errorf("failed to write validated workflow: %w", err)
	}

	fmt.Printf("[DONE] Created validated workflow: %s\n", validatedPath)
	return validatedPath, nil
}

func validateResources(cfg Config, job map[string]interface{}) error {
	// StorageClass validation
	if hasEphemeralVolume(job) {
		fmt.Printf("[INFO] Checking StorageClasses\n")
		
		scExistsFunc := storageClassExists
		if isKarmadaCluster(cfg) {
			scExistsFunc = storageClassExistsInMemberClusters
		}
		
		if scExistsFunc(cfg, storageClassSFSTurbo) {
			fmt.Printf("[OK] StorageClass '%s' exists\n", storageClassSFSTurbo)
		} else if scExistsFunc(cfg, storageClassCSINAS) {
			fmt.Printf("[WARN] StorageClass '%s' not found, using '%s' instead\n", storageClassSFSTurbo, storageClassCSINAS)
			job = replaceStorageClass(job, storageClassSFSTurbo, storageClassCSINAS)
		} else {
			fmt.Printf("[SKIP] Neither '%s' nor '%s' StorageClass found, keeping original\n", storageClassSFSTurbo, storageClassCSINAS)
		}
	} else {
		fmt.Println("[SKIP] No ephemeral volumes in workflow")
	}

	// Chip validation
	chipName := extractChipNameFromJob(job)
	if chipName != "" {
		if isKarmadaCluster(cfg) {
			fmt.Printf("[INFO] Checking chip '%s' across member clusters\n", chipName)
			chipCluster, err := getNodeChipClusterMember(cfg, chipName)
			if err != nil {
				return fmt.Errorf("[FAIL] Chip validation failed: %w", err)
			}
			if chipCluster == "" {
				fmt.Printf("[WARN] Chip '%s' found in multiple clusters, will rely on dispatch labels\n", chipName)
			} else {
				fmt.Printf("[OK] Chip '%s' found in cluster '%s'\n", chipName, chipCluster)
			}
		} else {
			if chipNodesExist(cfg, chipName) {
				fmt.Printf("[OK] Nodes with chip '%s' found in cluster\n", chipName)
			} else {
				return fmt.Errorf("[FAIL] No nodes found with chip label '%s', cannot submit workflow", chipName)
			}
		}
	} else {
		fmt.Println("[SKIP] No chip-specific nodeSelector in workflow")
	}

	// PVC validation
	pvcNames := extractPVCNamesFromJob(job)
	if len(pvcNames) > 0 {
		if isKarmadaCluster(cfg) {
			fmt.Printf("[INFO] Checking PVCs: %v across member clusters\n", pvcNames)
			pvcClusters, err := getPVCClusterMembers(cfg, pvcNames)
			if err != nil {
				return fmt.Errorf("[FAIL] PVC validation failed: %w", err)
			}
			for pvcName, cluster := range pvcClusters {
				if cluster != "" {
					fmt.Printf("[OK] PVC '%s' found in cluster '%s'\n", pvcName, cluster)
				} else {
					fmt.Printf("[WARN] PVC '%s' found in multiple clusters, will rely on dispatch labels\n", pvcName)
				}
			}
		} else {
			fmt.Printf("[INFO] Found PVCs: %v\n", pvcNames)
			for _, pvcName := range pvcNames {
				if pvcExists(cfg, pvcName) {
					fmt.Printf("[OK] PVC '%s' exists in namespace '%s'\n", pvcName, cfg.Namespace)
				} else {
					return fmt.Errorf("[FAIL] PVC '%s' not found in namespace '%s', cannot submit workflow", pvcName, cfg.Namespace)
				}
			}
		}
	} else {
		fmt.Println("[SKIP] No PVC mounts in workflow")
	}

	return nil
}

func harborExists(cfg Config) bool {
	output, err := execKubectl(cfg, "get", "pods", "-n", harborNamespace, "-l", registryPodLabel, "-o", "jsonpath={.items[*].status.phase}")
	if err != nil {
		return false
	}
	phases := strings.TrimSpace(string(output))
	if phases == "" {
		return false
	}
	for _, phase := range strings.Split(phases, " ") {
		if phase == "Running" {
			return true
		}
	}
	return false
}

func applyImageProxy(cfg Config, job map[string]interface{}) map[string]interface{} {
	proxyURL := cfg.ImageProxyURL
	if proxyURL == "" {
		proxyURL = "harbor-portal.osinfra.cn"
	}

	imageReplacements := map[string]string{
		"swr.cn-north-4.myhuaweicloud.com/": proxyURL + "/north4-myhuaweicloud/",
	}

	spec, ok := job["spec"].(map[string]interface{})
	if !ok {
		return job
	}

	tasks, ok := spec["tasks"].([]interface{})
	if !ok {
		return job
	}

	for _, task := range tasks {
		taskMap, ok := task.(map[string]interface{})
		if !ok {
			continue
		}

		template, ok := taskMap["template"].(map[string]interface{})
		if !ok {
			continue
		}

		specInner, ok := template["spec"].(map[string]interface{})
		if !ok {
			continue
		}

		containers, ok := specInner["containers"].([]interface{})
		if !ok || len(containers) == 0 {
			continue
		}

		container, ok := containers[0].(map[string]interface{})
		if !ok {
			continue
		}

		if image, ok := container["image"].(string); ok {
			for old, new := range imageReplacements {
				if strings.Contains(image, old) {
					container["image"] = strings.Replace(image, old, new, 1)
					fmt.Printf("[IMAGE] Replaced: %s -> %s\n", old, new)
				}
			}
		}
	}

	return job
}

func gitCacheExists(cfg Config) bool {
	output, err := execKubectl(cfg, "get", "pods", "-n", gitCacheNamespace, "-o", "jsonpath={.items[*].status.phase}")
	if err != nil {
		return false
	}
	phases := strings.TrimSpace(string(output))
	if phases == "" {
		return false
	}
	for _, phase := range strings.Split(phases, " ") {
		if phase == "Running" {
			return true
		}
	}
	return false
}

func applyGitProxy(job map[string]interface{}) map[string]interface{} {
	gitProxyConfig := `git config --global url."http://git-cache-http-server.git-cache.svc.cluster.local:8080".insteadOf "https://gitcode.com" || echo "WARNING: git config failed for gitcode"
git config --global url."http://git-cache-github.git-cache.svc.cluster.local:8080".insteadOf "https://gh-proxy.test.osinfra.cn/https://github.com" || echo "WARNING: git config failed for github"
git config --global url."http://git-cache-gitee.git-cache.svc.cluster.local:8080".insteadOf "https://gitee.com" || echo "WARNING: git config failed for gitee"
git config --global url."http://git-cache-atomgit.git-cache.svc.cluster.local:8080".insteadOf "https://atomgit.com" || echo "WARNING: git config failed for atomgit"
git config --global url."http://git-cache-codehub.git-cache.svc.cluster.local:8080".insteadOf "https://codehub.devcloud.cn-north-4.huaweicloud.com" || echo "WARNING: git config failed for codehub"`

	spec, ok := job["spec"].(map[string]interface{})
	if !ok {
		return job
	}

	tasks, ok := spec["tasks"].([]interface{})
	if !ok {
		return job
	}

	for _, task := range tasks {
		taskMap, ok := task.(map[string]interface{})
		if !ok {
			continue
		}

		template, ok := taskMap["template"].(map[string]interface{})
		if !ok {
			continue
		}

		specInner, ok := template["spec"].(map[string]interface{})
		if !ok {
			continue
		}

		taskName, ok := taskMap["name"].(string)
		if !ok {
			continue
		}

		if taskName != "main-script" && taskName != "task0" {
			continue
		}

		containers, ok := specInner["containers"].([]interface{})
		if !ok || len(containers) == 0 {
			continue
		}

		container, ok := containers[0].(map[string]interface{})
		if !ok {
			continue
		}

		args, ok := container["args"].([]interface{})
		if !ok || len(args) == 0 {
			continue
		}

		// Check if args contain git config already
		argsContent := ""
		for _, arg := range args {
			if str, ok := arg.(string); ok {
				argsContent += str + "\n"
			}
		}

		if strings.Contains(argsContent, "git config --global url") {
			continue
		}

		// Prepend git proxy config to the first arg (block scalar format)
		if len(args) > 0 {
			if firstArg, ok := args[0].(string); ok {
				newFirstArg := gitProxyConfig + "\n\n" + firstArg
				args[0] = newFirstArg
				container["args"] = args
				fmt.Printf("[GIT] Injected git proxy config into %s\n", taskName)
			}
		}
	}

	return job
}

func extractQueueName(job map[string]interface{}) string {
	spec, ok := job["spec"].(map[string]interface{})
	if !ok {
		return ""
	}

	queue, ok := spec["queue"].(string)
	if !ok {
		return ""
	}

	return queue
}

func queueExists(cfg Config, queueName string) bool {
	_, err := execKubectl(cfg, "get", "queue", queueName)
	return err == nil
}

func replaceQueue(job map[string]interface{}, newQueue string) map[string]interface{} {
	spec, ok := job["spec"].(map[string]interface{})
	if !ok {
		return job
	}

	spec["queue"] = newQueue
	return job
}

func extractChipNameFromJob(job map[string]interface{}) string {
	spec, ok := job["spec"].(map[string]interface{})
	if !ok {
		return ""
	}

	tasks, ok := spec["tasks"].([]interface{})
	if !ok {
		return ""
	}

	for _, task := range tasks {
		taskMap, ok := task.(map[string]interface{})
		if !ok {
			continue
		}

		template, ok := taskMap["template"].(map[string]interface{})
		if !ok {
			continue
		}

		specInner, ok := template["spec"].(map[string]interface{})
		if !ok {
			continue
		}

		nodeSelector, ok := specInner["nodeSelector"].(map[string]interface{})
		if !ok {
			continue
		}

		if chipName, exists := nodeSelector[chipNameLabel]; exists {
			if chipStr, ok := chipName.(string); ok {
				return chipStr
			}
		}
	}

	return ""
}

func chipNodesExist(cfg Config, chipName string) bool {
	labelSelector := fmt.Sprintf("%s=%s", chipNameLabel, chipName)
	output, err := execKubectl(cfg, "get", "nodes", "-l", labelSelector, "-o", "jsonpath={.items[*].metadata.name}")
	if err != nil {
		return false
	}
	nodes := strings.TrimSpace(string(output))
	return nodes != ""
}

func extractPVCNamesFromJob(job map[string]interface{}) []string {
	pvcNames := []string{}

	spec, ok := job["spec"].(map[string]interface{})
	if !ok {
		return pvcNames
	}

	tasks, ok := spec["tasks"].([]interface{})
	if !ok {
		return pvcNames
	}

	for _, task := range tasks {
		taskMap, ok := task.(map[string]interface{})
		if !ok {
			continue
		}

		template, ok := taskMap["template"].(map[string]interface{})
		if !ok {
			continue
		}

		specInner, ok := template["spec"].(map[string]interface{})
		if !ok {
			continue
		}

		volumes, ok := specInner["volumes"].([]interface{})
		if !ok {
			continue
		}

		for _, volume := range volumes {
			volumeMap, ok := volume.(map[string]interface{})
			if !ok {
				continue
			}

			pvc, ok := volumeMap["persistentVolumeClaim"].(map[string]interface{})
			if !ok {
				continue
			}

			claimName, ok := pvc["claimName"].(string)
			if !ok {
				continue
			}

			if claimName != "" {
				pvcNames = append(pvcNames, claimName)
			}
		}
	}

	return pvcNames
}

func pvcExists(cfg Config, pvcName string) bool {
	_, err := execKubectl(cfg, "get", "pvc", pvcName, "-n", cfg.Namespace)
	return err == nil
}

func hasEphemeralVolume(job map[string]interface{}) bool {
	spec, ok := job["spec"].(map[string]interface{})
	if !ok {
		return false
	}

	tasks, ok := spec["tasks"].([]interface{})
	if !ok {
		return false
	}

	for _, task := range tasks {
		taskMap, ok := task.(map[string]interface{})
		if !ok {
			continue
		}

		template, ok := taskMap["template"].(map[string]interface{})
		if !ok {
			continue
		}

		specInner, ok := template["spec"].(map[string]interface{})
		if !ok {
			continue
		}

		volumes, ok := specInner["volumes"].([]interface{})
		if !ok {
			continue
		}

		for _, volume := range volumes {
			volumeMap, ok := volume.(map[string]interface{})
			if !ok {
				continue
			}

			if _, ok := volumeMap["ephemeral"]; ok {
				return true
			}
		}
	}

	return false
}

func storageClassExists(cfg Config, scName string) bool {
	_, err := execKubectl(cfg, "get", "storageclass", scName)
	return err == nil
}

func storageClassExistsInMemberClusters(cfg Config, scName string) bool {
	clusters, err := getKarmadaMemberClusters(cfg)
	if err != nil {
		return false
	}

	for _, cluster := range clusters {
		rawPath := fmt.Sprintf("/apis/cluster.karmada.io/v1alpha1/clusters/%s/proxy/apis/storage.k8s.io/v1/storageclasses/%s", cluster, scName)
		output, err := execKubectl(cfg, "get", "--raw", rawPath)
		if err == nil && len(output) > 0 {
			return true
		}
	}

	return false
}

func replaceStorageClass(job map[string]interface{}, oldSC, newSC string) map[string]interface{} {
	spec, ok := job["spec"].(map[string]interface{})
	if !ok {
		return job
	}

	tasks, ok := spec["tasks"].([]interface{})
	if !ok {
		return job
	}

	for _, task := range tasks {
		taskMap, ok := task.(map[string]interface{})
		if !ok {
			continue
		}

		template, ok := taskMap["template"].(map[string]interface{})
		if !ok {
			continue
		}

		specInner, ok := template["spec"].(map[string]interface{})
		if !ok {
			continue
		}

		volumes, ok := specInner["volumes"].([]interface{})
		if !ok {
			continue
		}

		for _, volume := range volumes {
			volumeMap, ok := volume.(map[string]interface{})
			if !ok {
				continue
			}

			ephemeral, ok := volumeMap["ephemeral"].(map[string]interface{})
			if !ok {
				continue
			}

			volumeClaimTemplate, ok := ephemeral["volumeClaimTemplate"].(map[string]interface{})
			if !ok {
				continue
			}

			pvcSpec, ok := volumeClaimTemplate["spec"].(map[string]interface{})
			if !ok {
				continue
			}

			if scName, ok := pvcSpec["storageClassName"].(string); ok && scName == oldSC {
				pvcSpec["storageClassName"] = newSC
				fmt.Printf("[STORAGECLASS] Replaced: %s -> %s\n", oldSC, newSC)
			}
		}
	}

	return job
}
