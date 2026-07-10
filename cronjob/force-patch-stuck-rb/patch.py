#!/usr/bin/env python3
import base64, json, os, re, ssl, tempfile, shutil, urllib.request, urllib.error
from datetime import datetime, timezone

# 从环境变量读取配置
KUBECONFIG_PATH = os.environ.get("KUBECONFIG", "/etc/karmada/karmada.config")
THRESHOLD = int(os.environ.get("THRESHOLD", "900"))         # 卡住阈值（秒）
DEFAULT_CLUSTER = os.environ.get("DEFAULT_CLUSTER", "001")  # 默认调度集群


def load_kubeconfig(path):
    """解析 kubeconfig，提取 CA、客户端证书、私钥，构造 SSL 上下文"""
    raw = open(path).read()
    ca_b64 = re.search(r"certificate-authority-data:\s+(\S+)", raw).group(1)
    cert_b64 = re.search(r"client-certificate-data:\s+(\S+)", raw).group(1)
    key_b64 = re.search(r"client-key-data:\s+(\S+)", raw).group(1)
    server = re.search(r"server:\s+(\S+)", raw).group(1)

    # 将证书写入临时文件供 SSL 上下文加载
    td = tempfile.mkdtemp()
    ca = td + "/ca.crt"
    open(ca, "wb").write(base64.b64decode(ca_b64))
    cert = td + "/cl.crt"
    open(cert, "wb").write(base64.b64decode(cert_b64))
    key = td + "/cl.key"
    open(key, "wb").write(base64.b64decode(key_b64))

    ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
    ctx.load_verify_locations(ca)
    ctx.load_cert_chain(cert, key)
    return server, ctx, td


def api(server, ctx, method, path, body=None):
    """通用的 Karmada API 请求封装，返回 (响应体, HTTP 状态码)"""
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(server + path, data=data, method=method)
    if data:
        req.add_header("Content-Type", "application/merge-patch+json")
    try:
        resp = urllib.request.urlopen(req, context=ctx, timeout=15)
        return json.loads(resp.read()), resp.status
    except urllib.error.HTTPError as e:
        return None, e.code


def list_rbs(server, ctx):
    """分页获取所有 ResourceBinding"""
    items, url = [], "/apis/work.karmada.io/v1alpha2/resourcebindings"
    while url:
        d, code = api(server, ctx, "GET", url)
        if not d:
            break
        items.extend(d.get("items", []))
        cont = d.get("metadata", {}).get("continue", "")
        url = f"/apis/work.karmada.io/v1alpha2/resourcebindings?continue={cont}" if cont else None
    return items


server, ctx, td = load_kubeconfig(KUBECONFIG_PATH)
try:
    now = datetime.now(timezone.utc).timestamp()

    # 统计变量
    patch_count = 0        # 成功 patch 的 RB 数
    skip_suspended = 0     # 被调度挂起的 RB 数
    waiting_count = 0      # 卡住但未超阈值的 RB 数
    conflict_count = 0     # 冲突（409）的 RB 数
    no_ascend_count = 0    # 无 ascend 标签直接写入的 RB 数
    total_scanned = 0

    all_rbs = list_rbs(server, ctx)
    total_scanned = len(all_rbs)

    for item in all_rbs:
        # 跳过已有调度结果的 RB
        if item["spec"].get("clusters"):
            continue

        # 检查 label：如果没有 huawei.com/ascend 开头的标签，直接强制写入调度结果
        labels = item.get("metadata", {}).get("labels", {}) or {}
        has_ascend = any(k.startswith("huawei.com/ascend") for k in labels)
        if not has_ascend:
            # 无 ascend 标签的 RB 同样需要跳过挂起状态，避免与调度器竞争
            suspension = item["spec"].get("suspension", {})
            if suspension.get("scheduling"):
                skip_suspended += 1
                continue
            ns = item["metadata"]["namespace"]
            name = item["metadata"]["name"]
            cluster_names = (item["spec"]
                             .get("placement", {})
                             .get("clusterAffinity", {})
                             .get("clusterNames", []))
            cluster = cluster_names[0] if len(cluster_names) == 1 else DEFAULT_CLUSTER
            replicas = item["spec"].get("replicas", 1)
            patch_path = (f"/apis/work.karmada.io/v1alpha2/namespaces/{ns}"
                          f"/resourcebindings/{name}")
            _, code = api(server, ctx, "PATCH", patch_path,
                          {"spec": {"clusters": [{"name": cluster, "replicas": replicas}]}})
            if code and 200 <= code < 300:
                print(f"[OK-NO-ASCEND] {ns}/{name} -> {cluster}", flush=True)
                patch_count += 1
            elif code == 409:
                conflict_count += 1
            else:
                print(f"[FAIL] {ns}/{name} (HTTP {code})", flush=True)
            no_ascend_count += 1
            continue

        # 跳过被 Volcano Global 调度器挂起的 RB
        suspension = item["spec"].get("suspension", {})
        if suspension.get("scheduling"):
            skip_suspended += 1
            continue

        # 找到 Scheduled=False + SchedulerError 的条件
        conditions = item.get("status", {}).get("conditions", [])
        sched = next((c for c in conditions
                      if c.get("type") == "Scheduled"
                      and c.get("status") == "False"
                      and c.get("reason") == "SchedulerError"), None)
        if not sched:
            continue

        # 计算卡住时长
        ts = sched.get("lastTransitionTime", "").replace("Z", "+00:00")
        try:
            age = now - datetime.fromisoformat(ts).timestamp()
        except Exception:
            continue
        if age < THRESHOLD:
            waiting_count += 1
            continue

        # 确定目标集群：placement 只指定了一个 clusterName 则用该集群，否则用默认集群
        cluster_names = (item["spec"]
                         .get("placement", {})
                         .get("clusterAffinity", {})
                         .get("clusterNames", []))
        cluster = cluster_names[0] if len(cluster_names) == 1 else DEFAULT_CLUSTER

        ns = item["metadata"]["namespace"]
        name = item["metadata"]["name"]
        replicas = item["spec"].get("replicas", 1)
        age_min = int(age / 60)

        # 发起 merge-patch，强制写入调度结果
        patch_path = (f"/apis/work.karmada.io/v1alpha2/namespaces/{ns}"
                      f"/resourcebindings/{name}")
        _, code = api(server, ctx, "PATCH", patch_path,
                      {"spec": {"clusters": [{"name": cluster, "replicas": replicas}]}})
        if code and 200 <= code < 300:
            print(f"[OK] {ns}/{name} -> {cluster} (stuck {age_min}min)", flush=True)
            patch_count += 1
        elif code == 409:
            conflict_count += 1
        else:
            print(f"[FAIL] {ns}/{name} (HTTP {code})", flush=True)

    print(f"Scanned={total_scanned}, Patched={patch_count}, "
          f"Waiting(<{THRESHOLD}s)={waiting_count}, "
          f"Suspended={skip_suspended}, Conflict={conflict_count}, "
          f"NoAscend={no_ascend_count}", flush=True)
finally:
    shutil.rmtree(td, ignore_errors=True)
