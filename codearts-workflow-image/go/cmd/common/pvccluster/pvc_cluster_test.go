package pvccluster

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type mockExecutor struct {
	responses map[string][]byte
	errors    map[string]error
	callCount int
}

func (m *mockExecutor) Exec(args ...string) ([]byte, error) {
	m.callCount++
	key := strings.Join(args, " ")
	if m.errors != nil && m.errors[key] != nil {
		return nil, m.errors[key]
	}
	if m.responses != nil && m.responses[key] != nil {
		return m.responses[key], nil
	}
	return nil, nil
}

func (m *mockExecutor) ExecWithContext(ctx context.Context, args ...string) ([]byte, error) {
	return m.Exec(args...)
}

func TestExtractPVCClaimNames(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		wantClaims  []string
		wantErr     bool
	}{
		{
			name: "single PVC",
			yamlContent: `apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
  name: test-job
spec:
  tasks:
    - name: main
      template:
        spec:
          volumes:
            - name: dataset
              persistentVolumeClaim:
                claimName: testorg-testrepo-test15
`,
			wantClaims: []string{"testorg-testrepo-test15"},
			wantErr:    false,
		},
		{
			name: "multiple PVCs in different tasks",
			yamlContent: `apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
  name: test-job
spec:
  tasks:
    - name: main
      template:
        spec:
          volumes:
            - name: dataset
              persistentVolumeClaim:
                claimName: dataset-pvc
    - name: copy
      template:
        spec:
          volumes:
            - name: output
              persistentVolumeClaim:
                claimName: output-pvc
`,
			wantClaims: []string{"dataset-pvc", "output-pvc"},
			wantErr:    false,
		},
		{
			name: "no PVC volumes",
			yamlContent: `apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
  name: test-job
spec:
  tasks:
    - name: main
      template:
        spec:
          volumes:
            - name: config
              hostPath:
                path: /config
`,
			wantClaims: []string{},
			wantErr:    false,
		},
		{
			name: "duplicate PVC claimName",
			yamlContent: `apiVersion: batch.volcano.sh/v1alpha1
kind: Job
metadata:
  name: test-job
spec:
  tasks:
    - name: main
      template:
        spec:
          volumes:
            - name: dataset1
              persistentVolumeClaim:
                claimName: shared-pvc
            - name: dataset2
              persistentVolumeClaim:
                claimName: shared-pvc
`,
			wantClaims: []string{"shared-pvc"},
			wantErr:    false,
		},
		{
			name: "unsupported kind",
			yamlContent: `apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  name: test-workflow
`,
			wantClaims: nil,
			wantErr:    true,
		},
		{
			name:        "empty yaml",
			yamlContent: "",
			wantClaims:  nil,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			yamlPath := filepath.Join(tmpDir, "workflow.yaml")
			if tt.yamlContent != "" {
				if err := os.WriteFile(yamlPath, []byte(tt.yamlContent), 0644); err != nil {
					t.Fatalf("failed to write test yaml: %v", err)
				}
			}

			claims, err := ExtractPVCClaimNames(yamlPath)
			if tt.wantErr {
				if err == nil {
					t.Errorf("ExtractPVCClaimNames() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("ExtractPVCClaimNames() unexpected error: %v", err)
				return
			}

			if len(claims) != len(tt.wantClaims) {
				t.Errorf("ExtractPVCClaimNames() got %d claims, want %d", len(claims), len(tt.wantClaims))
				return
			}

			for _, want := range tt.wantClaims {
				found := false
				for _, got := range claims {
					if got == want {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("ExtractPVCClaimNames() missing claim %s", want)
				}
			}
		})
	}
}

func TestIsKarmadaCluster(t *testing.T) {
	tests := []struct {
		name    string
		crdErr  error
		want    bool
	}{
		{
			name:   "karmada cluster - CRD exists",
			crdErr: nil,
			want:   true,
		},
		{
			name:   "non-karmada cluster - CRD not found",
			crdErr: errors.New("crd not found"),
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				responses: map[string][]byte{},
				errors:    map[string]error{},
			}
			crdKey := "get crd resourcebindings.work.karmada.io"
			if tt.crdErr != nil {
				executor.errors[crdKey] = tt.crdErr
			} else {
				executor.responses[crdKey] = []byte("exists")
			}

			mgr := NewPVCClusterManager(executor, "argo", "")

			got := mgr.IsKarmadaCluster()
			if got != tt.want {
				t.Errorf("IsKarmadaCluster() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetPVCClusterMemberImmediate(t *testing.T) {
	tests := []struct {
		name       string
		isKarmada  bool
		pvcName    string
		clusterRes string
		want       string
		wantErr    bool
	}{
		{
			name:      "non-karmada returns empty",
			isKarmada: false,
			pvcName:   "test-pvc",
			want:      "",
			wantErr:   false,
		},
		{
			name:       "karmada with cluster",
			isKarmada:  true,
			pvcName:    "test-pvc",
			clusterRes: "external-gy-006",
			want:       "external-gy-006",
			wantErr:    false,
		},
		{
			name:      "karmada no cluster found",
			isKarmada: true,
			pvcName:   "test-pvc",
			want:      "",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			executor := &mockExecutor{
				responses: map[string][]byte{},
				errors:    map[string]error{},
			}

			crdKey := "get crd resourcebindings.work.karmada.io"
			bindingKey := strings.Join([]string{
				"get", "resourcebinding", tt.pvcName,
				"-n", "argo",
				"-o", "jsonpath={.spec.clusters[0].name}",
			}, " ")

			if !tt.isKarmada {
				executor.errors[crdKey] = errors.New("not found")
			} else {
				executor.responses[crdKey] = []byte("exists")
				if tt.clusterRes != "" {
					executor.responses[bindingKey] = []byte(tt.clusterRes)
				} else {
					executor.errors[bindingKey] = errors.New("not found")
				}
			}

			mgr := NewPVCClusterManager(executor, "argo", "")
			mgr.karmadaCheck = false

			got, err := mgr.GetPVCClusterMemberImmediate(tt.pvcName)
			if tt.wantErr {
				if err == nil {
					t.Errorf("GetPVCClusterMemberImmediate() expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("GetPVCClusterMemberImmediate() unexpected error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("GetPVCClusterMemberImmediate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildClusterLabelPatch(t *testing.T) {
	tests := []struct {
		name     string
		clusters map[string]string
		want     string
	}{
		{
			name:     "empty clusters",
			clusters: map[string]string{},
			want:     "",
		},
		{
			name: "single cluster",
			clusters: map[string]string{
				"pvc1": "cluster-a",
			},
			want: `{"metadata":{"labels":{"dispatch/cluster-a":"true"}}}`,
		},
		{
			name: "multiple clusters",
			clusters: map[string]string{
				"pvc1": "cluster-a",
				"pvc2": "cluster-b",
			},
			want: `{"metadata":{"labels":{"dispatch/cluster-a":"true","dispatch/cluster-b":"true"}}}`,
		},
		{
			name: "same cluster for multiple PVCs",
			clusters: map[string]string{
				"pvc1": "cluster-a",
				"pvc2": "cluster-a",
			},
			want: `{"metadata":{"labels":{"dispatch/cluster-a":"true"}}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildClusterLabelPatch(tt.clusters)
			if got != tt.want {
				t.Errorf("BuildClusterLabelPatch() = %v, want %v", got, tt.want)
			}
		})
	}
}