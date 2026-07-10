# Karmada 配置管理

当前生产环境的 **Karmada 控制平面**配置管理。

**重要说明：**
- 所有配置文件都应用于 **Karmada 控制平面**（通过 karmada-proxy.config）
- Karmada 会根据这些策略自动将工作负载传播到成员集群（member1、member2）
- **不要**将这些 Karmada 资源直接应用到成员集群

## 目录结构

```
configs/
├── apply.sh                    # 应用配置到集群
├── export.sh                   # 从集群导出配置（自动清理运行时字段）
├── clean.sh                    # 清理运行时字段
├── propagation-policies/       # Karmada 传播策略
│   ├── argo-argo-vcjob-policy.yaml
│   ├── default-k8s-job-policy.yaml
│   ├── cluster-argo-namespace-propagation.yaml
│   ├── cluster-volcano-global-all-queue-propagation.yaml
│   └── ...                     # 每个策略单独一个文件
├── override-policies/          # Karmada 覆盖策略（按集群定制配置）
├── queues/                     # Volcano 队列配置
│   ├── default.yaml
│   ├── large-task-shared-queue.yaml
│   ├── shared-flexible-queue.yaml
│   ├── test.yaml
│   └── user1-cpu-queue.yaml
└── rbac/                       # RBAC 权限配置
    ├── vcjob-clusterroles.yaml
    ├── vcjob-clusterrolebindings.yaml
    ├── namespaces.yaml
    └── volcano-global-serviceaccounts.yaml
```

## 配置类型

### 1. PropagationPolicy（命名空间级传播策略）

定义资源如何传播到成员集群，按命名空间隔离。

#### argo 命名空间策略

**argo-vcjob-policy**
- **作用**: 将 Argo 工作流创建的 Volcano Job 传播到 member2 集群
- **目标集群**: member2（单集群）
- **资源选择器**: `batch.volcano.sh/v1alpha1` Job（argo 命名空间所有 Volcano Job）
- **传播策略**: 单集群部署（maxGroups=1, minGroups=1）
- **冲突处理**: Overwrite（覆盖已有资源）
- **用途**: Argo Workflows 提交的 Volcano Job 统一在 member2 集群运行

#### default 命名空间策略

**k8s-job-policy**
- **作用**: 将标准 Kubernetes Job 分发到多集群
- **目标集群**: member1、member2（双集群）
- **资源选择器**: `batch/v1` Job，标签 `volcano-global.io/dispatch=true`
- **副本调度**: Divided（分割副本），Aggregated（聚合优先）
- **冲突处理**: Abort（遇到冲突中止）
- **用途**: 需要跨集群执行的 K8s 批处理任务

**large-task-member2-policy**
- **作用**: 将使用大任务队列的 Volcano Job 传播到 member2 集群
- **目标集群**: member2（单集群）
- **资源选择器**: `batch.volcano.sh/v1alpha1` Job，标签 `queue=large-task-shared-queue`
- **传播策略**: 单集群部署（maxGroups=1, minGroups=1）
- **冲突处理**: Abort（遇到冲突中止）
- **用途**: 高资源需求的大任务统一在 member2 集群执行（member2 有更多资源）

**mindspore-cpu**
- **作用**: 将 MindSpore CPU 训练任务分发到所有集群
- **目标集群**: 所有可用集群（未指定 clusterAffinity）
- **资源选择器**: `batch.volcano.sh/v1alpha1` Job，名称 `mindspore-cpu`
- **副本调度**: Divided（分割副本），Aggregated（聚合优先）
- **冲突处理**: Abort（遇到冲突中止）
- **用途**: MindSpore 深度学习框架的 CPU 训练任务

**test-member1-policy**
- **作用**: 将测试任务传播到 member1 集群
- **目标集群**: member1（单集群）
- **资源选择器**: `batch.volcano.sh/v1alpha1` Job，名称 `test-member1-job`
- **副本调度**: Divided（分割副本），Aggregated（聚合优先）
- **冲突处理**: Abort（遇到冲突中止）
- **用途**: 在 member1 集群进行测试任务验证

**user1-member1-only**
- **作用**: 将 user1 用户的任务传播到 member1 集群
- **目标集群**: member1（单集群）
- **资源选择器**: `batch.volcano.sh/v1alpha1` Job，标签 `user=user1`
- **副本调度**: Duplicated（完整副本，不分割）
- **冲突处理**: Abort（遇到冲突中止）
- **用途**: 为特定用户分配专用集群资源

**volcano-global-dispatch-policy**
- **作用**: 将标记为全局分发的 Volcano Job 分发到所有集群
- **目标集群**: 所有可用集群（未指定 clusterAffinity）
- **资源选择器**: `batch.volcano.sh/v1alpha1` Job，标签 `volcano-global.io/dispatch=true`
- **副本调度**: Divided（分割副本），Aggregated（聚合优先）
- **冲突处理**: Abort（遇到冲突中止）
- **用途**: 需要跨集群并行执行的 Volcano 任务

### 2. ClusterPropagationPolicy（集群级传播策略）

传播集群级资源（Namespace、ClusterRole、ClusterRoleBinding等）。

#### Namespace 传播策略

**argo-namespace-propagation**
- **作用**: 将 argo namespace 传播到所有工作集群
- **目标集群**: member1、member2（双集群）
- **资源选择器**: `v1` Namespace，名称 `argo`
- **副本调度**: Duplicated（完整副本）
- **冲突处理**: Overwrite（覆盖已有资源）
- **用途**: 确保 argo 命名空间在所有成员集群存在，为 Argo Workflows 提供运行环境
- **为什么必须用 ClusterPropagationPolicy**:
  - Namespace 是集群级资源（不属于任何命名空间）
  - PropagationPolicy 无法选择集群级资源
- **实际效果**: Karmada 控制平面创建 argo namespace 后，自动在 member1、member2 两个成员集群都创建相同的 argo namespace

**volcano-global-646d6c6947**
- **作用**: 将 volcano-global namespace 传播到 member2 集群
- **目标集群**: member2（单集群）
- **资源选择器**: `v1` Namespace，名称 `volcano-global`
- **冲突处理**: Overwrite（覆盖已有资源）
- **用途**: volcano-global 命名空间仅在 member2 集群，用于 Volcano 资源管理
- **为什么必须用 ClusterPropagationPolicy**:
  - Namespace 是集群级资源
- **与 argo namespace 的区别**: argo 在所有集群，volcano-global 仅在 member2

#### RBAC 传播策略

**vcjob-logger-54499c54c**
- **作用**: 将 vcjob-logger ClusterRole 传播到 member2 集群
- **目标集群**: member2（单集群）
- **资源选择器**: `rbac.authorization.k8s.io/v1` ClusterRole，名称 `vcjob-logger`
- **冲突处理**: Abort（遇到冲突中止）
- **用途**: 在 member2 集群提供 Volcano Job 日志查看权限
- **为什么必须用 ClusterPropagationPolicy**:
  - ClusterRole 是集群级 RBAC 资源
  - PropagationPolicy 无法选择 ClusterRole
- **实际效果**: Argo Workflows 在 member2 集群可以查看 Volcano Job 的 Pod 日志

**vcjob-logger-559c54456**
- **作用**: 将 vcjob-logger ClusterRoleBinding 传播到 member2 集群
- **目标集群**: member2（单集群）
- **资源选择器**: `rbac.authorization.k8s.io/v1` ClusterRoleBinding，名称 `vcjob-logger`
- **冲突处理**: Abort（遇到冲突中止）
- **用途**: 在 member2 集群绑定 vcjob-logger ClusterRole，使 ServiceAccount 具有日志查看权限
- **为什么必须用 ClusterPropagationPolicy**:
  - ClusterRoleBinding 是集群级 RBAC 资源
- **与 ClusterRole 的关系**:
  - ClusterRole 定义权限（能做什么）
  - ClusterRoleBinding 绑定权限（谁可以使用）
  - 两个资源都需要传播到成员集群才能生效

#### Queue 传播策略

**volcano-global-all-queue-propagation**
- **作用**: 将所有 Volcano Queue 传播到所有成员集群
- **目标集群**: 所有可用集群（未指定 clusterAffinity）
- **资源选择器**: `scheduling.volcano.sh/v1beta1` Queue（所有队列）
- **副本调度**: Duplicated（完整副本）
- **冲突处理**: Overwrite（覆盖已有资源）
- **特殊标签**: `resourcetemplate.karmada.io/deletion-protected: Always`（防止误删除）
- **用途**: 确保 Volcano 队列配置在所有集群同步，统一调度策略
- **为什么必须用 ClusterPropagationPolicy**:
  - Queue 是集群级资源
  - 需要同步所有队列到所有成员集群
- **实际效果**: 在 Karmada 控制平面创建任何 Queue（如 large-task-shared-queue），自动传播到 member1、member2 两个集群，所有集群使用相同的队列配置和资源配额

### 3. OverridePolicy（覆盖策略）

按集群定制资源配置（目前未使用）：
- 不同集群的资源限制
- 集群特定的环境变量
- 集群特定的副本数

### PropagationPolicy vs ClusterPropagationPolicy 对比

**核心区别：**

| 特性 | PropagationPolicy | ClusterPropagationPolicy |
|------|-------------------|--------------------------|
| **作用范围** | 命名空间级（namespace-scoped） | 集群级（cluster-scoped） |
| **metadata.namespace** | ✓ 必须指定 | ✗ 不存在（全局资源） |
| **资源选择范围** | 同命名空间的资源 | 所有命名空间 + 集群级资源 |
| **适用资源类型** | Pod、Job、Service、ConfigMap | Namespace、ClusterRole、Queue |
| **团队隔离** | ✓ 命名空间隔离 | ✗ 全局影响 |
| **推荐场景** | 应用/工作负载传播 | 基础设施/全局配置 |

**必须使用 ClusterPropagationPolicy 的场景：**

1. **集群级资源（无 namespace）**
   - Namespace（命名空间本身）
   - ClusterRole（集群角色）
   - ClusterRoleBinding（集群角色绑定）
   - Queue（Volcano 队列）
   - PersistentVolume（持久卷）
   - CustomResourceDefinition（自定义资源定义）

2. **跨命名空间选择资源**
   - 需要选择所有命名空间的资源
   - 全局配置同步

3. **基础设施传播**
   - 系统级 RBAC 配置
   - 全局队列配置
   - 基础命名空间创建

**推荐使用 PropagationPolicy 的场景：**

1. **应用和工作负载**
   - Pod、Deployment、Job、Service
   - ConfigMap、Secret（命名空间内）

2. **团队/项目隔离**
   - 多租户场景
   - 不同命名空间独立策略

**错误示例：**

```yaml
# ❌ 错误：用 PropagationPolicy 传播 Namespace
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy  # 错误！
metadata:
  name: namespace-policy
  namespace: argo
spec:
  resourceSelectors:
  - apiVersion: v1
    kind: Namespace  # 集群级资源，PropagationPolicy 无法选择
    name: argo
```

结果：Karmada 拒绝创建，报错 "PropagationPolicy cannot select cluster-scoped resources"

**正确示例：**

```yaml
# ✓ 正确：用 ClusterPropagationPolicy 传播 Namespace
apiVersion: policy.karmada.io/v1alpha1
kind: ClusterPropagationPolicy  # 正确！
metadata:
  name: argo-namespace-propagation
  # 无 namespace 字段（全局资源）
spec:
  resourceSelectors:
  - apiVersion: v1
    kind: Namespace  # 集群级资源
    name: argo
  placement:
    clusterAffinity:
      clusterNames:
      - member1
      - member2
```

**传播流程：**

```
┌─────────────────┐
│ Karmada 控制平面 │
│  ClusterPropaga-│
│  tionPolicy     │
└─────────────────┘
       ↓ 自动匹配并传播
┌──────────────┬──────────────┐
│ member1      │ member2      │
│ 集群级资源    │ 集群级资源    │
└──────────────┴──────────────┘
```

### 4. Queue（Volcano 队列）

控制工作负载调度和资源分配：
- **default**: 默认队列（weight: 1）
- **large-task-shared-queue**: 大任务队列（196 CPU, 1500Gi 内存, priority: 100）
- **shared-flexible-queue**: 灵活队列（32 CPU, 56Gi 内存, priority: 50）
- **user1-cpu-queue**: 用户专属队列（72 CPU, 132Gi 内存）
- **test**: 测试队列

**队列参数：**
- `capability`: 资源上限（CPU、memory）
- `priority`: 队列优先级（数值越大优先级越高）
- `weight`: 公平共享权重
- `reclaimable`: 是否允许资源回收

### 5. RBAC（权限配置）

工作流执行所需的权限：
- **vcjob-logger**: 日志查看权限
- **vcjob-submitter**: Volcano Job 提交权限
- **vcjob-proxy**: 集群代理访问权限
- **vcjob-unified**: 统一权限配置
- **queue-adjuster-role**: 队列调整权限

## 快速开始

### 导出配置

从集群导出当前配置（自动清理运行时字段）：

```bash
cd configs
./export.sh
```

导出的配置会自动删除运行时字段（creationTimestamp、resourceVersion、uid等）。

### 应用配置

将配置应用到 Karmada 控制平面：

```bash
cd configs
./apply.sh
```

**注意：**
- 配置应用于 **Karmada 控制平面**（kubeconfig: `/root/.kube/karmada-proxy.config`）
- Karmada 会自动将资源传播到成员集群
- 成员集群（member1、member2）不需要这些 Karmada 策略资源

### 清理配置

如果需要重新清理运行时字段：

```bash
cd configs
python3 clean.sh                 # 清理所有配置
python3 clean.sh queues/         # 清理特定目录
```

### 手动操作

```bash
# 应用所有传播策略
kubectl apply -f propagation-policies/

# 应用特定策略
kubectl apply -f propagation-policies/argo-argo-vcjob-policy.yaml

# 应用所有队列
kubectl apply -f queues/

# 应用 RBAC
kubectl apply -f rbac/
```

## 脚本说明

### apply.sh

将配置应用到 Karmada 集群。

**特性：**
- 先用 `--dry-run` 验证配置
- 逐个应用文件，便于错误追踪
- 显示应用进度和状态

**使用：**
```bash
./apply.sh
```

### export.sh

从集群导出配置并自动清理。

**特性：**
- 导出每个资源到单独文件
- 按类型组织（propagation-policies、queues、rbac）
- 自动创建目录
- 自动清理运行时字段

**使用：**
```bash
./export.sh
```

### clean.sh

清理运行时生成的字段，使配置可以安全地用于 `kubectl apply`。

**删除的字段：**
- 元数据：creationTimestamp、resourceVersion、uid、generation、finalizers、managedFields
- 标签：propagationpolicy.karmada.io/permanent-id、clusterpropagationpolicy.karmada.io/permanent-id
- 注解：kubectl.kubernetes.io/last-applied-configuration、clusterpropagationpolicy.karmada.io/name
- status 部分（不应被应用）

**保留的配置：**
- namespace.spec.finalizers（配置的一部分）
- RBAC 规则中的 */finalizers 资源（API 资源名称）

**使用：**
```bash
python3 clean.sh                  # 清理所有配置
python3 clean.sh queues/          # 清理指定目录
python3 clean.sh rbac/vcjob-clusterroles.yaml  # 清理指定文件
```

## 配置示例

### PropagationPolicy 示例

```yaml
apiVersion: policy.karmada.io/v1alpha1
kind: PropagationPolicy
metadata:
  name: argo-vcjob-policy
  namespace: argo
spec:
  conflictResolution: Overwrite
  placement:
    clusterAffinity:
      clusterNames:
      - member2
    spreadConstraints:
    - maxGroups: 1
      minGroups: 1
      spreadByField: cluster
  resourceSelectors:
  - apiVersion: batch.volcano.sh/v1alpha1
    kind: Job
    namespace: argo
```

### Queue 示例

```yaml
apiVersion: scheduling.volcano.sh/v1beta1
kind: Queue
metadata:
  name: large-task-shared-queue
spec:
  capability:
    cpu: '196'
    memory: 1500Gi
  priority: 100
  reclaimable: true
  weight: 10
```

## 验证配置

应用配置后验证：

```bash
# 检查传播策略
kubectl get propagationpolicies --all-namespaces

# 检查集群传播策略
kubectl get clusterpropagationpolicies

# 检查队列
kubectl get queues

# 检查队列详情
kubectl get queue large-task-shared-queue -o yaml
```

## 常见问题

### 配置应用失败

1. **检查 kubeconfig**：确保 KUBECONFIG 路径正确
2. **预验证**：使用 `kubectl apply --dry-run=client -f <file>`
3. **检查冲突**：查看是否存在同名策略
4. **验证命名空间**：确保目标命名空间存在

### PropagationPolicy 未生效

- **原因**：资源选择器不匹配
- **解决**：检查 `resourceSelectors` 是否匹配资源的标签

### Queue 未在成员集群创建

- **原因**：缺少 ClusterPropagationPolicy
- **解决**：确保 Queue 有对应的集群传播策略（volcano-global-all-queue-propagation）

### RBAC 权限被拒绝

- **原因**：ServiceAccount 未传播到成员集群
- **解决**：为 ServiceAccount 创建 ClusterPropagationPolicy

## 开发流程

1. **修改配置**：编辑 YAML 文件
2. **本地验证**：`kubectl apply --dry-run=client -f <file>`
3. **应用到集群**：`./apply.sh`
4. **验证结果**：检查资源是否正确创建/更新
5. **提交变更**：`git add && git commit`

## 集群信息

**当前 Karmada 集群：**
- `member1`: 第一个成员集群
- `member2`: 第二个成员集群

**Kubeconfig 路径：** `/root/.kube/karmada-proxy.config`

## 相关文档

- Karmada PropagationPolicy: https://karmada.io/docs/userguide/resource-propagation-policy
- Volcano Queue: https://volcano.sh/en/docs/queue/
- Karmada OverridePolicy: https://karmada.io/docs/userguide/override-policy