package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"

	runonparser "github.com/opensourceways/codearts-workflow-image-go/cmd/common"
	"github.com/opensourceways/codearts-workflow-image-go/cmd/kubeconfig/kubectl"
)

type NPUQuerier interface {
	QueryCluster(kubeconfigPath string) ([]kubeconfigEntry, error)
}

type realNPUQuerier struct{}

type info struct {
	chipName       string
	availableNPU   int
	allocatableNPU int
}

type kubeconfigEntry struct {
	filename    string // 原始文件路径，可用来标识集群
	nodeinfo    []info
	chipSummary []info // 按芯片聚合的 summary，此处未使用但可填充
}

// NPU 资源名，根据实际集群配置调整
const npuResourceName = "huawei.com/ascend-1980"

func (q *realNPUQuerier) QueryCluster(kubeconfigPath string) ([]kubeconfigEntry, error) {
	// 1. 读取并解码 base64 内容
	content, err := os.ReadFile(kubeconfigPath)
	if err != nil {
		return nil, fmt.Errorf("读取 kubeconfig 失败: %w", err)
	}

	decoded := make([]byte, base64.StdEncoding.DecodedLen(len(content)))
	n, err := base64.StdEncoding.Decode(decoded, content)
	if err != nil {
		return nil, fmt.Errorf("base64 解码失败: %w", err)
	}

	// 2. 写入临时文件
	tmpFile, err := os.CreateTemp("", "kubeconfig-*.yaml")
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(decoded[:n]); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("写入临时文件失败: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("关闭临时文件失败: %w", err)
	}

	// 3. 获取节点信息（JSON 格式）
	nodeJSON, err := execKubectl(tmpPath, "get", "nodes", "-o", "json")
	if err != nil {
		return nil, err
	}

	// 4. 获取所有 Pod 信息（JSON 格式）
	podJSON, err := execKubectl(tmpPath, "get", "pods", "--all-namespaces", "-o", "json")
	if err != nil {
		return nil, err
	}

	// 5. 解析节点信息
	var nodeList struct {
		Items []struct {
			Metadata struct {
				Name   string            `json:"name"`
				Labels map[string]string `json:"labels"`
			} `json:"metadata"`
			Status struct {
				Allocatable map[string]string `json:"allocatable"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(nodeJSON, &nodeList); err != nil {
		return nil, fmt.Errorf("解析节点 JSON 失败: %w", err)
	}

	// 6. 解析 Pod 信息，计算每个节点已使用的 NPU 卡数
	var podList struct {
		Items []struct {
			Spec struct {
				NodeName   string `json:"nodeName"`
				Containers []struct {
					Resources struct {
						Requests map[string]string `json:"requests"`
					} `json:"resources"`
				} `json:"containers"`
			} `json:"spec"`
			Status struct {
				Phase string `json:"phase"`
			} `json:"status"`
		} `json:"items"`
	}
	if err := json.Unmarshal(podJSON, &podList); err != nil {
		return nil, fmt.Errorf("解析 Pod JSON 失败: %w", err)
	}

	// 计算每个节点的已用量
	nodeUsed := make(map[string]int)
	for _, pod := range podList.Items {
		if pod.Status.Phase != "Running" {
			continue // 只统计运行中的 Pod
		}
		nodeName := pod.Spec.NodeName
		if nodeName == "" {
			continue
		}
		for _, c := range pod.Spec.Containers {
			if val, ok := c.Resources.Requests[npuResourceName]; ok {
				// 解析数值（字符串可能为 "1" 或 "1k" 等，但 NPU 通常是整数）
				var used int
				fmt.Sscanf(val, "%d", &used)
				nodeUsed[nodeName] += used
			}
		}
	}

	// 7. 构建每个节点的 info 列表
	var nodeInfos []info
	for _, node := range nodeList.Items {
		// 获取芯片型号
		chipName := node.Metadata.Labels["node.kubernetes.io/npu.chip.name"]
		if chipName == "" {
			continue // 忽略没有 NPU 标签的节点
		}

		// 获取 allocatable NPU
		allocStr, ok := node.Status.Allocatable[npuResourceName]
		if !ok {
			continue // 节点没有该资源
		}
		var alloc int
		if _, err := fmt.Sscanf(allocStr, "%d", &alloc); err != nil || alloc <= 0 {
			continue // 无法解析或值为非正数，跳过
		}

		used := nodeUsed[node.Metadata.Name]
		if used < 0 {
			used = 0
		}
		available := alloc - used
		if available < 0 {
			available = 0
		}

		nodeInfos = append(nodeInfos, info{
			chipName:       chipName,
			availableNPU:   available,
			allocatableNPU: alloc,
		})
	}

	// 8. 构建 kubeconfigEntry（简单返回一个包含该集群所有节点的 entry）
	entry := kubeconfigEntry{
		filename:    kubeconfigPath,
		nodeinfo:    nodeInfos,
		chipSummary: aggregateByChip(nodeInfos), // 可选聚合
	}

	return []kubeconfigEntry{entry}, nil
}

// 执行 kubectl 命令并返回标准输出
func execKubectl(kubeconfig string, args ...string) ([]byte, error) {
	executor := &kubectl.RealExecutor{Kubeconfig: kubeconfig}
	return kubectl.ExecWithRetry(context.Background(), executor, args, kubectl.DefaultRetryConfig())
}

// 按芯片型号聚合统计（可选，用于 chipSummary）
func aggregateByChip(infos []info) []info {
	summary := make(map[string]*info)
	for _, i := range infos {
		if _, ok := summary[i.chipName]; !ok {
			summary[i.chipName] = &info{chipName: i.chipName}
		}
		s := summary[i.chipName]
		s.allocatableNPU += i.allocatableNPU
		s.availableNPU += i.availableNPU
	}
	result := make([]info, 0, len(summary))
	for _, v := range summary {
		result = append(result, *v)
	}
	return result
}

func uniqueStrings(input []string) []string {
	seen := make(map[string]bool)
	result := []string{}
	for _, s := range input {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

var defaultNPUQuerier NPUQuerier = &realNPUQuerier{}

func setNPUQuerier(q NPUQuerier) {
	defaultNPUQuerier = q
}

func main() {
	workspace := flag.String("w", os.Getenv("WORKSPACE"), "WORKSPACE directory containing kubeconfig files")
	cpRunsOn := flag.String("c", os.Getenv("CP_runs_on"), "CP_runs_on environment value")
	flag.Parse()

	if *workspace == "" {
		fmt.Fprintln(os.Stderr, "错误：WORKSPACE 环境变量未设置")
		os.Exit(1)
	}

	if err := os.Chdir(*workspace); err != nil {
		fmt.Fprintf(os.Stderr, "错误：无法切换到 WORKSPACE 目录: %v\n", err)
		os.Exit(1)
	}

	targetChip, count := parseCPRunsOn(*cpRunsOn)

	entries, err := collectKubeconfigEntries(*workspace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误：%v\n", err)
		os.Exit(1)
	}

	if len(entries) == 0 {
		fmt.Fprintln(os.Stderr, "错误：未找到任何可用的 kubeconfig")
		os.Exit(1)
	}

	result := SelectCluster(count, targetChip, entries)
	if result == nil {
		fmt.Fprintf(os.Stderr, "错误：未找到任何可用的cluster")
		os.Exit(1)
	}

	fmt.Print(result.filename)
}

func parseCPRunsOn(s string) (string, int) {
	if s == "" {
		return "", 0
	}

	parsedSpec, err := runonparser.Parse(s)
	if err != nil {
		return "", 0
	}

	return parsedSpec.NPUChipName, parsedSpec.NPUCount
}

func collectKubeconfigEntries(workspace string) ([]kubeconfigEntry, error) {
	var entries []kubeconfigEntry

	kubeconfigKey := filepath.Join(workspace, "kubeconfig.key")
	if _, err := os.Stat(kubeconfigKey); err == nil {
		result, err := defaultNPUQuerier.QueryCluster(kubeconfigKey)
		if err != nil {
			fmt.Fprintf(os.Stderr, "警告：查询 %s 的集群信息失败: %v\n", kubeconfigKey, err)
		} else {
			entries = append(entries, result...)
		}
	}

	pattern := filepath.Join(workspace, "kubeconfig_*.key")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return nil, fmt.Errorf("查找 kubeconfig 文件失败: %w", err)
	}

	sort.Strings(matches)

	for _, match := range matches {
		result, err := defaultNPUQuerier.QueryCluster(match)
		if err != nil {
			fmt.Fprintf(os.Stderr, "警告：查询 %s 的集群信息失败: %v\n", match, err)
			continue
		}
		entries = append(entries, result...)
	}

	return entries, nil
}

func SelectCluster(need int, chipFilter string, clusters []kubeconfigEntry) *kubeconfigEntry {

	if len(clusters) == 1 {
		return &clusters[0]
	}

	// 1. 筛选可立即分配的集群
	var immediateClusters []*kubeconfigEntry
	for i := range clusters {
		c := &clusters[i]
		for _, node := range c.nodeinfo {
			if (chipFilter == "npu" || node.chipName == chipFilter) && node.availableNPU >= need {
				immediateClusters = append(immediateClusters, c)
				break // 只要有一个节点满足即可
			}
		}
	}

	if len(immediateClusters) > 0 {
		// 立即分配：选择第一个满足的（也可改为选择 available 最大的节点所在集群）
		return immediateClusters[0]
	}

	// 2. 无法立即分配：按最大 allocatableNPU 加权随机
	type weighted struct {
		cluster *kubeconfigEntry
		weight  int
	}
	var list []weighted

	for i := range clusters {
		c := &clusters[i]
		maxAlloc := 0
		for _, node := range c.nodeinfo {
			if chipFilter == "" || node.chipName == chipFilter {
				if node.allocatableNPU > maxAlloc {
					maxAlloc = node.allocatableNPU
				}
			}
		}
		if maxAlloc > 0 {
			list = append(list, weighted{cluster: c, weight: maxAlloc})
		}
	}

	if len(list) == 0 {
		return nil // 无可用集群
	}

	// 按权重随机选择一个集群
	total := 0
	for _, w := range list {
		total += w.weight
	}
	r := rand.Intn(total)
	for _, w := range list {
		if r < w.weight {
			return w.cluster
		}
		r -= w.weight
	}
	return nil
}
