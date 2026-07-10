package converter

import (
	"testing"
)

func TestDatasetManagerGetClaimName(t *testing.T) {
	tests := []struct {
		name      string
		mapping   map[string]string
		repoName  string
		wantClaim string
	}{
		{
			name:      "empty mapping returns repoName as default",
			mapping:   nil,
			repoName:  "my-repo",
			wantClaim: "my-repo",
		},
		{
			name:      "mapping with key returns mapped value",
			mapping:   map[string]string{"my-repo": "custom-pvc"},
			repoName:  "my-repo",
			wantClaim: "custom-pvc",
		},
		{
			name:      "mapping without key returns repoName as default",
			mapping:   map[string]string{"other-repo": "other-pvc"},
			repoName:  "my-repo",
			wantClaim: "my-repo",
		},
		{
			name:      "empty repoName returns empty",
			mapping:   nil,
			repoName:  "",
			wantClaim: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dm := NewDatasetManagerWithMapping(tt.mapping)
			got := dm.GetClaimName(tt.repoName)
			if got != tt.wantClaim {
				t.Errorf("GetClaimName(%q) = %q, want %q", tt.repoName, got, tt.wantClaim)
			}
		})
	}
}

func TestDatasetManagerSetAndGet(t *testing.T) {
	dm := NewDatasetManager()
	dm.SetClaimName("repo1", "pvc1")
	dm.SetClaimName("repo2", "pvc2")

	if got := dm.GetClaimName("repo1"); got != "pvc1" {
		t.Errorf("GetClaimName(repo1) = %q, want pvc1", got)
	}
	if got := dm.GetClaimName("repo2"); got != "pvc2" {
		t.Errorf("GetClaimName(repo2) = %q, want pvc2", got)
	}
	if got := dm.GetClaimName("repo3"); got != "repo3" {
		t.Errorf("GetClaimName(repo3) = %q, want repo3 (default)", got)
	}
}

func TestGetDatasetClaimName(t *testing.T) {
	dm := NewDatasetManagerWithMapping(map[string]string{
		"mapped-repo": "mapped-pvc",
	})
	oldDefault := defaultDatasetManager
	defaultDatasetManager = dm
	defer func() { defaultDatasetManager = oldDefault }()

	if got := GetDatasetClaimName("mapped-repo"); got != "mapped-pvc" {
		t.Errorf("GetDatasetClaimName(mapped-repo) = %q, want mapped-pvc", got)
	}
	if got := GetDatasetClaimName("unmapped-repo"); got != "unmapped-repo" {
		t.Errorf("GetDatasetClaimName(unmapped-repo) = %q, want unmapped-repo", got)
	}
}
