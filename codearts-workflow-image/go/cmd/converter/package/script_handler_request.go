package converter

import (
	"fmt"
	"strings"
)

type gitRequest struct {
	RepoURL      string
	MergeID      string
	TargetBranch string
	GitCacheURLs map[string]string
}

func (r gitRequest) GenerateGitCDNConfigScript() string {
	if len(r.GitCacheURLs) == 0 {
		return ""
	}

	var sb strings.Builder

	gitCDNMappings := []struct {
		oldURL   string
		cacheKey string
	}{
		{"https://gitcode.com", "gitcode"},
		{"https://gh-proxy.test.osinfra.cn/https://github.com", "github"},
		{"https://gitee.com", "gitee"},
		{"https://atomgit.com", "atomgit"},
		{"https://codehub.devcloud.cn-north-4.huaweicloud.com", "codehub"},
	}

	for _, mapping := range gitCDNMappings {
		if cacheURL, ok := r.GitCacheURLs[mapping.cacheKey]; ok && cacheURL != "" {
			sb.WriteString(fmt.Sprintf(`git config --global url."%s".insteadOf "%s" || echo "WARNING: git config failed for %s"`+"\n", cacheURL, mapping.oldURL, mapping.cacheKey))
		}
	}

	return sb.String()
}

func (r gitRequest) GenerateGitCloneScript() string {
	if r.RepoURL == "" || !isURLReachable(r.RepoURL) {
		return ""
	}

	var sb strings.Builder

	defaultBranchCmd := fmt.Sprintf("$(git ls-remote --symref %q HEAD | sed -n 's/.*refs\\/heads\\/\\([^[:space:]]*\\).*/\\1/p')", r.RepoURL)
	branch := r.TargetBranch
	if branch == "" {
		branch = defaultBranchCmd
	}

	sb.WriteString(fmt.Sprintf(`if [ -n "%s" ]; then
    CLONE_OK=true
    git clone --single-branch -b "%s" "%s" "$WORKSPACE" || {
        mkdir "$WORKSPACE" || echo "WARNING: directory exists"
        echo "WARNING: 主项目拉取失败！"
        CLONE_OK=false
    }
    cd "$WORKSPACE"
`, r.RepoURL, branch, r.RepoURL))

	if r.MergeID != "" {
		sb.WriteString(fmt.Sprintf(`    if [ "$CLONE_OK" = "true" ]; then
        git fetch origin +refs/merge-requests/%s/head:pr
        git config --global user.name "robot"
        git config --global user.email "your@emaple.com"
        git merge pr --no-ff -m "Merge %s from refs/merge-requests/%s/head" || {
            echo "WARNING: 主项目merge失败！"
        }
    fi
`, r.MergeID, branch, r.MergeID))
	}

	sb.WriteString("fi\n")
	return sb.String()
}

type delayExitRequest struct {
	DelayExitSeconds int
}

// GenerateDelayExitTrapScript returns a shell trap that sleeps
// DelayExitSeconds when the script exits with a non-zero code, giving
// operators a window to inspect the pod before it terminates.
func (r delayExitRequest) GenerateDelayExitTrapScript() string {
	return fmt.Sprintf("trap 'EXIT_CODE=$?; [ \"$EXIT_CODE\" -ne 0 ] && sleep %d' EXIT\n", r.DelayExitSeconds)
}

type artifactsRequest struct {
	Artifacts           string
	ArtifactsTempFolder string
}

func (r artifactsRequest) GenerateArtifactsCopyScript() string {
	if r.Artifacts == "" {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\ncd ${WORKSPACE}\n")

	patterns := strings.Split(r.Artifacts, ";")
	var trimmedPatterns []string
	for _, p := range patterns {
		p = strings.TrimSpace(p)
		if p != "" {
			trimmedPatterns = append(trimmedPatterns, p)
		}
	}

	if len(trimmedPatterns) > 0 {
		sb.WriteString(fmt.Sprintf("cp -r --parents %s %s/\n", strings.Join(trimmedPatterns, " "), r.ArtifactsTempFolder))
	}

	return sb.String()
}
