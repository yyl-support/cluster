package converter

import (
	"testing"
)

func TestImageProxyManagerGetProxyURL(t *testing.T) {
	tests := []struct {
		name      string
		proxyURL  string
		wantProxy string
	}{
		{
			name:      "empty proxy uses default",
			proxyURL:  "",
			wantProxy: defaultImageProxyURL,
		},
		{
			name:      "custom proxy url",
			proxyURL:  "mirror.example.com",
			wantProxy: "mirror.example.com",
		},
		{
			name:      "default proxy url",
			proxyURL:  defaultImageProxyURL,
			wantProxy: defaultImageProxyURL,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ipm := NewImageProxyManagerWithURL(tt.proxyURL)
			got := ipm.GetProxyURL()
			if got != tt.wantProxy {
				t.Errorf("GetProxyURL() = %q, want %q", got, tt.wantProxy)
			}
		})
	}
}

func TestImageProxyManagerSetAndGet(t *testing.T) {
	ipm := NewImageProxyManager()
	ipm.SetProxyURL("custom.proxy.com")

	if got := ipm.GetProxyURL(); got != "custom.proxy.com" {
		t.Errorf("GetProxyURL() = %q, want custom.proxy.com", got)
	}
}

func TestImageProxyManagerApplyProxy(t *testing.T) {
	tests := []struct {
		name      string
		proxyURL  string
		imageURL  string
		wantImage string
	}{
		{
			name:      "proxy URL matches original north-4 SWR registry - no change",
			proxyURL:  "swr.cn-north-4.myhuaweicloud.com",
			imageURL:  "swr.cn-north-4.myhuaweicloud.com/base_image/test:latest",
			wantImage: "swr.cn-north-4.myhuaweicloud.com/base_image/test:latest",
		},
		{
			name:      "swr north-4 registry with region path",
			proxyURL:  defaultImageProxyURL,
			imageURL:  "swr.cn-north-4.myhuaweicloud.com/base_image/test:latest",
			wantImage: "harbor-portal.osinfra.cn/north4-myhuaweicloud/base_image/test:latest",
		},
		{
			name:      "swr southwest-2 registry not in map - no change",
			proxyURL:  defaultImageProxyURL,
			imageURL:  "swr.cn-southwest-2.myhuaweicloud.com/base_image/ascend-ci/cann:8.2.rc1-910b-ubuntu22.04-py3.11",
			wantImage: "swr.cn-southwest-2.myhuaweicloud.com/base_image/ascend-ci/cann:8.2.rc1-910b-ubuntu22.04-py3.11",
		},
		{
			name:      "swr ap-southeast-1 registry not in map - no change",
			proxyURL:  defaultImageProxyURL,
			imageURL:  "swr.ap-southeast-1.myhuaweicloud.com/base_image/test:v1",
			wantImage: "swr.ap-southeast-1.myhuaweicloud.com/base_image/test:v1",
		},
		{
			name:      "docker.io registry not in map - no change",
			proxyURL:  defaultImageProxyURL,
			imageURL:  "docker.io/library/ubuntu:22.04",
			wantImage: "docker.io/library/ubuntu:22.04",
		},
		{
			name:      "gcr.io registry not in map - no change",
			proxyURL:  defaultImageProxyURL,
			imageURL:  "gcr.io/google-containers/pause:3.1",
			wantImage: "gcr.io/google-containers/pause:3.1",
		},
		{
			name:      "localhost registry not in map - no change",
			proxyURL:  defaultImageProxyURL,
			imageURL:  "localhost:5000/my-image:v1",
			wantImage: "localhost:5000/my-image:v1",
		},
		{
			name:      "custom proxy for north-4 registry",
			proxyURL:  "mirror.internal.com",
			imageURL:  "swr.cn-north-4.myhuaweicloud.com/base_image/test:latest",
			wantImage: "mirror.internal.com/north4-myhuaweicloud/base_image/test:latest",
		},
		{
			name:      "custom proxy for unmapped registry - no change",
			proxyURL:  "mirror.internal.com",
			imageURL:  "swr.cn-southwest-2.myhuaweicloud.com/base_image/test:latest",
			wantImage: "swr.cn-southwest-2.myhuaweicloud.com/base_image/test:latest",
		},
		{
			name:      "short image name without registry unchanged",
			proxyURL:  defaultImageProxyURL,
			imageURL:  "my-custom-image:latest",
			wantImage: "my-custom-image:latest",
		},
		{
			name:      "library image without registry unchanged",
			proxyURL:  defaultImageProxyURL,
			imageURL:  "nginx:latest",
			wantImage: "nginx:latest",
		},
		{
			name:      "empty proxy returns original",
			proxyURL:  "none",
			imageURL:  "swr.cn-north-4.myhuaweicloud.com/base_image/test:latest",
			wantImage: "swr.cn-north-4.myhuaweicloud.com/base_image/test:latest",
		},
		{
			name:      "non-swr registry not in map - no change",
			proxyURL:  defaultImageProxyURL,
			imageURL:  "harbor.example.com/base_image/test:v1",
			wantImage: "harbor.example.com/base_image/test:v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ipm := NewImageProxyManagerWithURL(tt.proxyURL)
			if tt.proxyURL == "none" {
				ipm.SetProxyURL("")
			}
			got := ipm.ApplyProxy(tt.imageURL)
			if got != tt.wantImage {
				t.Errorf("ApplyProxy(%q) = %q, want %q", tt.imageURL, got, tt.wantImage)
			}
		})
	}
}

func TestGetImageProxyURL(t *testing.T) {
	ipm := NewImageProxyManagerWithURL("custom-proxy.com")
	oldDefault := defaultImageProxyManager
	defaultImageProxyManager = ipm
	defer func() { defaultImageProxyManager = oldDefault }()

	if got := GetImageProxyURL(); got != "custom-proxy.com" {
		t.Errorf("GetImageProxyURL() = %q, want custom-proxy.com", got)
	}
}

func TestApplyImageProxy(t *testing.T) {
	ipm := NewImageProxyManagerWithURL(defaultImageProxyURL)
	oldDefault := defaultImageProxyManager
	defaultImageProxyManager = ipm
	defer func() { defaultImageProxyManager = oldDefault }()

	tests := []struct {
		imageURL  string
		wantImage string
	}{
		{
			imageURL:  "swr.cn-north-4.myhuaweicloud.com/base_image/test:v1",
			wantImage: "harbor-portal.osinfra.cn/north4-myhuaweicloud/base_image/test:v1",
		},
		{
			imageURL:  "swr.cn-southwest-2.myhuaweicloud.com/base_image/test:v1",
			wantImage: "swr.cn-southwest-2.myhuaweicloud.com/base_image/test:v1",
		},
		{
			imageURL:  "docker.io/library/nginx:latest",
			wantImage: "docker.io/library/nginx:latest",
		},
	}

	for _, tt := range tests {
		got := ApplyImageProxy(tt.imageURL)
		if got != tt.wantImage {
			t.Errorf("ApplyImageProxy(%q) = %q, want %q", tt.imageURL, got, tt.wantImage)
		}
	}
}

func TestSetDefaultImageProxyURL(t *testing.T) {
	original := defaultImageProxyManager.proxyURL
	defer func() { defaultImageProxyManager.proxyURL = original }()

	SetDefaultImageProxyURL("new-proxy.com")
	if got := GetImageProxyURL(); got != "new-proxy.com" {
		t.Errorf("GetImageProxyURL() = %q, want new-proxy.com", got)
	}

	SetDefaultImageProxyURL("")
	if got := GetImageProxyURL(); got != defaultImageProxyURL {
		t.Errorf("GetImageProxyURL() = %q, want %q", got, defaultImageProxyURL)
	}
}
