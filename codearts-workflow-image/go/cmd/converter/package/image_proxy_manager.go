package converter

import (
	"strings"
)

const (
	defaultImageProxyURL = "harbor-portal.osinfra.cn"
)

var registryProxyMap = map[string]string{
	"swr.cn-north-4.myhuaweicloud.com": "north4-myhuaweicloud",
}

type ImageProxyManager struct {
	proxyURL string
}

var defaultImageProxyManager = &ImageProxyManager{
	proxyURL: defaultImageProxyURL,
}

func NewImageProxyManager() *ImageProxyManager {
	return &ImageProxyManager{
		proxyURL: defaultImageProxyURL,
	}
}

func NewImageProxyManagerWithURL(proxyURL string) *ImageProxyManager {
	if proxyURL == "" {
		proxyURL = defaultImageProxyURL
	}
	return &ImageProxyManager{
		proxyURL: proxyURL,
	}
}

func (ipm *ImageProxyManager) SetProxyURL(proxyURL string) {
	ipm.proxyURL = proxyURL
}

func (ipm *ImageProxyManager) GetProxyURL() string {
	return ipm.proxyURL
}

func (ipm *ImageProxyManager) ApplyProxy(imageURL string) string {
	if imageURL == "" {
		return imageURL
	}

	if ipm.proxyURL == "" {
		return imageURL
	}

	parts := strings.SplitN(imageURL, "/", 2)
	if len(parts) < 2 {
		return imageURL
	}

	registry := parts[0]
	imagePath := parts[1]

	if registry == ipm.proxyURL {
		return imageURL
	}

	regionPath, shouldProxy := registryProxyMap[registry]
	if !shouldProxy {
		return imageURL
	}

	return ipm.proxyURL + "/" + regionPath + "/" + imagePath
}

func GetImageProxyURL() string {
	return defaultImageProxyManager.GetProxyURL()
}

func ApplyImageProxy(imageURL string) string {
	return defaultImageProxyManager.ApplyProxy(imageURL)
}

func SetDefaultImageProxyURL(proxyURL string) {
	if proxyURL == "" {
		proxyURL = defaultImageProxyURL
	}
	defaultImageProxyManager.SetProxyURL(proxyURL)
}
