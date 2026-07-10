//go:build debug

package main

import (
	"testing"
)

func Test_replaceEnvVarsInString(t *testing.T) {
	type args struct {
		input string
		env   map[string]string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "test1",
			args: args{input: test1input,
				env: map[string]string{
					"test_workflow_REPO_URL":      "https://gitcode.com/ascend-archive/test_workflow",
					"test_workflow_TARGET_BRANCH": "main",
					"MERGE_ID":                    "5",
					"gitcode_username":            "ascend-robot",
					"COMMIT_ID":                   "8a66307d20d4055d10176c53e63226a0e26e3dda",
				},
			},
			want: test1want,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			for key, value := range tt.args.env {
				t.Setenv(key, value)

			}

			if got := replaceEnvVarsInString(tt.args.input); got != tt.want {
				t.Errorf("replaceEnvVarsInString() = %v, want %v", got, tt.want)
			}
		})
	}
}

var test1input = `apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: check-lint-
spec:
  templates:
    - name: check-lint
      inputs: {}
      outputs: {}
      metadata: {}
      steps:
        - - name: checkout-code
            template: checkout-code
        - - name: run-pylint
            template: run-pylint
    - name:  checkout-code
      inputs: {}
      outputs: {}
      metadata: {}
      script:
        name: ""
        image: swr.cn-southwest-2.myhuaweicloud.com/modelfoundry/git:latest
        command:
          - sh
        resources: {}
        volumeMounts:
          - name: workspace
            mountPath: /workspace
        source: |
          git clone ${test_workflow_REPO_URL} 
          cd test_workflow
          git checkout ${test_workflow_TARGET_BRANCH}
          git fetch origin +refs/merge-requests/${MERGE_ID}/head:pr
          git config --global user.name ${gitcode_username}
          git config --global user.email "your@emaple.com"
          git merge ${COMMIT_ID} --no-ff -m "Merge ${COMMIT_ID} from refs/merge-requests/${MERGE_ID}/head"  
        workingDir: /workspace

    - name: run-pylint
      inputs: {}
      outputs: {}
      metadata: {}
      script:
        name: ascend
        image: swr.cn-southwest-2.myhuaweicloud.com/base_image/ascend-ci/cann:8.2.rc1-910b-ubuntu22.04-py3.11
        command:
          - sh
          - -c
        args:
          - |

            export cur_path=$(realpath . | sed 's:/*$::')

            git config --global user.name "${gitcode_username}"
            git config --global user.email "your@emaple.com"

            git clone https://gitcode.com/ascend-archive/CI.git repo
            cd repo
            git checkout define-pipeline
            cd ..

            pip config set global.index-url https://mirrors.huaweicloud.com/repository/pypi/simple
            pip3 install -r repo/ci-scripts/ci_template/prepare/requirements.txt

            bash $cur_path/repo/ci-repos/${repo_name}/scripts/lint_test.sh 
            ls

        workingDir: /workspace
        resources:
          limits:
            cpu: "8"
            memory: "8Gi"
          requests:
            cpu: "8"
            memory: "8Gi"
        volumeMounts:
          - name: workspace
            mountPath: /workspace
          - name: driver-tools
            readOnly: true
            mountPath: /usr/local/Ascend/driver/tools
      nodeSelector:
        kubernetes.io/arch: amd64
  entrypoint: check-lint
  arguments: {}
  volumes:
    - name: driver-tools
      hostPath:
        path: /usr/local/Ascend/driver/tools
        type: ""
  volumeClaimTemplates:
    - metadata:
        name: workspace
        creationTimestamp: null
      spec:
        accessModes:
          - ReadWriteMany
        resources:
          requests:
            storage: 1Gi
        storageClassName: sfsturbo-subpath-sc
      status: {}
  imagePullSecrets:
    - name: huawei-swr-image-pull-secret-model-gy
  schedulerName: volcano`

var test1want = `apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: check-lint-
spec:
  templates:
    - name: check-lint
      inputs: {}
      outputs: {}
      metadata: {}
      steps:
        - - name: checkout-code
            template: checkout-code
        - - name: run-pylint
            template: run-pylint
    - name:  checkout-code
      inputs: {}
      outputs: {}
      metadata: {}
      script:
        name: ""
        image: swr.cn-southwest-2.myhuaweicloud.com/modelfoundry/git:latest
        command:
          - sh
        resources: {}
        volumeMounts:
          - name: workspace
            mountPath: /workspace
        source: |
          git clone https://gitcode.com/ascend-archive/test_workflow 
          cd test_workflow
          git checkout main
          git fetch origin +refs/merge-requests/5/head:pr
          git config --global user.name ascend-robot
          git config --global user.email "your@emaple.com"
          git merge 8a66307d20d4055d10176c53e63226a0e26e3dda --no-ff -m "Merge 8a66307d20d4055d10176c53e63226a0e26e3dda from refs/merge-requests/5/head"  
        workingDir: /workspace

    - name: run-pylint
      inputs: {}
      outputs: {}
      metadata: {}
      script:
        name: ascend
        image: swr.cn-southwest-2.myhuaweicloud.com/base_image/ascend-ci/cann:8.2.rc1-910b-ubuntu22.04-py3.11
        command:
          - sh
          - -c
        args:
          - |

            export cur_path=$(realpath . | sed 's:/*$::')

            git config --global user.name "ascend-robot"
            git config --global user.email "your@emaple.com"

            git clone https://gitcode.com/ascend-archive/CI.git repo
            cd repo
            git checkout define-pipeline
            cd ..

            pip config set global.index-url https://mirrors.huaweicloud.com/repository/pypi/simple
            pip3 install -r repo/ci-scripts/ci_template/prepare/requirements.txt

            bash $cur_path/repo/ci-repos/${repo_name}/scripts/lint_test.sh 
            ls

        workingDir: /workspace
        resources:
          limits:
            cpu: "8"
            memory: "8Gi"
          requests:
            cpu: "8"
            memory: "8Gi"
        volumeMounts:
          - name: workspace
            mountPath: /workspace
          - name: driver-tools
            readOnly: true
            mountPath: /usr/local/Ascend/driver/tools
      nodeSelector:
        kubernetes.io/arch: amd64
  entrypoint: check-lint
  arguments: {}
  volumes:
    - name: driver-tools
      hostPath:
        path: /usr/local/Ascend/driver/tools
        type: ""
  volumeClaimTemplates:
    - metadata:
        name: workspace
        creationTimestamp: null
      spec:
        accessModes:
          - ReadWriteMany
        resources:
          requests:
            storage: 1Gi
        storageClassName: sfsturbo-subpath-sc
      status: {}
  imagePullSecrets:
    - name: huawei-swr-image-pull-secret-model-gy
  schedulerName: volcano`
