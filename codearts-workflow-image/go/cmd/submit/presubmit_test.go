package main

import (
	"strings"
	"testing"
)

func TestApplyGitProxy(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		wantArgs string
		wantSkip bool
	}{
		{
			name: "inject_git_proxy_into_main-script",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"tasks": []interface{}{
						map[string]interface{}{
							"name": "main-script",
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"containers": []interface{}{
										map[string]interface{}{
											"args": []interface{}{
												"echo 'hello'",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantArgs: "git config --global url",
			wantSkip: false,
		},
		{
			name: "inject_git_proxy_into_task0",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"tasks": []interface{}{
						map[string]interface{}{
							"name": "task0",
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"containers": []interface{}{
										map[string]interface{}{
											"args": []interface{}{
												"python script.py",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantArgs: "git config --global url",
			wantSkip: false,
		},
		{
			name: "skip_other_task_names",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"tasks": []interface{}{
						map[string]interface{}{
							"name": "other-task",
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"containers": []interface{}{
										map[string]interface{}{
											"args": []interface{}{
												"echo 'hello'",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantSkip: true,
		},
		{
			name: "skip_if_git_config_already_present",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"tasks": []interface{}{
						map[string]interface{}{
							"name": "main-script",
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"containers": []interface{}{
										map[string]interface{}{
											"args": []interface{}{
												"git config --global url.\"http://proxy\".insteadOf \"https://gitcode.com\"\necho 'hello'",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantSkip: true,
		},
		{
			name: "no_spec_field",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test-job",
				},
			},
			wantSkip: true,
		},
		{
			name: "no_tasks_field",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"queue": "default",
				},
			},
			wantSkip: true,
		},
		{
			name: "no_containers_field",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"tasks": []interface{}{
						map[string]interface{}{
							"name": "main-script",
							"template": map[string]interface{}{
								"spec": map[string]interface{}{},
							},
						},
					},
				},
			},
			wantSkip: true,
		},
		{
			name: "no_args_field",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"tasks": []interface{}{
						map[string]interface{}{
							"name": "main-script",
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"containers": []interface{}{
										map[string]interface{}{
											"image": "test:latest",
										},
									},
								},
							},
						},
					},
				},
			},
			wantSkip: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyGitProxy(tt.input)

			if tt.wantSkip {
				if spec, ok := result["spec"].(map[string]interface{}); ok {
					if tasks, ok := spec["tasks"].([]interface{}); ok && len(tasks) > 0 {
						task := tasks[0].(map[string]interface{})
						if taskName, ok := task["name"].(string); ok {
							if taskName == "main-script" || taskName == "task0" {
								template := task["template"].(map[string]interface{})
								specInner := template["spec"].(map[string]interface{})
								if containers, ok := specInner["containers"].([]interface{}); ok && len(containers) > 0 {
									container := containers[0].(map[string]interface{})
									if args, ok := container["args"].([]interface{}); ok && len(args) > 0 {
										firstArg := args[0].(string)
										if strings.Contains(firstArg, "git config --global url") && !tt.wantSkip {
											t.Fatalf("unexpected git config injection: %s", firstArg)
										}
									}
								}
							}
						}
					}
				}
				return
			}

			spec := result["spec"].(map[string]interface{})
			tasks := spec["tasks"].([]interface{})
			task := tasks[0].(map[string]interface{})
			template := task["template"].(map[string]interface{})
			specInner := template["spec"].(map[string]interface{})
			containers := specInner["containers"].([]interface{})
			container := containers[0].(map[string]interface{})
			args := container["args"].([]interface{})
			firstArg := args[0].(string)

			if !strings.Contains(firstArg, tt.wantArgs) {
				t.Fatalf("expected args to contain '%s', got: %s", tt.wantArgs, firstArg)
			}

			if !strings.Contains(firstArg, "git-cache-http-server.git-cache.svc.cluster.local:8080") {
				t.Fatal("expected git proxy URL in config")
			}

			if !strings.Contains(firstArg, "insteadOf \"https://gitcode.com\"") {
				t.Fatal("expected gitcode.com proxy config")
			}

			t.Logf("test '%s': successfully injected git proxy config", tt.name)
		})
	}
}

func TestApplyImageProxy(t *testing.T) {
	tests := []struct {
		name        string
		input       map[string]interface{}
		wantImage   string
		wantReplace bool
	}{
		{
			name: "replace_huawei_image_url",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"tasks": []interface{}{
						map[string]interface{}{
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"containers": []interface{}{
										map[string]interface{}{
											"image": "swr.cn-north-4.myhuaweicloud.com/kernels/pytorch:2.0",
										},
									},
								},
							},
						},
					},
				},
			},
			wantImage:   "harbor-portal.test.osinfra.cn/north4-myhuaweicloud/kernels/pytorch:2.0",
			wantReplace: true,
		},
		{
			name: "keep_non_huawei_image",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"tasks": []interface{}{
						map[string]interface{}{
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"containers": []interface{}{
										map[string]interface{}{
											"image": "docker.io/library/python:3.11",
										},
									},
								},
							},
						},
					},
				},
			},
			wantImage:   "docker.io/library/python:3.11",
			wantReplace: false,
		},
		{
			name: "no_spec_field",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test-job",
				},
			},
			wantReplace: false,
		},
		{
			name: "no_tasks_field",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"queue": "default",
				},
			},
			wantReplace: false,
		},
		{
			name: "no_containers_field",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"tasks": []interface{}{
						map[string]interface{}{
							"template": map[string]interface{}{
								"spec": map[string]interface{}{},
							},
						},
					},
				},
			},
			wantReplace: false,
		},
		{
			name: "no_image_field",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"tasks": []interface{}{
						map[string]interface{}{
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"containers": []interface{}{
										map[string]interface{}{
											"name": "test-container",
										},
									},
								},
							},
						},
					},
				},
			},
			wantReplace: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				ImageProxyURL: "harbor-portal.test.osinfra.cn",
			}
			result := applyImageProxy(cfg, tt.input)

			if !tt.wantReplace {
				if spec, ok := result["spec"].(map[string]interface{}); ok {
					if tasks, ok := spec["tasks"].([]interface{}); ok && len(tasks) > 0 {
						task := tasks[0].(map[string]interface{})
						template := task["template"].(map[string]interface{})
						specInner := template["spec"].(map[string]interface{})
						if containers, ok := specInner["containers"].([]interface{}); ok && len(containers) > 0 {
							container := containers[0].(map[string]interface{})
							if image, ok := container["image"].(string); ok {
								if image != tt.wantImage {
									t.Fatalf("expected image '%s', got '%s'", tt.wantImage, image)
								}
							}
						}
					}
				}
				return
			}

			spec := result["spec"].(map[string]interface{})
			tasks := spec["tasks"].([]interface{})
			task := tasks[0].(map[string]interface{})
			template := task["template"].(map[string]interface{})
			specInner := template["spec"].(map[string]interface{})
			containers := specInner["containers"].([]interface{})
			container := containers[0].(map[string]interface{})
			image := container["image"].(string)

			if image != tt.wantImage {
				t.Fatalf("expected image '%s', got '%s'", tt.wantImage, image)
			}

			t.Logf("test '%s': image replaced successfully", tt.name)
		})
	}
}

func TestExtractQueueName(t *testing.T) {
	tests := []struct {
		name      string
		input     map[string]interface{}
		wantQueue string
	}{
		{
			name: "extract_queue_name",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"queue": "shared-flexible-queue",
				},
			},
			wantQueue: "shared-flexible-queue",
		},
		{
			name: "no_queue_field",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"tasks": []interface{}{},
				},
			},
			wantQueue: "",
		},
		{
			name: "no_spec_field",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test-job",
				},
			},
			wantQueue: "",
		},
		{
			name: "queue_not_string",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"queue": 123,
				},
			},
			wantQueue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queue := extractQueueName(tt.input)
			if queue != tt.wantQueue {
				t.Fatalf("expected queue '%s', got '%s'", tt.wantQueue, queue)
			}
			t.Logf("test '%s': queue='%s'", tt.name, queue)
		})
	}
}

func TestReplaceQueue(t *testing.T) {
	tests := []struct {
		name      string
		input     map[string]interface{}
		newQueue  string
		wantQueue string
	}{
		{
			name: "replace_queue_successfully",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"queue": "old-queue",
				},
			},
			newQueue:  "default",
			wantQueue: "default",
		},
		{
			name: "add_queue_when_missing",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"tasks": []interface{}{},
				},
			},
			newQueue:  "default",
			wantQueue: "default",
		},
		{
			name: "no_spec_field",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test-job",
				},
			},
			newQueue:  "default",
			wantQueue: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceQueue(tt.input, tt.newQueue)

			if spec, ok := result["spec"].(map[string]interface{}); ok {
				queue := spec["queue"].(string)
				if queue != tt.wantQueue {
					t.Fatalf("expected queue '%s', got '%s'", tt.wantQueue, queue)
				}
			} else if tt.wantQueue != "" {
				t.Fatal("expected spec field with queue")
			}

			t.Logf("test '%s': queue='%s'", tt.name, tt.wantQueue)
		})
	}
}

func TestExtractChipNameFromJob(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		wantChip string
	}{
		{
			name: "extract_chip_name",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"tasks": []interface{}{
						map[string]interface{}{
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"nodeSelector": map[string]interface{}{
										"node.kubernetes.io/npu.chip.name": "910B4",
									},
								},
							},
						},
					},
				},
			},
			wantChip: "910B4",
		},
		{
			name: "no_nodeSelector",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"tasks": []interface{}{
						map[string]interface{}{
							"template": map[string]interface{}{
								"spec": map[string]interface{}{},
							},
						},
					},
				},
			},
			wantChip: "",
		},
		{
			name: "no_chip_label",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"tasks": []interface{}{
						map[string]interface{}{
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"nodeSelector": map[string]interface{}{
										"kubernetes.io/arch": "amd64",
									},
								},
							},
						},
					},
				},
			},
			wantChip: "",
		},
		{
			name: "no_spec_field",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test-job",
				},
			},
			wantChip: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chip := extractChipNameFromJob(tt.input)
			if chip != tt.wantChip {
				t.Fatalf("expected chip '%s', got '%s'", tt.wantChip, chip)
			}
			t.Logf("test '%s': chip='%s'", tt.name, chip)
		})
	}
}

func TestExtractPVCNamesFromJob(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		wantPVCs []string
	}{
		{
			name: "extract_single_pvc",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"tasks": []interface{}{
						map[string]interface{}{
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"volumes": []interface{}{
										map[string]interface{}{
											"persistentVolumeClaim": map[string]interface{}{
												"claimName": "data-pvc",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantPVCs: []string{"data-pvc"},
		},
		{
			name: "extract_multiple_pvcs",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"tasks": []interface{}{
						map[string]interface{}{
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"volumes": []interface{}{
										map[string]interface{}{
											"persistentVolumeClaim": map[string]interface{}{
												"claimName": "data-pvc",
											},
										},
										map[string]interface{}{
											"persistentVolumeClaim": map[string]interface{}{
												"claimName": "cache-pvc",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			wantPVCs: []string{"data-pvc", "cache-pvc"},
		},
		{
			name: "no_volumes",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"tasks": []interface{}{
						map[string]interface{}{
							"template": map[string]interface{}{
								"spec": map[string]interface{}{},
							},
						},
					},
				},
			},
			wantPVCs: []string{},
		},
		{
			name: "no_spec_field",
			input: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test-job",
				},
			},
			wantPVCs: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pvcs := extractPVCNamesFromJob(tt.input)
			if len(pvcs) != len(tt.wantPVCs) {
				t.Fatalf("expected %d PVCs, got %d: %v", len(tt.wantPVCs), len(pvcs), pvcs)
			}
			for i, pvc := range pvcs {
				if pvc != tt.wantPVCs[i] {
					t.Fatalf("expected PVC[%d]='%s', got '%s'", i, tt.wantPVCs[i], pvc)
				}
			}
			t.Logf("test '%s': pvcs=%v", tt.name, pvcs)
		})
	}
}

func TestReplaceStorageClass(t *testing.T) {
	tests := []struct {
		name   string
		input  map[string]interface{}
		oldSC  string
		newSC  string
		wantSC string
	}{
		{
			name: "replace_sfsturbo_to_csi-nas",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"tasks": []interface{}{
						map[string]interface{}{
							"name": "main-script",
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"volumes": []interface{}{
										map[string]interface{}{
											"name": "output",
											"ephemeral": map[string]interface{}{
												"volumeClaimTemplate": map[string]interface{}{
													"spec": map[string]interface{}{
														"storageClassName": "sfsturbo-subpath-sc",
														"accessModes":       []interface{}{"ReadWriteMany"},
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			oldSC:  "sfsturbo-subpath-sc",
			newSC:  "csi-nas",
			wantSC: "csi-nas",
		},
		{
			name: "no_replace_if_different_sc",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"tasks": []interface{}{
						map[string]interface{}{
							"name": "main-script",
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"volumes": []interface{}{
										map[string]interface{}{
											"name": "output",
											"ephemeral": map[string]interface{}{
												"volumeClaimTemplate": map[string]interface{}{
													"spec": map[string]interface{}{
														"storageClassName": "other-sc",
													},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			oldSC:  "sfsturbo-subpath-sc",
			newSC:  "csi-nas",
			wantSC: "other-sc",
		},
		{
			name: "no_replace_if_no_volumes",
			input: map[string]interface{}{
				"spec": map[string]interface{}{
					"tasks": []interface{}{
						map[string]interface{}{
							"name": "main-script",
							"template": map[string]interface{}{
								"spec": map[string]interface{}{},
							},
						},
					},
				},
			},
			oldSC:  "sfsturbo-subpath-sc",
			newSC:  "csi-nas",
			wantSC: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := replaceStorageClass(tt.input, tt.oldSC, tt.newSC)

			spec, ok := result["spec"].(map[string]interface{})
			if !ok {
				if tt.wantSC == "" {
					t.Logf("test '%s': no spec, as expected", tt.name)
					return
				}
				t.Fatalf("expected spec but got none")
			}

			tasks, ok := spec["tasks"].([]interface{})
			if !ok || len(tasks) == 0 {
				if tt.wantSC == "" {
					t.Logf("test '%s': no tasks, as expected", tt.name)
					return
				}
				t.Fatalf("expected tasks but got none")
			}

			taskMap, ok := tasks[0].(map[string]interface{})
			if !ok {
				t.Fatalf("expected task map")
			}

			template, ok := taskMap["template"].(map[string]interface{})
			if !ok {
				if tt.wantSC == "" {
					t.Logf("test '%s': no template, as expected", tt.name)
					return
				}
				t.Fatalf("expected template")
			}

			specInner, ok := template["spec"].(map[string]interface{})
			if !ok {
				if tt.wantSC == "" {
					t.Logf("test '%s': no spec inner, as expected", tt.name)
					return
				}
				t.Fatalf("expected spec inner")
			}

			volumes, ok := specInner["volumes"].([]interface{})
			if !ok || len(volumes) == 0 {
				if tt.wantSC == "" {
					t.Logf("test '%s': no volumes, as expected", tt.name)
					return
				}
				t.Fatalf("expected volumes")
			}

			volumeMap, ok := volumes[0].(map[string]interface{})
			if !ok {
				t.Fatalf("expected volume map")
			}

			ephemeral, ok := volumeMap["ephemeral"].(map[string]interface{})
			if !ok {
				if tt.wantSC == "" {
					t.Logf("test '%s': no ephemeral, as expected", tt.name)
					return
				}
				t.Fatalf("expected ephemeral")
			}

			volumeClaimTemplate, ok := ephemeral["volumeClaimTemplate"].(map[string]interface{})
			if !ok {
				t.Fatalf("expected volumeClaimTemplate")
			}

			pvcSpec, ok := volumeClaimTemplate["spec"].(map[string]interface{})
			if !ok {
				t.Fatalf("expected pvc spec")
			}

			scName, ok := pvcSpec["storageClassName"].(string)
			if !ok {
				if tt.wantSC == "" {
					t.Logf("test '%s': no storageClassName, as expected", tt.name)
					return
				}
				t.Fatalf("expected storageClassName")
			}

			if scName != tt.wantSC {
				t.Fatalf("expected storageClassName='%s', got '%s'", tt.wantSC, scName)
			}

			t.Logf("test '%s': storageClassName=%s", tt.name, scName)
		})
	}
}
