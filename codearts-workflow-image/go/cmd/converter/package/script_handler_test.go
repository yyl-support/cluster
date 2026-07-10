package converter

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func Test_handlerScript_Execute(t *testing.T) {
	tests := []struct {
		name          string
		scriptContent string
		envVars       map[string]string
		expectError   bool
		expectedLog   []string
	}{
		{
			name:          "valid URL with mergeID and invalid branch",
			scriptContent: "echo 'hello world'",
			envVars: map[string]string{
				"CP_repo_url":      "https://gitcode.com/Ascend/AscendNPU-IR.git",
				"CP_merge_id":      "700",
				"CP_target_branch": "main",
			},
			expectError: false,
			expectedLog: []string{"WARNING: 主项目拉取失败！"},
		},
		{
			name:          "valid URL without mergeID and invalid branch",
			scriptContent: "echo 'hello world'",
			envVars: map[string]string{
				"CP_repo_url":      "https://gitcode.com/Ascend/AscendNPU-IR.git",
				"CP_merge_id":      "",
				"CP_target_branch": "develop",
			},
			expectError: false,
			expectedLog: []string{"WARNING: 主项目拉取失败！"},
		},
		{
			name:          "valid URL with empty branch (default detection) - clone only, no merge",
			scriptContent: "echo 'hello world'",
			envVars: map[string]string{
				"CP_repo_url":      "https://gitcode.com/Ascend/AscendNPU-IR.git",
				"CP_merge_id":      "",
				"CP_target_branch": "",
			},
			expectError: false,
			expectedLog: []string{"hello world"},
		},
		{
			name:          "valid URL with empty branch (default detection) - clone and merge success",
			scriptContent: "echo 'hello world'",
			envVars: map[string]string{
				"CP_repo_url":      "https://gitcode.com/Ascend/AscendNPU-IR.git",
				"CP_merge_id":      "700",
				"CP_target_branch": "",
			},
			expectError: false,
			expectedLog: []string{"hello world"},
		},
		{
			name:          "empty URL - no clone, script runs",
			scriptContent: "echo 'hello world'",
			envVars: map[string]string{
				"CP_repo_url":      "",
				"CP_merge_id":      "789",
				"CP_target_branch": "main",
			},
			expectError: false,
			expectedLog: []string{"hello world"},
		},
		{
			name:          "unreachable URL - no clone, script runs",
			scriptContent: "echo 'unreachable test'",
			envVars: map[string]string{
				"CP_repo_url":      "https://nonexistent-domain-12345.com/repo.git",
				"CP_merge_id":      "111",
				"CP_target_branch": "main",
			},
			expectError: false,
			expectedLog: []string{"unreachable test"},
		},
		{
			name:          "invalid URL - no clone, script runs",
			scriptContent: "echo 'invalid test'",
			envVars: map[string]string{
				"CP_repo_url":      "https://invalid-domain-xyz123.com/repo.git",
				"CP_merge_id":      "222",
				"CP_target_branch": "main",
			},
			expectError: false,
			expectedLog: []string{"invalid test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := exec.Command("git", "config", "--global", "user.name")
			origName, _ := cmd.Output()
			cmd = exec.Command("git", "config", "--global", "user.email")
			origEmail, _ := cmd.Output()
			defer func() {
				exec.Command("git", "config", "--global", "user.name", strings.TrimSpace(string(origName))).Run()
				exec.Command("git", "config", "--global", "user.email", strings.TrimSpace(string(origEmail))).Run()
			}()

			repoURL := tt.envVars["CP_repo_url"]
			mergeID := tt.envVars["CP_merge_id"]
			targetBranch := tt.envVars["CP_target_branch"]

			script := handlerScript(tt.scriptContent, gitRequest{RepoURL: repoURL, MergeID: mergeID, TargetBranch: targetBranch, GitCacheURLs: nil}, artifactsRequest{}, delayExitRequest{DelayExitSeconds: 10})

			tmpDir, err := os.MkdirTemp("", "test_script_exec_*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			tmpFile, err := os.CreateTemp(tmpDir, "test_script_*.sh")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())
			defer tmpFile.Close()

			if _, err := tmpFile.WriteString(script); err != nil {
				t.Fatalf("Failed to write script: %v", err)
			}
			tmpFile.Close()

			if err := os.Chmod(tmpFile.Name(), 0755); err != nil {
				t.Fatalf("Failed to chmod: %v", err)
			}

			cmd = exec.Command("bash", "-n", tmpFile.Name())
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Errorf("Script syntax check failed: %v\nOutput: %s", err, output)
			}

			workspaceDir := filepath.Join(tmpDir, "workspace")
			os.MkdirAll(workspaceDir, 0755)

			env := os.Environ()
			env = append(env, "WORKSPACE="+workspaceDir)

			cmd = exec.Command("bash", tmpFile.Name())
			cmd.Dir = tmpDir
			cmd.Env = env

			output, err := cmd.CombinedOutput()
			t.Logf("Script output:\n%s", string(output))

			if tt.expectError {
				if err == nil {
					t.Errorf("Expected script to fail, but it succeeded")
				}
			} else {
				if err != nil {
					t.Errorf("Expected script to succeed, but got error: %v", err)
				}
			}

			if tt.expectedLog != nil {
				outputStr := string(output)
				for _, expected := range tt.expectedLog {
					if !strings.Contains(outputStr, expected) {
						t.Errorf("Expected log %q not found in output: %s", expected, outputStr)
					}
				}
			}
		})
	}
}

func Test_handlerScript_DelayExitTrap(t *testing.T) {
	tests := []struct {
		name             string
		delayExitSeconds int
		wantTrapLine     string
	}{
		{
			name:             "default 10 seconds",
			delayExitSeconds: 10,
			wantTrapLine:     "trap 'EXIT_CODE=$?; [ \"$EXIT_CODE\" -ne 0 ] && sleep 10' EXIT",
		},
		{
			name:             "custom 30 seconds",
			delayExitSeconds: 30,
			wantTrapLine:     "trap 'EXIT_CODE=$?; [ \"$EXIT_CODE\" -ne 0 ] && sleep 30' EXIT",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			script := handlerScript(
				"echo 'user script'",
				gitRequest{RepoURL: "", MergeID: "", TargetBranch: "", GitCacheURLs: nil},
				artifactsRequest{},
				delayExitRequest{DelayExitSeconds: tt.delayExitSeconds},
			)

			lines := strings.Split(script, "\n")
			if len(lines) == 0 || lines[0] != tt.wantTrapLine {
				t.Errorf("expected script to start with trap line %q, got first line %q\nFull script:\n%s", tt.wantTrapLine, lines[0], script)
			}

			trapIdx := strings.Index(script, "trap ")
			userIdx := strings.Index(script, "echo 'user script'")
			if trapIdx == -1 || userIdx == -1 || trapIdx > userIdx {
				t.Errorf("expected trap to appear before user script content, script:\n%s", script)
			}
		})
	}
}

func Test_isURLReachable(t *testing.T) {
	tests := []struct {
		url           string
		wantReachable bool
	}{
		{"https://gitcode.com/Ascend/AscendNPU-IR.git", true},
		{"https://nonexistent-domain-12345.com/repo.git", false},
		{"http://invalid-domain-xyz123.com/repo.git", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := isURLReachable(tt.url)
			if got != tt.wantReachable {
				t.Errorf("isURLReachable(%q) = %v, want %v", tt.url, got, tt.wantReachable)
			}
		})
	}
}
