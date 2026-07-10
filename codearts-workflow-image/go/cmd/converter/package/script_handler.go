package converter

import (
	"context"
	"fmt"
	"net/url"
	"os/exec"
	"strings"
	"time"
)

func handlerScript(scriptContent string, req gitRequest, artifactsReq artifactsRequest, delayExitReq delayExitRequest) string {
	var sb strings.Builder

	sb.WriteString(delayExitReq.GenerateDelayExitTrapScript())
	sb.WriteString(req.GenerateGitCloneScript())
	sb.WriteString(scriptContent)

	artifactsScript := artifactsReq.GenerateArtifactsCopyScript()
	if artifactsScript != "" {
		if !strings.HasSuffix(scriptContent, "\n") {
			sb.WriteString("\n")
		}
		sb.WriteString(artifactsScript)
	}

	return sb.String()
}

func isURLReachable(repoURL string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	safeURL, err := validateGitURL(repoURL)
	if err != nil {
		return false
	}

	cmd := exec.CommandContext(ctx, "git", "ls-remote", "--heads", "--", safeURL) // #nosec G702 G204
	err = cmd.Run()
	return err == nil
}

func validateGitURL(rawURL string) (string, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL: %w", err)
	}

	allowedSchemes := map[string]bool{"http": true, "https": true, "git": true}
	if !allowedSchemes[u.Scheme] {
		return "", fmt.Errorf("unsupported protocol: %s", u.Scheme)
	}

	if strings.HasPrefix(rawURL, "-") {
		return "", fmt.Errorf("URL cannot start with hyphen")
	}

	for _, r := range rawURL {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("URL contains control characters")
		}
	}
	return u.String(), nil
}
