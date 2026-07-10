package main

import (
	"flag"
	"fmt"
)

var (
	templatePath string
	outputPath   string
)

func init() {
	flag.StringVar(&templatePath, "t", "case/workflow_templatev2.yaml", "Template YAML path")
	flag.StringVar(&outputPath, "o", "./workflow_trans.yaml", "Output workflow YAML path")
}

func getYamlPaths() (templateYamlPath, targetYamlPath string) {
	templateYamlPath = "case/workflow_templatev2.yaml"
	targetYamlPath = "./workflow_trans.yaml"

	if templatePath != "" {
		templateYamlPath = templatePath
	}
	if outputPath != "" {
		targetYamlPath = outputPath
	}

	return
}

func main() {
	flag.Parse()
	template, target := getYamlPaths()
	fmt.Printf("template: %s, target: %s\n", template, target)
}
