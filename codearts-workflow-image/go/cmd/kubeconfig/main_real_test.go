package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestExtractRealClusterData(t *testing.T) {
	homeDir := os.Getenv("HOME")
	kubeconfigPath := filepath.Join(homeDir, ".kube", "kubeconfig.key")

	t.Logf("Testing with kubeconfig at: %s", kubeconfigPath)

	if _, err := os.Stat(kubeconfigPath); err != nil {
		t.Skipf("kubeconfig not found at %s, skipping real cluster test", kubeconfigPath)
	}

	t.Logf("defaultNPUQuerier type: %T", defaultNPUQuerier)

	entries, err := defaultNPUQuerier.QueryCluster(kubeconfigPath)
	if err != nil {
		t.Fatalf("QueryCluster failed: %v", err)
	}

	fmt.Printf("\n=== Real Cluster Data ===\n")
	fmt.Printf("Kubeconfig: %s\n", kubeconfigPath)
	fmt.Printf("Number of entries: %d\n", len(entries))
	for i, e := range entries {
		for j, node := range e.nodeinfo {
			fmt.Printf("Entry %d node %d  chipName=%s, availableNPU=%d  allocatebla%d\n", i+1, j+1, node.chipName, node.availableNPU, node.allocatableNPU)
		}

	}

	if len(entries) == 0 {
		t.Log("No NPU entries found in cluster")
	}
}
