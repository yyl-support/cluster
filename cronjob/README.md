# cronjob

Karmada 控制面定时任务，用于清理和修复卡住的资源。

## 脚本

### `force-patch-stuck-rb/patch.py`

扫描所有 `ResourceBinding`，找出 `Scheduled=False` 且 `reason=SchedulerError` 且持续超过 `THRESHOLD` 秒的，直接 patch 指定集群使其强制调度。

### `vcjob-ttl-cleaner/clean.py`

扫描所有 Volcano Job，删除处于终态（`Completed`/`Failed`/`Aborted`/`Terminated`）超过 `TTL_MINUTES` 分钟的 Job。支持 `DRY_RUN`。

## 环境变量

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `KUBECONFIG` | `/etc/karmada/karmada.config` | Karmada kubeconfig 路径 |
| `THRESHOLD` | `900` | (patch) 卡住阈值，秒 |
| `DEFAULT_CLUSTER` | `001` | (patch) 默认调度的目标集群 |
| `TTL_MINUTES` | `2` | (clean) 终态 Job 保留时间，分钟 |
| `DRY_RUN` | `false` | (clean) 仅打印不删除 |

## 构建

```bash
docker build -t cronjob .
```
