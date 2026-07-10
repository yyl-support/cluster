package main

import (
	"flag"
	"os"
	"testing"
)

func Test_getYamlPaths(t *testing.T) {
	tests := []struct {
		name         string
		flagT        string
		flagO        string
		wantTemplate string
		wantTarget   string
	}{
		{
			name:         "defaults",
			flagT:        "",
			flagO:        "",
			wantTemplate: "case/workflow_templatev2.yaml",
			wantTarget:   "./workflow_trans.yaml",
		},
		{
			name:         "with_output",
			flagT:        "",
			flagO:        "custom.yaml",
			wantTemplate: "case/workflow_templatev2.yaml",
			wantTarget:   "custom.yaml",
		},
		{
			name:         "with_template",
			flagT:        "./custom/template.yaml",
			flagO:        "",
			wantTemplate: "./custom/template.yaml",
			wantTarget:   "./workflow_trans.yaml",
		},
		{
			name:         "with_both",
			flagT:        "./custom/template.yaml",
			flagO:        "output.yaml",
			wantTemplate: "./custom/template.yaml",
			wantTarget:   "output.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

			templatePath = ""
			outputPath = ""

			flag.StringVar(&templatePath, "t", "case/workflow_templatev2.yaml", "")
			flag.StringVar(&outputPath, "o", "./workflow_trans.yaml", "")

			if tt.flagT != "" {
				templatePath = tt.flagT
			}
			if tt.flagO != "" {
				outputPath = tt.flagO
			}

			gotTemplate, gotTarget := getYamlPaths()

			if gotTemplate != tt.wantTemplate {
				t.Errorf("template = %v, want %v", gotTemplate, tt.wantTemplate)
			}
			if gotTarget != tt.wantTarget {
				t.Errorf("target = %v, want %v", gotTarget, tt.wantTarget)
			}
		})
	}
}

func Test_main_full_flow(t *testing.T) {
	tests := []struct {
		name         string
		args         []string
		wantTemplate string
		wantTarget   string
	}{
		{
			name:         "defaults",
			args:         []string{"cmd"},
			wantTemplate: "case/workflow_templatev2.yaml",
			wantTarget:   "./workflow_trans.yaml",
		},
		{
			name:         "with_output",
			args:         []string{"cmd", "-o", "custom.yaml"},
			wantTemplate: "case/workflow_templatev2.yaml",
			wantTarget:   "custom.yaml",
		},
		{
			name:         "with_template",
			args:         []string{"cmd", "-t", "./custom/template.yaml"},
			wantTemplate: "./custom/template.yaml",
			wantTarget:   "./workflow_trans.yaml",
		},
		{
			name:         "with_both",
			args:         []string{"cmd", "-o", "output.yaml", "-t", "template.yaml"},
			wantTemplate: "template.yaml",
			wantTarget:   "output.yaml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)

			templatePath = ""
			outputPath = ""

			flag.StringVar(&templatePath, "t", "case/workflow_templatev2.yaml", "")
			flag.StringVar(&outputPath, "o", "./workflow_trans.yaml", "")

			os.Args = tt.args
			flag.Parse()

			gotTemplate, gotTarget := getYamlPaths()

			if gotTemplate != tt.wantTemplate {
				t.Errorf("template = %v, want %v", gotTemplate, tt.wantTemplate)
			}
			if gotTarget != tt.wantTarget {
				t.Errorf("target = %v, want %v", gotTarget, tt.wantTarget)
			}
		})
	}
}
