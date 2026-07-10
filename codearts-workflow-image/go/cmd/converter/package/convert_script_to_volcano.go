package converter

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	runonparser "github.com/opensourceways/codearts-workflow-image-go/cmd/common"
	"github.com/opensourceways/codearts-workflow-image-go/cmd/converter/dto/volcano"
	"go.yaml.in/yaml/v3"
)

type VolcanoConversionResult struct {
	Job            volcano.Job
	SecretManifest string
}

func ConvertScriptToVolcano(
	scriptContent string,
	runsOn string,
	dockerImage string,
	envVars map[string]string,
	pipelineRunID string,
	mergeID string,
	repoURL string,
	targetBranch string,
	uniqueID string,
	yamlTemplatePath string,
	cpDataset string,
	cpArtifacts string,
	cpArtifactsTempFolder string,
	cpTimeoutSeconds int,
	cpShm string,
	cpBandwidth string,
	cpImagePullPolicy string,
	cpDelayExitSeconds int,
) VolcanoConversionResult {

	dir := filepath.Dir(yamlTemplatePath)
	if dir == "." {
		dir = ""
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		fmt.Printf("fail to open root directory: %v\n", err)
		os.Exit(1)
	}

	filename := filepath.Base(yamlTemplatePath)
	f, err := root.Open(filename)
	if err != nil {
		root.Close() //nolint:errcheck
		fmt.Printf("fail to load yaml template %s : %v\n", yamlTemplatePath, err)
		os.Exit(1)
	}
	defer root.Close() //nolint:errcheck
	defer f.Close()    //nolint:errcheck
	data, err := io.ReadAll(f)
	if err != nil {
		fmt.Printf("fail to load yaml template %s : %v\n", yamlTemplatePath, err)
		os.Exit(1) //nolint:gocritic
	}

	var volcanoJob volcano.Job
	err = yaml.Unmarshal(data, &volcanoJob)
	if err != nil {
		fmt.Print(err.Error())
		os.Exit(1) //nolint:gocritic
	}

	task := volcanoJob.Spec.Tasks[0]
	container := task.Template.Spec.Containers[0]

	volcanoJob.Spec.Queue = DetermineQueue(runsOn)

	repoName := ""
	if repoURL != "" {
		repoName = extractOrgRepoVolcano(repoURL)
	}

	setupVolcanoLabels(&volcanoJob, pipelineRunID, mergeID, repoName)

	task.Name = "main-script"
	task.Replicas = 1

	gitCacheURLs := map[string]string{
		"gitcode": "http://git-cache-http-server.git-cache.svc.cluster.local:8080",
		"github":  "http://git-cache-github.git-cache.svc.cluster.local:8080",
		"gitee":   "http://git-cache-gitee.git-cache.svc.cluster.local:8080",
		"atomgit": "http://git-cache-atomgit.git-cache.svc.cluster.local:8080",
		"codehub": "http://git-cache-codehub.git-cache.svc.cluster.local:8080",
	}
	artifactsReq := artifactsRequest{
		Artifacts:           cpArtifacts,
		ArtifactsTempFolder: GetArtifactMountPath(cpArtifactsTempFolder),
	}

	script := handlerScript(
		scriptContent,
		gitRequest{RepoURL: repoURL, MergeID: mergeID, TargetBranch: targetBranch, GitCacheURLs: gitCacheURLs},
		artifactsReq,
		delayExitRequest{DelayExitSeconds: cpDelayExitSeconds},
	)

	container.Command = []string{"bash", "-c"}
	container.Args = []string{script}

	if dockerImage == "" {
		dockerImage = "swr.cn-southwest-2.myhuaweicloud.com/modelfoundry/git:latest"
	}
	container.Image = dockerImage
	if cpImagePullPolicy != "" {
		container.ImagePullPolicy = cpImagePullPolicy
	}

	task.Template.Spec.NodeSelector = convertJobArch(runsOn)

	AddNPUAffinity(&task.Template.Spec, runsOn)

	argoResources, err := convertJobResource(runsOn)
	if err != nil {
		fmt.Printf("fail to convert runs_on: %v\n", err)
		os.Exit(1) //nolint:gocritic
	}
	container.Resources = argoResources

	parsedSpec, err := runonparser.Parse(runsOn)
	if err == nil && parsedSpec != nil && !parsedSpec.IsNPUEmpty() {
		addNPUVolumes(&container, &task)
	}

	AddShmVolume(&container, &task, cpShm)
	AddBandwidthAnnotation(&task, cpBandwidth)

	if repoURL != "" {
		volcanoJob.Metadata.GenerateName = repoName + "-"
	} else {
		volcanoJob.Metadata.GenerateName = "job-script-"
	}

	sensitiveFromEnv, plainEnv := FilterSensitiveEnv(envVars)
	plainEnv = AddCustomEnv(plainEnv, argoResources)
	resolvedSensitive := ResolveSensitiveEnvValues(sensitiveFromEnv)

	plainEnvList := convertEnvVolcano(plainEnv)
	container.Env = append(container.Env, plainEnvList...)

	if cpDataset != "" {
		if repoName == "" {
			fmt.Println("fail to configure CP_dataset: missing repo name")
			os.Exit(1) //nolint:gocritic
		}
		datasetPath, datasetReadOnly, err := parseDatasetConfig(cpDataset)
		if err != nil {
			fmt.Printf("fail to parse CP_dataset: %v\n", err)
			os.Exit(1)
		}
		addDatasetVolume(&container, &task, repoName, datasetPath, datasetReadOnly)
	}

	secretManifest := ""
	if len(resolvedSensitive) > 0 {
		secretName := BuildSecretName(pipelineRunID, uniqueID)

		secretEnvVars := ConvertToSecretEnvVolcano(secretName, resolvedSensitive)
		container.Env = append(container.Env, secretEnvVars...)

		secretManifest = BuildSecretManifest(secretName, "", pipelineRunID, resolvedSensitive)
	}

	task.Template.Spec.ActiveDeadlineSeconds = int64(cpTimeoutSeconds)

	task.Template.Spec.SecurityContext = &volcano.PodSecurityContext{
		RunAsUser: ptrInt64(0),
	}

	task.Template.Spec.Containers = []volcano.Container{container}

	if NeedsArtifactMultiTask(cpArtifacts, cpArtifactsTempFolder) {
		mountPath := GetArtifactMountPath(cpArtifactsTempFolder)

		addArtifactVolumeAndSidecar(&task, mountPath, task.Template.Spec.NodeSelector, task.Template.Spec.ImagePullSecrets, task.Template.Spec.ActiveDeadlineSeconds)
	}

	volcanoJob.Spec.Tasks = []volcano.TaskSpec{task}

	AddLabelsFromPodSpec(&volcanoJob)

	return VolcanoConversionResult{
		Job:            volcanoJob,
		SecretManifest: secretManifest,
	}
}

func setupVolcanoLabels(job *volcano.Job, pipelineRunID, mergeID, repoName string) {
	if pipelineRunID == "" && mergeID == "" && repoName == "" {
		return
	}
	job.Metadata.Labels = map[string]string{}
	if pipelineRunID != "" {
		job.Metadata.Labels["pipeline/run-id"] = pipelineRunID
	}
	if mergeID != "" {
		job.Metadata.Labels["jobPRID"] = mergeID
	}
	if repoName != "" {
		job.Metadata.Labels["jobRepositoryName"] = repoName
	}
}

func addNPUVolumes(container *volcano.Container, task *volcano.TaskSpec) {
	container.VolumeMounts = append(container.VolumeMounts, volcano.VolumeMount{
		Name:      "ascend-driver",
		MountPath: "/usr/local/Ascend/driver",
		ReadOnly:  true,
	})
	task.Template.Spec.Volumes = append(task.Template.Spec.Volumes, volcano.Volume{
		Name: "ascend-driver",
		HostPath: &volcano.HostPath{
			Path: "/usr/local/Ascend/driver",
		},
	})
}

func addDatasetVolume(container *volcano.Container, task *volcano.TaskSpec, repoName, cpDataset string, readOnly bool) {
	sharedVolume := volcano.Volume{
		Name: "dataset",
		PersistentVolumeClaim: &volcano.PersistentVolumeClaimVolume{
			ClaimName: GetDatasetClaimName(repoName),
		},
	}
	task.Template.Spec.Volumes = append(task.Template.Spec.Volumes, sharedVolume)
	container.VolumeMounts = append(container.VolumeMounts, volcano.VolumeMount{
		Name:      "dataset",
		MountPath: cpDataset,
		ReadOnly:  readOnly,
	})
}

func parseDatasetConfig(cpDataset string) (mountPath string, readOnly bool, err error) {
	if !strings.Contains(cpDataset, ",") {
		return cpDataset, false, nil
	}
	parts := strings.Split(cpDataset, ",")
	if len(parts) != 2 || parts[1] != "readonly" {
		return "", false, fmt.Errorf("invalid CP_dataset format %q: only /path or /path,readonly supported", cpDataset)
	}
	mountPath = parts[0]
	if strings.HasPrefix(mountPath, "path:") {
		return "", false, fmt.Errorf("invalid CP_dataset: path: prefix not supported")
	}
	if mountPath == "" {
		return "", false, fmt.Errorf("invalid CP_dataset: mount path is empty")
	}
	readOnly = true
	return
}

func ptrInt64(i int64) *int64 {
	return &i
}

func extractOrgRepo(repoURL string) string {
	repoURL = strings.TrimSpace(repoURL)
	repoURL = strings.TrimPrefix(repoURL, "https://")
	repoURL = strings.TrimPrefix(repoURL, "http://")
	repoURL = strings.TrimSuffix(repoURL, ".git")
	parts := strings.Split(repoURL, "/")
	if len(parts) >= 2 {
		org := parts[len(parts)-2]
		repo := parts[len(parts)-1]
		name := org + "-" + repo
		name = strings.ToLower(name)
		name = strings.ReplaceAll(name, "_", "-")
		name = strings.Trim(name, "-")
		return name
	}
	repoURL = strings.ToLower(repoURL)
	repoURL = strings.ReplaceAll(repoURL, "_", "-")
	return strings.Trim(repoURL, "-")
}

func extractOrgRepoVolcano(repoURL string) string {
	return extractOrgRepo(repoURL)
}

func ConvertToSecretEnvVolcano(secretName string, sensitive map[string]string) []volcano.EnvVar {
	envVars := []volcano.EnvVar{}
	for key := range sensitive {
		envVars = append(envVars, volcano.EnvVar{
			Name: key,
			ValueFrom: &volcano.EnvVarSource{
				SecretKeyRef: &volcano.SecretKeySelector{
					Name: secretName,
					Key:  key,
				},
			},
		})
	}
	return envVars
}

func convertEnvVolcano(env map[string]string) []volcano.EnvVar {
	vars := []volcano.EnvVar{}
	for k, v := range env {
		vars = append(vars, volcano.EnvVar{Name: k, Value: v})
	}
	return vars
}
