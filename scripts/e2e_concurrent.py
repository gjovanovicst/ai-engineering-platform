"""Create N tasks and drive them concurrently to merged PRs through the API."""
import json
import os
import sys
import time
import urllib.error
import urllib.request

BASE = os.environ["API_BASE"]
ORG = os.environ["ORG"]
PROJECT = os.environ["PROJECT"]
REPO = os.environ["REPO"]
TOKEN = os.environ["TOKEN"]
N = int(os.environ.get("N", "10"))


def api(method, path, body=None, headers=None):
    data = json.dumps(body).encode() if body is not None else None
    hdrs = {"Content-Type": "application/json", "Authorization": f"Bearer {TOKEN}"}
    hdrs.update(headers or {})
    req = urllib.request.Request(BASE + path, data=data, method=method, headers=hdrs)
    try:
        with urllib.request.urlopen(req, timeout=20) as resp:
            return resp.status, json.loads(resp.read() or b"{}")
    except urllib.error.HTTPError as exc:
        return exc.code, json.loads(exc.read() or b"{}")


def wait(predicate, seconds=180, label="condition"):
    deadline = time.time() + seconds
    last = None
    while time.time() < deadline:
        last = predicate()
        if last:
            return last
        time.sleep(1)
    raise SystemExit(f"timeout waiting for {label}: {last}")


# 1. tasks ready + planning runs
tasks = []
for i in range(N):
    status, task = api("POST", f"/api/v1/orgs/{ORG}/projects/{PROJECT}/tasks", {
        "title": f"concurrent {i}", "acceptance": [{"given": "g", "when": "w", "then": "t"}],
    })
    if status != 201:
        raise SystemExit(f"create task {i}: {status} {task}")
    ref = task["ref"]
    status, body = api("POST", f"/api/v1/orgs/{ORG}/projects/{PROJECT}/tasks/{ref}/transitions", {"to": "ready"})
    if status != 200:
        raise SystemExit(f"ready {ref}: {status} {body}")
    run = None
    for _ in range(120):
        status, body = api("POST", f"/api/v1/orgs/{ORG}/projects/{PROJECT}/tasks/{ref}/runs", {"agent_type": "planning"})
        if status in (200, 201, 202):
            run = body
            break
        if status == 429:
            # free a slot by deciding any pending plan approval, then retry
            _, data = api("GET", f"/api/v1/orgs/{ORG}/approvals")
            for approval in [a for a in data.get("data", []) if a.get("type") == "plan" and a.get("status") == "pending"]:
                api("POST", f"/api/v1/orgs/{ORG}/approvals/{approval['id']}/decision", {"decision": "approved", "reason": "concurrent e2e"})
            time.sleep(2)
            continue
        raise SystemExit(f"planning {ref}: {status} {body}")
    if run is None:
        raise SystemExit(f"planning {ref}: no slot")
    tasks.append({"id": task["id"], "ref": ref, "plan_run": run["id"], "dev_run": None, "pr": None})
print(f"started {N} tasks")

# 2. approve plan approvals as they appear; development runs follow automatically
def approvals():
    return api("GET", f"/api/v1/orgs/{ORG}/approvals")[1].get("data", [])

def prs():
    _, data = api("GET", f"/api/v1/orgs/{ORG}/projects/{PROJECT}/pull-requests")
    return data.get("data", [])

def drive_plans():
    pending = [a for a in approvals() if a.get("type") == "plan" and a.get("status") == "pending"]
    for approval in pending:
        api("POST", f"/api/v1/orgs/{ORG}/approvals/{approval['id']}/decision", {"decision": "approved", "reason": "concurrent e2e"})
    return len(prs()) >= N

wait(drive_plans, seconds=int(os.environ.get("WAIT_PR", "900")), label="pull requests")

# 3. wait for all PRs
def prs():
    _, data = api("GET", f"/api/v1/orgs/{ORG}/projects/{PROJECT}/pull-requests")
    return data.get("data", [])

by_task = {pr["task_id"]: pr for pr in prs()}
for task in tasks:
    if task["id"] not in by_task:
        raise SystemExit(f"no PR for {task['ref']}")
    task["pr"] = by_task[task["id"]]
numbers = [t["pr"]["number"] for t in tasks]
if len(set(numbers)) != N:
    raise SystemExit(f"duplicate PR numbers: {numbers}")
print(f"pull requests: {numbers}")

# 4. wait for all PR approvals, then approve every pending one
wait(lambda: len([a for a in approvals() if a.get("type") == "pr"]) >= N, label="PR approvals")
for approval in [a for a in approvals() if a.get("type") == "pr" and a.get("status") == "pending"]:
    api("POST", f"/api/v1/orgs/{ORG}/approvals/{approval['id']}/decision", {"decision": "approved", "reason": "concurrent e2e"})
wait(lambda: not [a for a in approvals() if a.get("type") == "pr" and a.get("status") == "pending"], label="PR decisions")

# 5. merge every PR via the GitLab webhook
for task in tasks:
    body = {
        "object_kind": "merge_request",
        "project": {"path_with_namespace": "group/project"},
        "object_attributes": {"iid": task["pr"]["number"], "action": "merge", "state": "merged"},
    }
    status, out = api("POST", f"/webhooks/gitlab/{REPO}", body, {"X-Gitlab-Token": "hook-secret"})
    if status not in (200, 202):
        raise SystemExit(f"merge {task['pr']['number']}: {status} {out}")

def all_merged():
    merged = [p for p in prs() if p["state"] == "merged"]
    return len(merged) >= N

wait(all_merged, seconds=900, label="merged PRs")
print("merged", N, "PRs; distinct:", len(set(numbers)))
