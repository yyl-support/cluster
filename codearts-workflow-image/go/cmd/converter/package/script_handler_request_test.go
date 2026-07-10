package converter

import (
	"testing"
)

func Test_gitRequest_GenerateGitCDNConfigScript(t *testing.T) {
	tests := []struct {
		name string
		req  gitRequest
		want string
	}{
		{
			name: "empty GitCacheURLs",
			req: gitRequest{
				GitCacheURLs: nil,
			},
			want: "",
		},
		{
			name: "empty map",
			req: gitRequest{
				GitCacheURLs: map[string]string{},
			},
			want: "",
		},
		{
			name: "single CDN mapping",
			req: gitRequest{
				GitCacheURLs: map[string]string{
					"gitcode": "http://git-cache-gitcode.git-cache.svc.cluster.local:8080",
				},
			},
			want: `git config --global url."http://git-cache-gitcode.git-cache.svc.cluster.local:8080".insteadOf "https://gitcode.com" || echo "WARNING: git config failed for gitcode"
`,
		},
		{
			name: "multiple CDN mappings",
			req: gitRequest{
				GitCacheURLs: map[string]string{
					"gitcode": "http://git-cache-gitcode.git-cache.svc.cluster.local:8080",
					"github":  "http://git-cache-github.git-cache.svc.cluster.local:8080",
					"gitee":   "http://git-cache-gitee.git-cache.svc.cluster.local:8080",
					"atomgit": "http://git-cache-atomgit.git-cache.svc.cluster.local:8080",
					"codehub": "http://git-cache-codehub.git-cache.svc.cluster.local:8080",
				},
			},
			want: `git config --global url."http://git-cache-gitcode.git-cache.svc.cluster.local:8080".insteadOf "https://gitcode.com" || echo "WARNING: git config failed for gitcode"
git config --global url."http://git-cache-github.git-cache.svc.cluster.local:8080".insteadOf "https://gh-proxy.test.osinfra.cn/https://github.com" || echo "WARNING: git config failed for github"
git config --global url."http://git-cache-gitee.git-cache.svc.cluster.local:8080".insteadOf "https://gitee.com" || echo "WARNING: git config failed for gitee"
git config --global url."http://git-cache-atomgit.git-cache.svc.cluster.local:8080".insteadOf "https://atomgit.com" || echo "WARNING: git config failed for atomgit"
git config --global url."http://git-cache-codehub.git-cache.svc.cluster.local:8080".insteadOf "https://codehub.devcloud.cn-north-4.huaweicloud.com" || echo "WARNING: git config failed for codehub"
`,
		},
		{
			name: "partial CDN mappings",
			req: gitRequest{
				GitCacheURLs: map[string]string{
					"gitcode": "http://git-cache-gitcode.git-cache.svc.cluster.local:8080",
					"gitee":   "http://git-cache-gitee.git-cache.svc.cluster.local:8080",
				},
			},
			want: `git config --global url."http://git-cache-gitcode.git-cache.svc.cluster.local:8080".insteadOf "https://gitcode.com" || echo "WARNING: git config failed for gitcode"
git config --global url."http://git-cache-gitee.git-cache.svc.cluster.local:8080".insteadOf "https://gitee.com" || echo "WARNING: git config failed for gitee"
`,
		},
		{
			name: "empty cache URL value",
			req: gitRequest{
				GitCacheURLs: map[string]string{
					"gitcode": "",
					"gitee":   "http://git-cache-gitee.git-cache.svc.cluster.local:8080",
				},
			},
			want: `git config --global url."http://git-cache-gitee.git-cache.svc.cluster.local:8080".insteadOf "https://gitee.com" || echo "WARNING: git config failed for gitee"
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := tt.req.GenerateGitCDNConfigScript()
			if script != tt.want {
				t.Errorf("GenerateGitCDNConfigScript() mismatch:\nGot:\n%s\nWant:\n%s", script, tt.want)
			}
		})
	}
}

func Test_gitRequest_GenerateGitCloneScript(t *testing.T) {
	tests := []struct {
		name string
		req  gitRequest
		want string
	}{
		{
			name: "empty URL",
			req: gitRequest{
				RepoURL: "",
				MergeID: "789",
			},
			want: "",
		},
		{
			name: "unreachable URL",
			req: gitRequest{
				RepoURL:      "https://nonexistent-domain-12345.com/repo.git",
				MergeID:      "111",
				TargetBranch: "main",
			},
			want: "",
		},
		{
			name: "valid URL with mergeID and target branch",
			req: gitRequest{
				RepoURL:      "https://gitcode.com/Ascend/AscendNPU-IR.git",
				MergeID:      "123",
				TargetBranch: "main",
				GitCacheURLs: nil,
			},
			want: `if [ -n "https://gitcode.com/Ascend/AscendNPU-IR.git" ]; then
    CLONE_OK=true
    git clone --single-branch -b "main" "https://gitcode.com/Ascend/AscendNPU-IR.git" "$WORKSPACE" || {
        mkdir "$WORKSPACE" || echo "WARNING: directory exists"
        echo "WARNING: 主项目拉取失败！"
        CLONE_OK=false
    }
    cd "$WORKSPACE"
    if [ "$CLONE_OK" = "true" ]; then
        git fetch origin +refs/merge-requests/123/head:pr
        git config --global user.name "robot"
        git config --global user.email "your@emaple.com"
        git merge pr --no-ff -m "Merge main from refs/merge-requests/123/head" || {
            echo "WARNING: 主项目merge失败！"
        }
    fi
fi
`,
		},
		{
			name: "valid URL without mergeID",
			req: gitRequest{
				RepoURL:      "https://gitcode.com/Ascend/AscendNPU-IR.git",
				MergeID:      "",
				TargetBranch: "develop",
				GitCacheURLs: nil,
			},
			want: `if [ -n "https://gitcode.com/Ascend/AscendNPU-IR.git" ]; then
    CLONE_OK=true
    git clone --single-branch -b "develop" "https://gitcode.com/Ascend/AscendNPU-IR.git" "$WORKSPACE" || {
        mkdir "$WORKSPACE" || echo "WARNING: directory exists"
        echo "WARNING: 主项目拉取失败！"
        CLONE_OK=false
    }
    cd "$WORKSPACE"
fi
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := tt.req.GenerateGitCloneScript()
			if script != tt.want {
				t.Errorf("GenerateGitCloneScript() mismatch:\nGot:\n%s\nWant:\n%s", script, tt.want)
			}
		})
	}
}

func Test_delayExitRequest_GenerateDelayExitTrapScript(t *testing.T) {
	tests := []struct {
		name string
		req  delayExitRequest
		want string
	}{
		{
			name: "default 10 seconds",
			req:  delayExitRequest{DelayExitSeconds: 10},
			want: "trap 'EXIT_CODE=$?; [ \"$EXIT_CODE\" -ne 0 ] && sleep 10' EXIT\n",
		},
		{
			name: "custom 30 seconds",
			req:  delayExitRequest{DelayExitSeconds: 30},
			want: "trap 'EXIT_CODE=$?; [ \"$EXIT_CODE\" -ne 0 ] && sleep 30' EXIT\n",
		},
		{
			name: "zero seconds",
			req:  delayExitRequest{DelayExitSeconds: 0},
			want: "trap 'EXIT_CODE=$?; [ \"$EXIT_CODE\" -ne 0 ] && sleep 0' EXIT\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := tt.req.GenerateDelayExitTrapScript()
			if script != tt.want {
				t.Errorf("GenerateDelayExitTrapScript() mismatch:\nGot:\n%s\nWant:\n%s", script, tt.want)
			}
		})
	}
}

func Test_artifactsRequest_GenerateArtifactsCopyScript(t *testing.T) {
	tests := []struct {
		name string
		req  artifactsRequest
		want string
	}{
		{
			name: "empty path",
			req: artifactsRequest{
				Artifacts:           "",
				ArtifactsTempFolder: "/output",
			},
			want: "",
		},
		{
			name: "single pattern",
			req: artifactsRequest{
				Artifacts:           "./code/xihe/tests/ci-test/r*",
				ArtifactsTempFolder: "/output",
			},
			want: `
cd ${WORKSPACE}
cp -r --parents ./code/xihe/tests/ci-test/r* /output/
`,
		},
		{
			name: "multiple patterns",
			req: artifactsRequest{
				Artifacts:           "./code/xihe/tests/ci-test/r*;./code/xihe/tests/*",
				ArtifactsTempFolder: "/output",
			},
			want: `
cd ${WORKSPACE}
cp -r --parents ./code/xihe/tests/ci-test/r* ./code/xihe/tests/* /output/
`,
		},
		{
			name: "multiple patterns with spaces",
			req: artifactsRequest{
				Artifacts:           "./code/xihe/tests/ci-test/r* ; ./code/xihe/tests/*",
				ArtifactsTempFolder: "/output",
			},
			want: `
cd ${WORKSPACE}
cp -r --parents ./code/xihe/tests/ci-test/r* ./code/xihe/tests/* /output/
`,
		},
		{
			name: "patterns with glob and multiple semicolons",
			req: artifactsRequest{
				Artifacts:           "*.txt;*.log;*.json",
				ArtifactsTempFolder: "/output/artifact",
			},
			want: `
cd ${WORKSPACE}
cp -r --parents *.txt *.log *.json /output/artifact/
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := tt.req.GenerateArtifactsCopyScript()
			if script != tt.want {
				t.Errorf("GenerateArtifactsCopyScript() mismatch:\nGot:\n%s\nWant:\n%s", script, tt.want)
			}
		})
	}
}
