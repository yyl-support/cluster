#!/usr/bin/env python3
"""
扫描 Karmada 控制面中的所有 Volcano Job，
删除处于终态（Completed / Failed / Aborted / Terminated）超过 TTL_MINUTES 分钟的 Job。
Summary 行会输出 ResourceBinding 调度统计和集群 NPU 资源摘要。
"""
import base64
import json
import os
import re
import ssl
import sys
import shutil
import tempfile
import urllib.request, urllib.error
from datetime import datetime, timezone, timedelta

# 终态集合
TERMINAL_PHASES = {"Completed", "Failed", "Aborted", "Terminated"}
TTL_MINUTES = int(os.environ.get("TTL_MINUTES", "2"))               # 删除前的保留时间（分钟）
TTL = timedelta(minutes=TTL_MINUTES)
KUBECONFIG_PATH = os.environ.get("KUBECONFIG", "/etc/karmada/karmada.config")
DRY_RUN = os.environ.get("DRY_RUN", "false").lower() == "true"     # 仅打印不删除


def load_kubeconfig(path):
    """解析 kubeconfig，构造双向 TLS 认证的 SSL 上下文"""
    raw = open(path).read()
    ca_b64   = re.search(r"certificate-authority-data:\s+(\S+)", raw).group(1)
    cert_b64 = re.search(r"client-certificate-data:\s+(\S+)", raw).group(1)
    key_b64  = re.search(r"client-key-data:\s+(\S+)", raw).group(1)
    server   = re.search(r"server:\s+(\S+)", raw).group(1)

    td = tempfile.mkdtemp()
    ca_path   = td + "/ca.crt"
    cert_path = td + "/client.crt"
    key_path  = td + "/client.key"
    open(ca_path,   "wb").write(base64.b64decode(ca_b64))
    open(cert_path, "wb").write(base64.b64decode(cert_b64))
    open(key_path,  "wb").write(base64.b64decode(key_b64))

    ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
    ctx.load_verify_locations(ca_path)
    ctx.load_cert_chain(cert_path, key_path)
    return server, ctx, td


def api_request(server, ctx, method, path, body=None):
    """通用的 Karmada API 请求封装，返回反序列化后的 JSON 响应"""
    url = server + path
    data = json.dumps(body).encode() if body else None
    req = urllib.request.Request(url, data=data, method=method)
    if data:
        req.add_header("Content-Type", "application/json")
    resp = urllib.request.urlopen(req, context=ctx, timeout=10)
    return json.loads(resp.read())


def list_vcjobs(server, ctx):
    """分页获取所有 Volcano Job"""
    items = []
    url = "/apis/batch.volcano.sh/v1alpha1/jobs"
    while url:
        data = api_request(server, ctx, "GET", url)
        items.extend(data.get("items", []))
        cont = data.get("metadata", {}).get("continue", "")
        url = f"/apis/batch.volcano.sh/v1alpha1/jobs?continue={cont}" if cont else None
    return items


def get_scheduling_stats(server, ctx):
    """统计 ResourceBinding 调度状态分布：已调度到 001 / wlcb / SchedulerError 未调度"""
    try:
        a001 = awlcb = serr = 0
        url = "/apis/work.karmada.io/v1alpha2/resourcebindings"
        while url:
            data = api_request(server, ctx, "GET", url)
            for item in data.get("items", []):
                clusters = [c["name"] for c in item.get("spec", {}).get("clusters", [])]
                conds = item.get("status", {}).get("conditions", [])
                sched = next((c for c in conds if c.get("type") == "Scheduled"), None)
                if "wlcb" in clusters:
                    awlcb += 1
                elif "001" in clusters:
                    a001 += 1
                elif sched and sched.get("reason") == "SchedulerError" and not clusters:
                    serr += 1
            cont = data.get("metadata", {}).get("continue", "")
            url = f"/apis/work.karmada.io/v1alpha2/resourcebindings?continue={cont}" if cont else None
        return a001, awlcb, serr
    except Exception:
        return None, None, None


def get_cluster_npu(server, ctx, cluster_name):
    """查询指定集群的 NPU 资源摘要，返回 (allocatable, allocated, allocating, free)"""
    try:
        data = api_request(server, ctx, "GET",
            f"/apis/cluster.karmada.io/v1alpha1/clusters/{cluster_name}")
        rs = data.get("status", {}).get("resourceSummary", {})
        able = rs.get("allocatable", {}).get("huawei.com/ascend-1980", "?")
        ed   = rs.get("allocated",   {}).get("huawei.com/ascend-1980", "?")
        ing  = rs.get("allocating",  {}).get("huawei.com/ascend-1980", "?")
        try:
            free = max(0, int(able) - int(ed) - int(ing))
        except Exception:
            free = "?"
        return able, ed, ing, free
    except Exception:
        return "?", "?", "?", "?"


def delete_vcjob(server, ctx, namespace, name):
    """删除指定 Volcano Job，404 视为已删除"""
    path = f"/apis/batch.volcano.sh/v1alpha1/namespaces/{namespace}/jobs/{name}"
    try:
        api_request(server, ctx, "DELETE", path)
        return True
    except urllib.error.HTTPError as e:
        if e.code == 404:
            return True  # 已被删除，视为成功
        print(f"ERROR deleting {namespace}/{name}: HTTP {e.code} {e.reason}", file=sys.stderr)
        return False
    except Exception as e:
        print(f"ERROR deleting {namespace}/{name}: {e}", file=sys.stderr)
        return False


def main():
    if DRY_RUN:
        print("DRY_RUN=true — 不会实际删除任何 Job")

    server, ctx, td = load_kubeconfig(KUBECONFIG_PATH)
    try:
        now = datetime.now(timezone.utc)
        jobs = list_vcjobs(server, ctx)
        deleted = 0
        kept = 0
        skipped = 0

        for job in jobs:
            meta  = job.get("metadata", {})
            ns    = meta.get("namespace", "")
            name  = meta.get("name", "")

            # 跳过正在删除中的 Job
            if meta.get("deletionTimestamp"):
                continue

            state = job.get("status", {}).get("state", {})
            phase = state.get("phase", "")

            # 仅处理终态 Job
            if phase not in TERMINAL_PHASES:
                skipped += 1
                continue

            last_t = state.get("lastTransitionTime", "")
            if not last_t:
                print(f"WARN  {ns}/{name}: phase={phase} but no lastTransitionTime, skip")
                skipped += 1
                continue

            # 计算进入终态后的时长
            finish_time = datetime.fromisoformat(last_t.replace("Z", "+00:00"))
            elapsed = now - finish_time

            if elapsed >= TTL:
                elapsed_min = elapsed.total_seconds() / 60
                print(f"DELETE {ns}/{name}  phase={phase}  elapsed={elapsed_min:.1f}min")
                if not DRY_RUN:
                    if delete_vcjob(server, ctx, ns, name):
                        deleted += 1
                else:
                    deleted += 1
            else:
                remaining = (TTL - elapsed).total_seconds() / 60
                elapsed_min = elapsed.total_seconds() / 60
                print(f"KEEP   {ns}/{name}  phase={phase}  elapsed={elapsed_min:.1f}min  (expires in {remaining:.1f}min)")
                kept += 1

        # ResourceBinding 调度统计
        a001, awlcb, serr = get_scheduling_stats(server, ctx)
        sched_str = ""
        if a001 is not None:
            sched_str = f"  | RB: 001={a001} wlcb={awlcb} SchedulerError={serr}"

        # wlcb 集群 NPU 资源摘要
        able, ed, ing, free = get_cluster_npu(server, ctx, "wlcb")
        npu_str = f"  | wlcb NPU: allocatable={able} allocated={ed} allocating={ing} free={free}"

        print(f"\nSummary: deleted={deleted}  kept={kept}  skipped(non-terminal)={skipped}  total={len(jobs)}{sched_str}{npu_str}")
    finally:
        shutil.rmtree(td, ignore_errors=True)


if __name__ == "__main__":
    main()
