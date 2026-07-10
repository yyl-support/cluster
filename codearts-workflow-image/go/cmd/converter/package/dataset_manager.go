package converter

type DatasetManager struct {
	mapping map[string]string
}

var defaultDatasetManager = &DatasetManager{
	mapping: map[string]string{
		"testorg-testrepo-test16":          "testorg-testrepo-test15",
		"testorg-testrepo-test21":          "testorg-testrepo-test15",
		"ascend-op-plugin":                 "ascend-op-plugin",
		"ascend-pytorch":                   "ascend-op-plugin",
		"ascend-text-embeddings-inference": "ascend-ragsdk",
		"ascend-ragsdk":                    "ascend-ragsdk",
		"npu-ir-cicd":                      "ascend-ascendnpu-ir",
		"ascend-ascendnpu-ir":              "ascend-ascendnpu-ir",
	},
}

func NewDatasetManager() *DatasetManager {
	return &DatasetManager{
		mapping: make(map[string]string),
	}
}

func NewDatasetManagerWithMapping(mapping map[string]string) *DatasetManager {
	dm := NewDatasetManager()
	for k, v := range mapping {
		dm.mapping[k] = v
	}
	return dm
}

func (dm *DatasetManager) SetClaimName(repoName, claimName string) {
	dm.mapping[repoName] = claimName
}

func (dm *DatasetManager) GetClaimName(repoName string) string {
	if claimName, ok := dm.mapping[repoName]; ok {
		return claimName
	}
	return repoName
}

func GetDatasetClaimName(repoName string) string {
	return defaultDatasetManager.GetClaimName(repoName)
}
