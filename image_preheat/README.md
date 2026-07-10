# Karmada 多集群镜像预热服务

在物理集群部署 webhook 服务，通过 Vault 注入的 kubeconfig 访问 Karmada API，自动在所有成员集群预热镜像。

## 工作原理

```
用户 push 镜像到 Harbor
    ↓
Harbor webhook → 物理集群 webhook 服务
    ↓
webhook 启动:
  1. 从 Vault 注入的路径读取 kubeconfig
  2. 加载到内存，删除文件
  3. 通过 Kubernetes API 访问 Karmada
    ↓
预热流程:
  1. 检测镜像架构
  2. 获取成员集群列表
  3. 创建 DaemonSet + PropagationPolicy
  4. Karmada 分发到成员集群
  5. 成员集群节点拉镜像
  6. 检查状态，清理 DaemonSet
```

## 部署步骤

### 1. 构建镜像

```bash
export REGISTRY=harbor.example.com/library
./build.sh
```

### 2. 配置 Vault

确保 Vault 中存储了 Karmada kubeconfig，deployment.yaml 中的 Vault 注入配置正确：

```yaml
annotations:
  vault.hashicorp.com/agent-inject: "true"
  vault.hashicorp.com/role: "preheat-webhook"
  vault.hashicorp.com/agent-inject-secret-kubeconfig: "secret/data/karmada/kubeconfig"
```

### 3. 部署

```bash
./deploy.sh
```

### 4. 配置 Harbor

- Endpoint: `http://harbor-preheat-webhook.preheat:8080/webhook`
- Event Type: `PUSH_ARTIFACT` 或 `PULL_ARTIFACT`

## 配置项

| 环境变量 | 说明 | 默认值 |
|---|---|---|
| **KUBECONFIG_PATH** | kubeconfig 文件路径（Vault 注入） | `/vault/secrets/kubeconfig` |
| **NAMESPACE** | 命名空间 | `preheat` |
| **CLUSTER_SELECTOR** | 集群标签过滤 | 空 |
| **PARALLEL_IMAGES** | 并发预热镜像数 | `3` |
| **TIMEOUT** | 单个镜像超时秒数（0=无限） | `300` |
| **INCLUDE_NODES** | 指定节点 | 空 |
| **EXCLUDE_NODES** | 排除节点 | 空 |

## 文件结构

```
glm/
├── build.sh                 # 构建镜像
├── deploy.sh                # 部署脚本
├── Dockerfile               # 不含 kubectl
├── webhook_server.py        # Webhook 服务（Kubernetes API）
├── requirements.txt         # flask, pyyaml, kubernetes
├── README.md
└── deploy/
    ├── deployment.yaml      # Deployment + Vault 注入
    └── service.yaml
```

## 关键说明

1. kubeconfig 由 Vault 注入，读取到内存后删除文件
2. 使用 Kubernetes Python client 直接调用 API，不依赖 kubectl
3. webhook 部署在物理集群，Karmada 控制面不运行工作负载