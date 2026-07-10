package main

import (
	"os"
	"testing"
)

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid",
			cfg: Config{
				WorkDir:        "/workspace",
				WorkflowOutput: "workflow.yaml",
				KubeconfigPath: "/workspace/workflowtool/k8s-cluster-kubeconfig.yaml",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.cfg)
			if tt.wantErr && err == nil {
				t.Fatal("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoadConfigImageProxyPriority(t *testing.T) {
	originalEnv := os.Getenv("CP_image_proxy")
	defer os.Setenv("CP_image_proxy", originalEnv)

	tests := []struct {
		name           string
		envValue       string
		args           []string
		expectedProxy  string
	}{
		{
			name:          "default when no env and no flag",
			envValue:      "",
			args:          []string{},
			expectedProxy: defaultImageProxyURL,
		},
		{
			name:          "env var overrides default",
			envValue:      "harbor-portal.test.osinfra.cn",
			args:          []string{},
			expectedProxy: "harbor-portal.test.osinfra.cn",
		},
		{
			name:          "flag overrides env var",
			envValue:      "harbor-portal.test.osinfra.cn",
			args:          []string{"--image-proxy-url", "custom.harbor.com"},
			expectedProxy: "custom.harbor.com",
		},
		{
			name:          "flag overrides default",
			envValue:      "",
			args:          []string{"--image-proxy-url", "custom.harbor.com"},
			expectedProxy: "custom.harbor.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Setenv("CP_image_proxy", tt.envValue)
			
			cfg, err := loadConfig(tt.args)
			if err != nil {
				t.Fatalf("loadConfig failed: %v", err)
			}

			if cfg.ImageProxyURL != tt.expectedProxy {
				t.Errorf("ImageProxyURL = %s, want %s", cfg.ImageProxyURL, tt.expectedProxy)
			}
		})
	}
}
