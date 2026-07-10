# Cluster 基础设施服务

Karmada 多集群环境下的基础设施工具集，覆盖 CI/CD 任务调度、集群资源运维、Git 缓存加速和镜像分发预热四个核心场景。

---

## 项目概览

| 项目 | 语言 | 职责 | 关键依赖 |
|------|------|------|----------|
| `codearts-workflow-image` | Go | CI/CD 流水线 → Volcano Job 转换与提交 | Volcano, Karmada, Kubernetes API |
| `cronjob` | Python | Karmada 控制面定时清理与卡住资源修复 | Karmada API, urllib |
| `git-cache-http-server` | Haxe → Node.js | 上游 Git 仓库镜像缓存与 HTTP 代理 | Node.js, Git |
| `image_preheat` | Python | Harbor Webhook → 多集群 DaemonSet 镜像预热 | Flask, Kubernetes Python Client, Karmada |

---

## 1. codearts-workflow-image

**将 CI/CD 流水线配置转换为 Volcano Job CRD 并提交到 Karmada 多集群。**

### 核心流程

```
shell.sh + env.sh + workflow_templatev2.yaml
    │
    ▼
Converter: 生成 Volcano Job CRD + K8s Secret
    │
    ▼
Submitter: kubectl apply → Karmada 控制面 → 成员集群调度执行
```

### 关键能力

- **资源规格自动计算**：根据 `CP_runs_on` 解析 CPU/内存/NPU 需求，支持 0-8 卡 Ascend NPU 的自动扩缩
- **密钥安全注入**：敏感环境变量（password/token/secret/key）自动提取为 K8s Secret，通过 `secretKeyRef` 注入，ownerReference 绑定生命周期
- **Git 代码拉取**：支持 gitcode/github/gitee/codehub 多源，通过 CDN 缓存加速，自动处理 PR 合并
- **多集群调度**：PVC 亲和性路由到成员集群，Secret 跟随分发，延迟退出机制确保日志采集完整
- **NPU 全链路**：Ascend 910A/910B/910B4/310P3 芯片支持，ascend-driver hostPath 挂载，NPU 资源请求注入
- **22 个 E2E 测试用例**，覆盖基本执行、密钥管理、Git 克隆、数据集挂载、镜像拉取失败检测等场景

### 技术栈

- **语言**：Go 1.24
- **运行时**：容器化部署（Dockerfile），作为 CI step 镜像
- **API**：直接调用 kubectl（支持 Karmada 代理）
- **核心依赖**：`gopkg.in/yaml.v3`

---

## 2. cronjob

**Karmada 控制面定时运维任务，自动清理和修复集群资源。**

### 子任务

#### `force-patch-stuck-rb/patch.py`
- 扫描所有 `ResourceBinding`，找出 `Scheduled=False` + `reason=SchedulerError` 且持续超过阈值的
- 通过 Karmada API 直接 patch，将卡住的资源强制调度到指定集群
- 支持阈值配置（默认 900s）和默认目标集群配置

#### `vcjob-ttl-cleaner/clean.py`
- 扫描所有 Volcano Job，删除终态超时（`Completed`/`Failed`/`Aborted`/`Terminated`）的 Job
- 输出 ResourceBinding 调度统计和集群 NPU 资源摘要
- 支持 Dry Run 模式预览

### 安全设计

- 不依赖 kubectl 二进制，使用 Python 标准库 `urllib` 直接调用 Karmada API
- 双向 TLS 认证：从 kubeconfig 解析 CA/客户端证书/私钥，构造 SSLContext
- 证书写入临时目录，用后清理

---

## 3. git-cache-http-server

**Git 仓库透明缓存代理，加速集群内 Git 操作。**

### 工作原理

```
git clone https://gitcode.com/org/repo.git
    │  git config url.<cache>.insteadOf https://gitcode.com
    ▼
git-cache-http-server (cluster-local)
    │  首次：从上游 fetch + 缓存到磁盘
    │  后续：直接从缓存响应
    ▼
Pod 内 git clone → 本地网络，秒级完成
```

### 技术特点

- Haxe 源码编译为 Node.js，轻量单进程运行
- 上游认证透传（HTTP Basic Auth），支持 public/private 仓库
- URL 路径第一段作为上游 hostname，自动路由
- 可用于 git clone/fetch，不占用集群外带宽

---

## 4. image_preheat

**Harbor Webhook 触发，在 Karmada 所有成员集群中并行预热容器镜像。**

### 工作流程

```
用户 push 镜像到 Harbor
    ↓
Harbor Webhook → webhook_server.py (Flask)
    ↓
1. 检测镜像架构 (amd64/arm64)
2. 获取 Karmada 成员集群列表
3. 为每个镜像创建 DaemonSet + PropagationPolicy
4. Karmada 分发 DaemonSet 到所有成员集群
5. 每个节点上 kubelet 拉取镜像
6. 轮询 Pod 状态，全部 Running 后清理
```

### 关键设计

- **Vault 安全注入**：kubeconfig 由 Vault Agent Sidecar 注入，读取到内存后即删除磁盘文件
- **Kubernetes Python Client**：直接调用 API，不依赖 kubectl 二进制
- **并发控制**：可配置并行预热镜像数（默认 3），单镜像超时（默认 300s）
- **节点过滤**：支持 `INCLUDE_NODES` / `EXCLUDE_NODES` 指定预热范围
- **集群选择器**：`CLUSTER_SELECTOR` 标签过滤目标集群
- **镜像拉取异常检测**：识别 `ImagePullBackOff`/`ErrImagePull` 等失败状态

---

## 项目关系

```
┌─────────────────────────────────────────────────┐
│                  Harbor Registry                  │
└──────────────────────┬──────────────────────────┘
                       │ push event
                       ▼
┌──────────────────────────────────────────────────┐
│              image_preheat (Webhook)              │
│       创建 DaemonSet → Karmada → 成员集群预热      │
└──────────────────────────────────────────────────┘

┌──────────────────────────────────────────────────┐
│          git-cache-http-server (CDN)             │
│      Git 仓库缓存 → 集群内高速 clone               │
└─────────────────┬────────────────────────────────┘
                  │ git clone via cache
                  ▼
┌──────────────────────────────────────────────────┐
│         codearts-workflow-image (Converter)      │
│     shell.sh + env.sh → Volcano Job → Karmada    │
└──────────────────────┬───────────────────────────┘
                       │ submit
                       ▼
┌──────────────────────────────────────────────────┐
│                  Karmada 控制面                    │
│    ┌──────────────────────────────────────┐      │
│    │         cronjob (运维)                │      │
│    │  • ResourceBinding 卡住修复           │      │
│    │  • Volcano Job 终态清理               │      │
│    └──────────────────────────────────────┘      │
└──────────────────────────────────────────────────┘
```

---

## 部署概述

| 项目 | 部署位置 | 部署方式 |
|------|----------|----------|
| `codearts-workflow-image` | CI 流水线 step 容器镜像 | Docker 构建 |
| `cronjob` | Karmada 控制面集群 CronJob | Docker 构建 |
| `git-cache-http-server` | 集群内部署 Service + Deployment | K8s YAML |
| `image_preheat` | 物理集群（有 Harbor Webhook 网络可达） | K8s Deployment + Vault 注入 |
