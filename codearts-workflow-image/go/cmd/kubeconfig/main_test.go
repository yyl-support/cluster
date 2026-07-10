package main

import (
	"os"
	"path/filepath"
	"testing"
)

type mockNPUQuerier struct {
	results map[string][]kubeconfigEntry
}

func (m *mockNPUQuerier) QueryCluster(kubeconfigPath string) ([]kubeconfigEntry, error) {
	if m.results != nil {
		if entries, ok := m.results[kubeconfigPath]; ok {
			return entries, nil
		}
	}
	return nil, nil
}

func TestParseCPRunsOn(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		expectedChip string
		expectedNum  int
	}{
		{"arm64 with 910b4", "arm64-910b4-1", "910B4", 1},
		{"arm64 with 910B4 uppercase", "arm64-910B4-1", "910B4", 1},
		{"arm64 with 910b3", "arm64-910b3-2", "910B3", 2},
		{"arm64 with npu only", "arm64-npu-1", "npu", 1},
		{"amd64", "amd64", "", 0},
		{"empty string", "", "", 0},
		{"arm64 with 910a", "arm64-910a-1", "910A", 1},
		{"arm64 with 310p3", "arm64-310p3-1", "310P3", 1},
		{"unknown chip", "arm64-unknownchip-1", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chip, num := parseCPRunsOn(tt.input)
			if chip != tt.expectedChip || num != tt.expectedNum {
				t.Errorf("parseCPRunsOn(%q) = (%q, %d), want (%q, %d)", tt.input, chip, num, tt.expectedChip, tt.expectedNum)
			}
		})
	}
}

func TestCollectKubeconfigEntries(t *testing.T) {
	tmpDir := t.TempDir()

	kubeconfigKey := filepath.Join(tmpDir, "kubeconfig.key")
	if err := os.WriteFile(kubeconfigKey, []byte("dGVzdA=="), 0600); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	kubeconfig910b4 := filepath.Join(tmpDir, "kubeconfig_910b4.key")
	if err := os.WriteFile(kubeconfig910b4, []byte("dGVzdA=="), 0600); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	kubeconfig310p3 := filepath.Join(tmpDir, "kubeconfig_310p3.key")
	if err := os.WriteFile(kubeconfig310p3, []byte("dGVzdA=="), 0600); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	nonKubeconfig := filepath.Join(tmpDir, "other.key")
	if err := os.WriteFile(nonKubeconfig, []byte("dGVzdA=="), 0600); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	mock := &mockNPUQuerier{
		results: map[string][]kubeconfigEntry{
			kubeconfigKey: {
				{
					filename: kubeconfigKey,
					nodeinfo: []info{
						{chipName: "910B4", availableNPU: 4, allocatableNPU: 8},
					},
				},
			},
			kubeconfig910b4: {
				{
					filename: kubeconfig910b4,
					nodeinfo: []info{
						{chipName: "910B4", availableNPU: 8, allocatableNPU: 16},
					},
				},
			},
			kubeconfig310p3: {
				{
					filename: kubeconfig310p3,
					nodeinfo: []info{
						{chipName: "310P3", availableNPU: 2, allocatableNPU: 4},
					},
				},
			},
		},
	}

	origQuerier := defaultNPUQuerier
	setNPUQuerier(mock)
	defer func() { setNPUQuerier(origQuerier) }()

	entries, err := collectKubeconfigEntries(tmpDir)
	if err != nil {
		t.Fatalf("collectKubeconfigEntries 失败: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("期望 3 个条目，实际 %d", len(entries))
	}
}

func TestSelectClusterImmediateAllocation(t *testing.T) {
	entries := []kubeconfigEntry{
		{
			filename: "/path/kubeconfig_910b3.key",
			nodeinfo: []info{
				{chipName: "910B3", availableNPU: 4, allocatableNPU: 8},
			},
		},
		{
			filename: "/path/kubeconfig_910b4.key",
			nodeinfo: []info{
				{chipName: "910B4", availableNPU: 8, allocatableNPU: 16},
			},
		},
	}

	result := SelectCluster(1, "910B4", entries)
	if result == nil {
		t.Fatal("SelectCluster 返回 nil")
	}
	if result.filename != "/path/kubeconfig_910b4.key" {
		t.Errorf("期望 /path/kubeconfig_910b4.key，实际 %s", result.filename)
	}
}

func TestSelectClusterNoChipMatch(t *testing.T) {
	entries := []kubeconfigEntry{
		{
			filename: "/path/kubeconfig_910b3.key",
			nodeinfo: []info{
				{chipName: "910B3", availableNPU: 4, allocatableNPU: 8},
			},
		},
		{
			filename: "/path/kubeconfig_310p3.key",
			nodeinfo: []info{
				{chipName: "310P3", availableNPU: 2, allocatableNPU: 4},
			},
		},
	}

	result := SelectCluster(1, "910B4", entries)
	if result != nil {
		t.Errorf("期望 nil（无匹配），实际 %s", result.filename)
	}
}

func TestSelectClusterNoChipPrefersMaxAllocatable(t *testing.T) {
	entries := []kubeconfigEntry{
		{
			filename: "/path/kubeconfig_910b3.key",
			nodeinfo: []info{
				{chipName: "910B3", availableNPU: 4, allocatableNPU: 8},
			},
		},
		{
			filename: "/path/kubeconfig_910b4.key",
			nodeinfo: []info{
				{chipName: "910B4", availableNPU: 8, allocatableNPU: 1000},
			},
		},
		{
			filename: "/path/kubeconfig_310p3.key",
			nodeinfo: []info{
				{chipName: "310P3", availableNPU: 2, allocatableNPU: 4},
			},
		},
	}

	result := SelectCluster(100, "", entries)
	if result == nil {
		t.Fatal("SelectCluster 返回 nil")
	}
	if result.filename != "/path/kubeconfig_910b4.key" {
		t.Errorf("期望 /path/kubeconfig_910b4.key (max allocatable)，实际 %s", result.filename)
	}
}

func TestSelectClusterWithChipMatchPrefersMaxAvailable(t *testing.T) {
	entries := []kubeconfigEntry{
		{
			filename: "/path/kubeconfig_910b4.key",
			nodeinfo: []info{
				{chipName: "910B4", availableNPU: 4, allocatableNPU: 8},
			},
		},
		{
			filename: "/path/kubeconfig_910b4-2.key",
			nodeinfo: []info{
				{chipName: "910B4", availableNPU: 8, allocatableNPU: 16},
			},
		},
	}

	result := SelectCluster(1, "910B4", entries)
	if result == nil {
		t.Fatal("SelectCluster 返回 nil")
	}
	if result.filename != "/path/kubeconfig_910b4.key" {
		t.Errorf("期望 /path/kubeconfig_910b4.key (first immediate match)，实际 %s", result.filename)
	}
}

func TestSelectClusterWeightedRandomWhenNoImmediateMatch(t *testing.T) {
	entries := []kubeconfigEntry{
		{
			filename: "/path/kubeconfig_910b4.key",
			nodeinfo: []info{
				{chipName: "910B4", availableNPU: 2, allocatableNPU: 8},
			},
		},
		{
			filename: "/path/kubeconfig_910b4-2.key",
			nodeinfo: []info{
				{chipName: "910B4", availableNPU: 2, allocatableNPU: 1000},
			},
		},
	}

	result := SelectCluster(4, "910B4", entries)
	if result == nil {
		t.Fatal("SelectCluster 返回 nil")
	}
	if result.filename != "/path/kubeconfig_910b4-2.key" {
		t.Errorf("期望 /path/kubeconfig_910b4-2.key (max allocatable when no immediate match)，实际 %s", result.filename)
	}
}

func TestSelectClusterNoAvailableCluster(t *testing.T) {
	entries := []kubeconfigEntry{}

	result := SelectCluster(1, "", entries)
	if result != nil {
		t.Errorf("期望 nil，实际 %s", result.filename)
	}
}

func TestSelectClusterInsufficientButWeightedRandom(t *testing.T) {
	entries := []kubeconfigEntry{
		{
			filename: "/path/kubeconfig_910b3.key",
			nodeinfo: []info{
				{chipName: "910B3", availableNPU: 2, allocatableNPU: 4},
			},
		},
		{
			filename: "/path/kubeconfig_910b4.key",
			nodeinfo: []info{
				{chipName: "910B4", availableNPU: 2, allocatableNPU: 1000},
			},
		},
	}

	result := SelectCluster(4, "910B4", entries)
	if result == nil {
		t.Fatal("SelectCluster 返回 nil（应该用加权随机选择）")
	}
	if result.filename != "/path/kubeconfig_910b4.key" {
		t.Errorf("期望 /path/kubeconfig_910b4.key (max allocatable)，实际 %s", result.filename)
	}
}

func TestUniqueStrings(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected []string
	}{
		{"no duplicates", []string{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"with duplicates", []string{"a", "b", "a", "c", "b"}, []string{"a", "b", "c"}},
		{"empty strings", []string{"a", "", "b", ""}, []string{"a", "b"}},
		{"single element", []string{"a"}, []string{"a"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := uniqueStrings(tt.input)
			if len(result) != len(tt.expected) {
				t.Errorf("uniqueStrings(%v) = %v, want %v", tt.input, result, tt.expected)
				return
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("uniqueStrings(%v)[%d] = %q, want %q", tt.input, i, v, tt.expected[i])
				}
			}
		})
	}
}

func TestMockNPUQuerier(t *testing.T) {
	mock := &mockNPUQuerier{
		results: map[string][]kubeconfigEntry{
			"/path/kubeconfig.key": {
				{
					filename: "/path/kubeconfig.key",
					nodeinfo: []info{
						{chipName: "910B4", availableNPU: 4, allocatableNPU: 8},
						{chipName: "310P3", availableNPU: 2, allocatableNPU: 4},
					},
				},
			},
		},
	}

	entries, err := mock.QueryCluster("/path/kubeconfig.key")
	if err != nil {
		t.Fatalf("QueryCluster 失败: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("期望 1 个条目，实际 %d", len(entries))
	}

	if len(entries[0].nodeinfo) != 2 {
		t.Errorf("期望 2 个 nodeinfo，实际 %d", len(entries[0].nodeinfo))
	}

	if entries[0].nodeinfo[0].chipName != "910B4" || entries[0].nodeinfo[0].availableNPU != 4 {
		t.Errorf("unexpected entry: %+v", entries[0].nodeinfo[0])
	}
}

func TestCollectKubeconfigEntriesWithMock(t *testing.T) {
	tmpDir := t.TempDir()

	kubeconfig910b4 := filepath.Join(tmpDir, "kubeconfig_910b4.key")
	if err := os.WriteFile(kubeconfig910b4, []byte("dGVzdA=="), 0600); err != nil {
		t.Fatalf("创建测试文件失败: %v", err)
	}

	mock := &mockNPUQuerier{
		results: map[string][]kubeconfigEntry{
			kubeconfig910b4: {
				{
					filename: kubeconfig910b4,
					nodeinfo: []info{
						{chipName: "910B4", availableNPU: 8, allocatableNPU: 16},
						{chipName: "310P3", availableNPU: 2, allocatableNPU: 4},
					},
				},
			},
		},
	}

	origQuerier := defaultNPUQuerier
	setNPUQuerier(mock)
	defer func() { setNPUQuerier(origQuerier) }()

	entries, err := collectKubeconfigEntries(tmpDir)
	if err != nil {
		t.Fatalf("collectKubeconfigEntries 失败: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("期望 1 个条目（同一 kubeconfig），实际 %d", len(entries))
	}

	if len(entries[0].nodeinfo) != 2 {
		t.Errorf("期望 2 个 nodeinfo，实际 %d", len(entries[0].nodeinfo))
	}

	found910B4 := false
	found310P3 := false
	for _, node := range entries[0].nodeinfo {
		if node.chipName == "910B4" && node.availableNPU == 8 {
			found910B4 = true
		}
		if node.chipName == "310P3" && node.availableNPU == 2 {
			found310P3 = true
		}
	}

	if !found910B4 {
		t.Error("未找到 910B4 条目")
	}
	if !found310P3 {
		t.Error("未找到 310P3 条目")
	}
}

func TestAggregateByChip(t *testing.T) {
	infos := []info{
		{chipName: "910B4", availableNPU: 4, allocatableNPU: 8},
		{chipName: "910B4", availableNPU: 2, allocatableNPU: 4},
		{chipName: "310P3", availableNPU: 2, allocatableNPU: 4},
	}

	result := aggregateByChip(infos)

	if len(result) != 2 {
		t.Errorf("期望 2 个聚合结果，实际 %d", len(result))
	}

	chipMap := make(map[string]info)
	for _, r := range result {
		chipMap[r.chipName] = r
	}

	if chipMap["910B4"].availableNPU != 6 {
		t.Errorf("910B4 availableNPU 期望 6，实际 %d", chipMap["910B4"].availableNPU)
	}
	if chipMap["910B4"].allocatableNPU != 12 {
		t.Errorf("910B4 allocatableNPU 期望 12，实际 %d", chipMap["910B4"].allocatableNPU)
	}
	if chipMap["310P3"].availableNPU != 2 {
		t.Errorf("310P3 availableNPU 期望 2，实际 %d", chipMap["310P3"].availableNPU)
	}
}
