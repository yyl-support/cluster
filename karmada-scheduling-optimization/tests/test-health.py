#!/usr/bin/env python3

import json
import subprocess
import tempfile
from pathlib import Path

BASE = Path(__file__).resolve().parents[1]


def write(path, name, value):
    (path / name).write_text(json.dumps(value), encoding="utf-8")


with tempfile.TemporaryDirectory() as directory:
    root = Path(directory)
    ready_condition = [{"type": "Ready", "status": "True"}]
    write(root, "clusters.json", {"items": [{"status": {"conditions": ready_condition}}]})
    write(root, "pods-001.json", {"items": [{"spec": {}, "status": {"phase": "Pending"}}, {"spec": {"nodeName": "n1"}, "status": {"phase": "Pending"}}]})
    write(root, "pods-wlcb.json", {"items": []})
    write(root, "nodes-001.json", {"items": [{"status": {"conditions": ready_condition}}]})
    write(root, "nodes-wlcb.json", {"items": []})
    write(root, "queue-001.json", {"status": {"state": "Open"}})
    write(root, "queue-wlcb.json", {"status": {"state": "Open"}})
    result = subprocess.run(["python3", str(BASE / "lib" / "health.py"), str(root)], check=True, capture_output=True, text=True)
    data = json.loads(result.stdout)
    assert data["clusters"]["all_ready"] is True
    assert data["cluster001"]["unbound_pending"] == 1
    assert data["cluster001"]["bound_pending"] == 1

print("health.py tests passed")
