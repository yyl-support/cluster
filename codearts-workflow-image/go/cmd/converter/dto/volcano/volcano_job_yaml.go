package volcano

type Job struct {
	APIVersion string   `yaml:"apiVersion"`
	Kind       string   `yaml:"kind"`
	Metadata   Metadata `yaml:"metadata"`
	Spec       JobSpec  `yaml:"spec"`
}

type Metadata struct {
	GenerateName string            `yaml:"generateName,omitempty"`
	Labels       map[string]string `yaml:"labels,omitempty"`
	Annotations  map[string]string `yaml:"annotations,omitempty"`
}

type JobSpec struct {
	Policies                []Policy   `yaml:"policies,omitempty"`
	Queue                   string     `yaml:"queue"`
	MaxRetry                int        `yaml:"maxRetry"`
	MinAvailable            int        `yaml:"minAvailable,omitempty"`
	SchedulerName           string     `yaml:"schedulerName,omitempty"`
	TTLSecondsAfterFinished int        `yaml:"ttlSecondsAfterFinished,omitempty"`
	Tasks                   []TaskSpec `yaml:"tasks"`
}

type Policy struct {
	Event  string `yaml:"event"`
	Action string `yaml:"action"`
}

type TaskSpec struct {
	Name         string          `yaml:"name"`
	Replicas     int             `yaml:"replicas"`
	MaxRetry     int             `yaml:"maxRetry,omitempty"`
	MinAvailable int             `yaml:"minAvailable,omitempty"`
	Template     PodTemplateSpec `yaml:"template"`
}

type PodTemplateSpec struct {
	Metadata Metadata `yaml:"metadata,omitempty"`
	Spec     PodSpec  `yaml:"spec"`
}

type PodSpec struct {
	Containers            []Container            `yaml:"containers"`
	NodeSelector          map[string]string      `yaml:"nodeSelector,omitempty"`
	Affinity              *Affinity              `yaml:"affinity,omitempty"`
	ImagePullSecrets      []LocalObjectReference `yaml:"imagePullSecrets,omitempty"`
	ActiveDeadlineSeconds int64                  `yaml:"activeDeadlineSeconds,omitempty"`
	SecurityContext       *PodSecurityContext    `yaml:"securityContext,omitempty"`
	Volumes               []Volume               `yaml:"volumes,omitempty"`
	RestartPolicy         string                 `yaml:"restartPolicy,omitempty"`
}

type Affinity struct {
	NodeAffinity *NodeAffinity `yaml:"nodeAffinity,omitempty"`
}

type NodeAffinity struct {
	RequiredDuringSchedulingIgnoredDuringExecution  *NodeSelector             `yaml:"requiredDuringSchedulingIgnoredDuringExecution,omitempty"`
	PreferredDuringSchedulingIgnoredDuringExecution []PreferredSchedulingTerm `yaml:"preferredDuringSchedulingIgnoredDuringExecution,omitempty"`
}

type NodeSelector struct {
	NodeSelectorTerms []NodeSelectorTerm `yaml:"nodeSelectorTerms"`
}

type NodeSelectorTerm struct {
	MatchExpressions []NodeSelectorRequirement `yaml:"matchExpressions,omitempty"`
	MatchFields      []NodeSelectorRequirement `yaml:"matchFields,omitempty"`
}

type NodeSelectorRequirement struct {
	Key      string   `yaml:"key"`
	Operator string   `yaml:"operator"`
	Values   []string `yaml:"values,omitempty"`
}

type PreferredSchedulingTerm struct {
	Weight     int64            `yaml:"weight"`
	Preference NodeSelectorTerm `yaml:"preference"`
}

type Container struct {
	Name            string        `yaml:"name"`
	Image           string        `yaml:"image"`
	ImagePullPolicy string        `yaml:"imagePullPolicy,omitempty"`
	Command         []string      `yaml:"command"`
	Args            []string      `yaml:"args"`
	WorkingDir      string        `yaml:"workingDir,omitempty"`
	Resources       Resources     `yaml:"resources,omitempty"`
	VolumeMounts    []VolumeMount `yaml:"volumeMounts,omitempty"`
	Env             []EnvVar      `yaml:"env,omitempty"`
}

type Resources struct {
	Limits   ResourceList `yaml:"limits,omitempty"`
	Requests ResourceList `yaml:"requests,omitempty"`
}

type ResourceList map[string]string

type VolumeMount struct {
	Name      string `yaml:"name"`
	MountPath string `yaml:"mountPath"`
	ReadOnly  bool   `yaml:"readOnly,omitempty"`
}

type Volume struct {
	Name                  string                       `yaml:"name,omitempty"`
	HostPath              *HostPath                    `yaml:"hostPath,omitempty"`
	PersistentVolumeClaim *PersistentVolumeClaimVolume `yaml:"persistentVolumeClaim,omitempty"`
	EmptyDir              *EmptyDir                    `yaml:"emptyDir,omitempty"`
	Ephemeral             *EphemeralVolume             `yaml:"ephemeral,omitempty"`
}

type EphemeralVolume struct {
	VolumeClaimTemplate VolumeClaimTemplate `yaml:"volumeClaimTemplate"`
}

type VolumeClaimTemplate struct {
	Spec PersistentVolumeClaimSpec `yaml:"spec"`
}

type PersistentVolumeClaimSpec struct {
	AccessModes      []string        `yaml:"accessModes"`
	StorageClassName string          `yaml:"storageClassName,omitempty"`
	Resources        ResourceRequest `yaml:"resources"`
}

type ResourceRequest struct {
	Requests map[string]string `yaml:"requests"`
	EmptyDir *EmptyDir         `yaml:"emptyDir,omitempty"`
}

type HostPath struct {
	Path string `yaml:"path"`
	Type string `yaml:"type,omitempty"`
}

type PersistentVolumeClaimVolume struct {
	ClaimName string `yaml:"claimName"`
}

type EmptyDir struct {
	Medium    string `yaml:"medium,omitempty"`
	SizeLimit string `yaml:"sizeLimit,omitempty"`
}

type EnvVar struct {
	Name      string        `yaml:"name" json:"name"`
	Value     string        `yaml:"value,omitempty" json:"value,omitempty"`
	ValueFrom *EnvVarSource `yaml:"valueFrom,omitempty" json:"valueFrom,omitempty"`
}

type EnvVarSource struct {
	SecretKeyRef *SecretKeySelector `yaml:"secretKeyRef,omitempty" json:"secretKeyRef,omitempty"`
}

type SecretKeySelector struct {
	Name     string `yaml:"name" json:"name"`
	Key      string `yaml:"key" json:"key"`
	Optional *bool  `yaml:"optional,omitempty" json:"optional,omitempty"`
}

type LocalObjectReference struct {
	Name string `yaml:"name"`
}

type PodSecurityContext struct {
	RunAsUser *int64   `yaml:"runAsUser,omitempty"`
	Sysctls   []Sysctl `yaml:"sysctls,omitempty"`
}

type Sysctl struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}
