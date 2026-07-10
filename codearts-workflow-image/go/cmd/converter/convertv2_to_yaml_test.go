package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/opensourceways/codearts-workflow-image-go/cmd/common/testutil"
)

var regenerate bool

func init() {
	flag.BoolVar(&regenerate, "regenerate", false, "Regenerate expected.yaml files instead of testing")
}

func Test_main(t *testing.T) {
	tests := []struct {
		name             string
		testDir          string
		wantSecret       bool
		wantCopyPod      bool
		dynamicTimestamp bool
	}{
		{name: "simple", testDir: "case/newtest/test1-simple", wantSecret: false, wantCopyPod: false},
		{name: "with-secrets", testDir: "case/newtest/test2-with-secrets", wantSecret: false, wantCopyPod: false},
		{name: "custom-resources", testDir: "case/newtest/test3-custom-resources", wantSecret: false, wantCopyPod: false},
		{name: "custom-image", testDir: "case/newtest/test4-custom-image", wantSecret: false, wantCopyPod: false},
		{name: "no-merge-id", testDir: "case/newtest/test5-no-merge-id", wantSecret: false, wantCopyPod: false},
		{name: "empty-sensitive-value", testDir: "case/newtest/test6-empty-sensitive-value", wantSecret: false, wantCopyPod: false},
		{name: "workspace-filtered", testDir: "case/newtest/test7-workspace-filtered", wantSecret: false, wantCopyPod: false},
		{name: "git-clone", testDir: "case/newtest/test8-git-clone", wantSecret: false, wantCopyPod: false},
		{name: "910b4", testDir: "case/newtest/test9-910b4", wantSecret: false, wantCopyPod: false},
		{name: "git-clone-var-ref", testDir: "case/newtest/test11-git-clone-var-ref", wantSecret: false, wantCopyPod: false},
		{name: "test14-exit1", testDir: "case/newtest/test14-exit1", wantSecret: false, wantCopyPod: false},
		{name: "test15-dataset", testDir: "case/newtest/test15-dataset", wantSecret: false, wantCopyPod: false},
		{name: "test16-dataset-mapping", testDir: "case/newtest/test16-dataset-mapping", wantSecret: false, wantCopyPod: false},
		{name: "test12-normal-workflow", testDir: "case/newtest/test12-normal-workflow", wantSecret: false, wantCopyPod: false},
		{name: "test17-image-pull-failure", testDir: "case/newtest/test17-image-pull-failure", wantSecret: false, wantCopyPod: false},
		{name: "test18-with-secrets", testDir: "case/newtest/test18-with-secrets", wantSecret: false, wantCopyPod: false},
		{name: "test19-dynamic-timestamp", testDir: "case/newtest/test19-dynamic-timestamp", wantSecret: false, wantCopyPod: false, dynamicTimestamp: true},
		{name: "test20-ascend-driver", testDir: "case/newtest/test20-ascend-driver", wantSecret: false, wantCopyPod: false},
		// {name: "test21-ipv6-verify", testDir: "case/newtest/test21-ipv6-verify", wantSecret: false, wantCopyPod: false},
		{name: "test21-dataset", testDir: "case/newtest/test21-dataset", wantSecret: false, wantCopyPod: false},
		{name: "test22-git-cdn", testDir: "case/newtest/test22-git-cdn", wantSecret: false, wantCopyPod: false},
		{name: "test23-large-queue", testDir: "case/newtest/test23-large-queue", wantSecret: false, wantCopyPod: false},
		{name: "test24-image-proxy", testDir: "case/newtest/test24-image-proxy", wantSecret: false, wantCopyPod: false},
		{name: "test10-cp-artifacts", testDir: "case/newtest/test10-cp-artifacts", wantSecret: false, wantCopyPod: false},
		{name: "test13-cp-artifacts-v2", testDir: "case/newtest/test13-cp-artifacts-v2", wantSecret: false, wantCopyPod: false},
		{name: "test26-npu-generic", testDir: "case/newtest/test26-npu-generic", wantSecret: false, wantCopyPod: false},
		{name: "test27-310p3", testDir: "case/newtest/test27-310p3", wantSecret: false, wantCopyPod: false},
		{name: "test28-gz-dataset", testDir: "case/newtest/test28-gz-dataset", wantSecret: false, wantCopyPod: false},
		{name: "test25-shm", testDir: "case/newtest/test25-shm", wantSecret: false, wantCopyPod: false},
		{name: "test29-cp-artifact-failure", testDir: "case/newtest/test29-cp-artifact-failure", wantSecret: false, wantCopyPod: false},
		{name: "test30-cp-pull-failure", testDir: "case/newtest/test30-cp-pull-failure", wantSecret: false, wantCopyPod: false},
		{name: "test31-git-clone-cp-artifacts", testDir: "case/newtest/test31-git-clone-cp-artifacts", wantSecret: false, wantCopyPod: false},
		{name: "test32-no-artifact-files", testDir: "case/newtest/test32-no-artifact-files", wantSecret: false, wantCopyPod: false},
		{name: "test33-goproxy", testDir: "case/newtest/test33-goproxy", wantSecret: false, wantCopyPod: false},
		{name: "test34-ipv6", testDir: "case/newtest/test34-ipv6", wantSecret: false, wantCopyPod: false},
		{name: "test35-ingress-bandwidth", testDir: "case/newtest/test35-ingress-bandwidth", wantSecret: false, wantCopyPod: false},
		{name: "test36-image-pull-policy", testDir: "case/newtest/test36-image-pull-policy", wantSecret: false, wantCopyPod: false},
		{name: "test37-delay-exit", testDir: "case/newtest/test37-delay-exit", wantSecret: false, wantCopyPod: false},
		{name: "test38-dataset-readonly", testDir: "case/newtest/test38-dataset-readonly", wantSecret: false, wantCopyPod: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			absTestDir, err := filepath.Abs(tt.testDir)
			if err != nil {
				t.Fatalf("获取测试目录绝对路径失败: %v", err)
			}

			envsb, err := os.ReadFile(filepath.Join(tt.testDir, "env.sh"))
			if err != nil {
				t.Fatalf("读取环境变量文件失败: %v", err)
			}

			envs := strings.SplitSeq(string(envsb), "\n")
			for env := range envs {
				env = strings.TrimSpace(env)
				if env == "" || strings.HasPrefix(env, "#") {
					continue
				}

				if !strings.HasPrefix(env, "export ") {
					continue
				}

				env = strings.TrimPrefix(env, "export ")
				keyValue := strings.SplitN(env, "=", 2)
				if len(keyValue) != 2 {
					continue
				}

				key := keyValue[0]
				value := strings.Trim(keyValue[1], `"'`)
				t.Setenv(key, value)
			}

			t.Setenv("WORKSPACE", absTestDir)

			_ = os.Remove("./workflow_trans.yaml")
			_ = os.Remove("./workflow_trans-secret.yaml")

			outputFile := "./workflow_trans.yaml"
			workflowTemplate := os.Getenv("workflow_template")
			customTemplate := filepath.Join(tt.testDir, "workflow_templatev2.yaml")
			if _, err := os.Stat(customTemplate); err == nil {
				workflowTemplate = customTemplate
			}
			args := []string{"cmd"}
			if workflowTemplate != "" {
				args = append(args, "-t", workflowTemplate)
			}
			args = append(args, "-o", outputFile)
			os.Args = args

			main()

			got, err := os.ReadFile(outputFile)
			if err != nil {
				t.Fatalf("读取生成的 workflow 文件失败: %v", err)
			}

			wantFile := filepath.Join(tt.testDir, "expected.yaml")

			var want []byte
			if regenerate {
				if err := os.WriteFile(wantFile, got, 0600); err != nil {
					t.Fatalf("写入期望 workflow 文件失败: %v", err)
				}
				t.Logf("Regenerated: %s", wantFile)
			} else if tt.dynamicTimestamp {
				want = got
				if err := os.WriteFile(wantFile, got, 0600); err != nil {
					t.Fatalf("写入动态 workflow 文件失败: %v", err)
				}
			} else {
				want, err = os.ReadFile(wantFile)
				if err != nil {
					t.Fatalf("读取期望文件失败: %v", err)
				}

				if ok, err := testutil.YamlEqual(got, want); !ok || err != nil {
					t.Errorf("生成的 workflow 不匹配, err=%v\nGot:\n%s\nWant:\n%s", err, string(got), string(want))
				}
			}

			if tt.wantSecret {
				secretFile := "./workflow_trans-secret.yaml"
				gotSecret, err := os.ReadFile(secretFile)
				if err != nil {
					t.Fatalf("读取生成的 secret 文件失败: %v", err)
				}

				wantSecretFile := filepath.Join(tt.testDir, "expected-secret.yaml")

				if regenerate {
					if err := os.WriteFile(wantSecretFile, gotSecret, 0600); err != nil {
						t.Fatalf("写入期望 secret 文件失败: %v", err)
					}
					t.Logf("Regenerated: %s", wantSecretFile)
				} else if tt.dynamicTimestamp {
					wantSecret := gotSecret
					if ok, err := testutil.YamlEqual(gotSecret, wantSecret); !ok || err != nil {
						t.Errorf("生成的 secret 不匹配, err=%v\nGot:\n%s\nWant:\n%s", err, string(gotSecret), string(wantSecret))
					}
					if err := os.WriteFile(wantSecretFile, gotSecret, 0600); err != nil {
						t.Fatalf("写入动态 secret 文件失败: %v", err)
					}
				} else {
					wantSecret, err := os.ReadFile(wantSecretFile)
					if err != nil {
						t.Fatalf("读取期望的 secret 文件失败: %v", err)
					}
					if ok, err := testutil.YamlEqual(gotSecret, wantSecret); !ok || err != nil {
						t.Errorf("生成的 secret 不匹配, err=%v\nGot:\n%s\nWant:\n%s", err, string(gotSecret), string(wantSecret))
					}
				}
			} else {
				if _, err := os.Stat("./workflow_trans-secret.yaml"); err == nil {
					t.Error("不应生成 secret 文件，但生成了")
				}
			}

			if tt.wantCopyPod {
				copyPodFile := "./workflow_trans-copy-pod.yaml"
				gotCopyPod, err := os.ReadFile(copyPodFile)
				if err != nil {
					t.Fatalf("读取生成的 CopyPod 文件失败: %v", err)
				}

				wantCopyPodFile := filepath.Join(tt.testDir, "expected-copy-pod.yaml")
				wantCopyPod, err := os.ReadFile(wantCopyPodFile)
				if err != nil {
					t.Fatalf("读取期望的 CopyPod 文件失败: %v", err)
				}

				if ok, err := testutil.YamlEqual(gotCopyPod, wantCopyPod); !ok || err != nil {
					t.Errorf("生成的 CopyPod 不匹配, err=%v\nGot:\n%s\nWant:\n%s", err, string(gotCopyPod), string(wantCopyPod))
				}
			} else {
				if _, err := os.Stat("./workflow_trans-copy-pod.yaml"); err == nil {
					t.Error("不应生成 CopyPod 文件，但生成了")
				}
			}

			_ = os.Remove(outputFile)
			_ = os.Remove("./workflow_trans-secret.yaml")
			_ = os.Remove("./workflow_trans-copy-pod.yaml")
		})
	}
}

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
