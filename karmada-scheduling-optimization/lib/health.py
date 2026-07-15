#!/usr/bin/env python3

import json
import os
import sys
from datetime import datetime, timezone


def load(directory, name):
    with open(os.path.join(directory, name), encoding="utf-8") as handle:
        return json.load(handle)


def pod_summary(document):
    phases = {}
    unbound_pending = 0
    bound_pending = 0
    for pod in document.get("items", []):
        phase = pod.get("status", {}).get("phase", "Unknown")
        phases[phase] = phases.get(phase, 0) + 1
        if phase == "Pending":
            if pod.get("spec", {}).get("nodeName"):
                bound_pending += 1
            else:
                unbound_pending += 1
    return {"phases": phases, "unbound_pending": unbound_pending, "bound_pending": bound_pending}


def node_summary(document):
    ready = sum(any(condition.get("type") == "Ready" and condition.get("status") == "True" for condition in node.get("status", {}).get("conditions", [])) for node in document.get("items", []))
    return {"ready_nodes": ready, "total_nodes": len(document.get("items", []))}


directory = sys.argv[1]
clusters = load(directory, "clusters.json")
cluster_items = clusters.get("items", [])
all_ready = bool(cluster_items) and all(any(condition.get("type") == "Ready" and condition.get("status") == "True" for condition in item.get("status", {}).get("conditions", [])) for item in cluster_items)
queue_001 = load(directory, "queue-001.json")
queue_wlcb = load(directory, "queue-wlcb.json")
result = {
    "captured_at": datetime.now(timezone.utc).isoformat(),
    "clusters": {"all_ready": all_ready, "count": len(cluster_items)},
    "cluster001": {**pod_summary(load(directory, "pods-001.json")), **node_summary(load(directory, "nodes-001.json")), "queue_open": queue_001.get("status", {}).get("state") == "Open"},
    "clusterwlcb": {**pod_summary(load(directory, "pods-wlcb.json")), **node_summary(load(directory, "nodes-wlcb.json")), "queue_open": queue_wlcb.get("status", {}).get("state") == "Open"},
}
json.dump(result, sys.stdout, ensure_ascii=False, indent=2)
sys.stdout.write("\n")
