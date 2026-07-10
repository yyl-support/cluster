package main

import (
	"fmt"
	"os"

	"github.com/opensourceways/codearts-workflow-image-go/cmd/common/namespace"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "Usage: ns <command> <argument>")
		fmt.Fprintln(os.Stderr, "Commands:")
		fmt.Fprintln(os.Stderr, "  --from-repo <repo_name>")
		fmt.Fprintln(os.Stderr, "  --from-workflow <workflow_file>")
		os.Exit(1)
	}

	command := os.Args[1]
	arg := os.Args[2]

	switch command {
	case "--from-repo":
		fmt.Println(namespace.GetNamespaceFromRepoName(arg))
	case "--from-workflow":
		ns, err := namespace.GetNamespaceFromWorkflow(arg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
			os.Exit(1)
		}
		fmt.Println(ns)
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", command)
		os.Exit(1)
	}
}