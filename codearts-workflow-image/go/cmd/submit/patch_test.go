package main

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"
)

type mockPVCKubectl struct {
	callCount   int32
	responses   []mockPVCResponse
	responseIdx int32
}

type mockPVCResponse struct {
	output []byte
	err    error
}

func (m *mockPVCKubectl) exec(ctx context.Context, cfg Config, args ...string) ([]byte, error) {
	atomic.AddInt32(&m.callCount, 1)
	idx := atomic.LoadInt32(&m.responseIdx)
	if int(idx) >= len(m.responses) {
		return nil, errors.New("no more mock responses")
	}
	resp := m.responses[idx]
	atomic.AddInt32(&m.responseIdx, 1)
	return resp.output, resp.err
}

func setupMockPVCKubectl(mock *mockPVCKubectl) func() {
	original := execKubectlWithContext
	execKubectlWithContext = mock.exec
	return func() {
		execKubectlWithContext = original
	}
}

func TestGetPVCClusterMembers(t *testing.T) {
	tests := []struct {
		name          string
		mockResponses []mockPVCResponse
		pvcNames      []string
		wantResult    map[string]string
		wantErr       bool
		wantErrContain string
	}{
		{
			name: "single_pvc_single_cluster",
			mockResponses: []mockPVCResponse{
				{output: []byte("gy-001 wlcb-001")},
				{output: []byte("pvc-data")},
				{err: errors.New("not found")},
			},
			pvcNames:   []string{"test-pvc"},
			wantResult: map[string]string{"test-pvc": "gy-001"},
			wantErr:    false,
		},
		{
			name: "single_pvc_multiple_clusters_empty_result",
			mockResponses: []mockPVCResponse{
				{output: []byte("gy-001 wlcb-001")},
				{output: []byte("pvc-data")},
				{output: []byte("pvc-data")},
			},
			pvcNames:   []string{"test-pvc"},
			wantResult: map[string]string{"test-pvc": ""},
			wantErr:    false,
		},
		{
			name: "two_pvcs_same_cluster",
			mockResponses: []mockPVCResponse{
				{output: []byte("gy-001 wlcb-001")},
				{output: []byte("pvc1-data")},
				{err: errors.New("not found")},
				{output: []byte("pvc2-data")},
				{err: errors.New("not found")},
			},
			pvcNames:   []string{"pvc1", "pvc2"},
			wantResult: map[string]string{"pvc1": "gy-001", "pvc2": "gy-001"},
			wantErr:    false,
		},
		{
			name: "two_pvcs_different_clusters_error",
			mockResponses: []mockPVCResponse{
				{output: []byte("gy-001 wlcb-001")},
				{output: []byte("pvc1-data")},
				{err: errors.New("not found")},
				{err: errors.New("not found")},
				{output: []byte("pvc2-data")},
			},
			pvcNames:      []string{"pvc1", "pvc2"},
			wantErr:       true,
			wantErrContain: "different clusters",
		},
		{
			name: "pvc_not_found_in_any_cluster",
			mockResponses: []mockPVCResponse{
				{output: []byte("gy-001 wlcb-001")},
				{err: errors.New("not found")},
				{err: errors.New("not found")},
			},
			pvcNames:      []string{"missing-pvc"},
			wantErr:       true,
			wantErrContain: "please contact manager",
		},
		{
			name: "get_clusters_error",
			mockResponses: []mockPVCResponse{
				{err: errors.New("cluster api error")},
			},
			pvcNames: []string{"test-pvc"},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockPVCKubectl{responses: tt.mockResponses}
			cleanup := setupMockPVCKubectl(mock)
			defer cleanup()

			cfg := Config{Namespace: "argo"}

			result, err := getPVCClusterMembers(cfg, tt.pvcNames)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrContain != "" && !strings.Contains(err.Error(), tt.wantErrContain) {
					t.Fatalf("expected error containing '%s', got: %v", tt.wantErrContain, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				if len(result) != len(tt.wantResult) {
					t.Fatalf("expected %d results, got %d: %v", len(tt.wantResult), len(result), result)
				}
				for k, v := range tt.wantResult {
					if result[k] != v {
						t.Fatalf("expected result[%s]='%s', got '%s'", k, v, result[k])
					}
				}
			}

			t.Logf("test '%s': result=%v, err=%v", tt.name, result, err)
		})
	}
}

func TestGetNodeChipClusterMember(t *testing.T) {
	tests := []struct {
		name           string
		mockResponses  []mockPVCResponse
		chipName       string
		wantCluster    string
		wantErr        bool
		wantErrContain string
	}{
		{
			name: "single_cluster_has_nodes",
			mockResponses: []mockPVCResponse{
				{output: []byte("member1 member2")},
				{output: []byte("{\"items\":[{\"metadata\":{\"name\":\"node1\"}}]}")},
				{output: []byte("{\"items\":[]}")},
			},
			chipName:    "910B4",
			wantCluster: "member1",
			wantErr:     false,
		},
		{
			name: "multiple_clusters_have_nodes_empty_result",
			mockResponses: []mockPVCResponse{
				{output: []byte("member1 member2")},
				{output: []byte("{\"items\":[{\"metadata\":{\"name\":\"node1\"}}]}")},
				{output: []byte("{\"items\":[{\"metadata\":{\"name\":\"node2\"}}]}")},
			},
			chipName:    "910B4",
			wantCluster: "",
			wantErr:     false,
		},
		{
			name: "no_cluster_has_nodes_error",
			mockResponses: []mockPVCResponse{
				{output: []byte("member1 member2")},
				{output: []byte("{\"items\":[]}")},
				{output: []byte("{\"items\":[]}")},
			},
			chipName:       "910B4",
			wantErr:        true,
			wantErrContain: "please contact manager",
		},
		{
			name: "get_clusters_error",
			mockResponses: []mockPVCResponse{
				{err: errors.New("cluster api error")},
			},
			chipName: "910B4",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockPVCKubectl{responses: tt.mockResponses}
			cleanup := setupMockPVCKubectl(mock)
			defer cleanup()

			cfg := Config{Namespace: "argo"}

			cluster, err := getNodeChipClusterMember(cfg, tt.chipName)

			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tt.wantErrContain != "" && !strings.Contains(err.Error(), tt.wantErrContain) {
					t.Fatalf("expected error containing '%s', got: %v", tt.wantErrContain, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				if cluster != tt.wantCluster {
					t.Fatalf("expected cluster '%s', got '%s'", tt.wantCluster, cluster)
				}
			}

			t.Logf("test '%s': cluster=%s, err=%v", tt.name, cluster, err)
		})
	}
}

func TestAddClusterDispatchLabels(t *testing.T) {
	tests := []struct {
		name           string
		mockResponses  []mockPVCResponse
		wantErr        bool
		wantErrContain string
	}{
		{
			name: "pvc_and_chip_same_cluster",
			mockResponses: []mockPVCResponse{
				{output: []byte("member1 member2")},
				{output: []byte("{\"items\":[{\"metadata\":{\"name\":\"test-pvc\"}}]}")},
				{output: []byte("{}")},
				{output: []byte("{\"items\":[{\"metadata\":{\"labels\":{\"node.kubernetes.io/npu.chip.name\":\"910B4\"}}}]}")},
				{output: []byte("{}")},
			},
			wantErr: false,
		},
		{
			name: "pvc_and_chip_different_cluster_conflict",
			mockResponses: []mockPVCResponse{
				{output: []byte("member1 member2")},
				{output: []byte("{\"items\":[{\"metadata\":{\"name\":\"test-pvc\"}}]}")},
				{output: []byte("{}")},
				{output: []byte("{}")},
				{output: []byte("{\"items\":[{\"metadata\":{\"labels\":{\"node.kubernetes.io/npu.chip.name\":\"910B4\"}}}]}")},
			},
			wantErr:        true,
			wantErrContain: "dispatch conflict",
		},
		{
			name: "pvc_only_single_cluster",
			mockResponses: []mockPVCResponse{
				{output: []byte("member1 member2")},
				{output: []byte("{\"items\":[{\"metadata\":{\"name\":\"test-pvc\"}}]}")},
				{output: []byte("{}")},
			},
			wantErr: false,
		},
		{
			name: "chip_only_single_cluster",
			mockResponses: []mockPVCResponse{
				{output: []byte("member1 member2")},
				{output: []byte("{\"items\":[{\"metadata\":{\"labels\":{\"node.kubernetes.io/npu.chip.name\":\"910B4\"}}}]}")},
				{output: []byte("{}")},
			},
			wantErr: false,
		},
		{
			name: "pvc_random_dispatch_multiple_clusters",
			mockResponses: []mockPVCResponse{
				{output: []byte("member1 member2")},
				{output: []byte("{\"items\":[{\"metadata\":{\"name\":\"test-pvc\"}}]}")},
				{output: []byte("{\"items\":[{\"metadata\":{\"name\":\"test-pvc\"}}]}")},
			},
			wantErr: false,
		},
		{
			name: "chip_random_dispatch_multiple_clusters",
			mockResponses: []mockPVCResponse{
				{output: []byte("member1 member2")},
				{output: []byte("{\"items\":[{\"metadata\":{\"labels\":{\"node.kubernetes.io/npu.chip.name\":\"910B4\"}}}]}")},
				{output: []byte("{\"items\":[{\"metadata\":{\"labels\":{\"node.kubernetes.io/npu.chip.name\":\"910B4\"}}}]}")},
			},
			wantErr: false,
		},
		{
			name: "get_clusters_error",
			mockResponses: []mockPVCResponse{
				{err: errors.New("cluster api error")},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockPVCKubectl{responses: tt.mockResponses}
			cleanup := setupMockPVCKubectl(mock)
			defer cleanup()

			t.Logf("test '%s': wantErr=%v, wantErrContain='%s'", tt.name, tt.wantErr, tt.wantErrContain)
		})
	}
}