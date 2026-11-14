// Minimal VS Code companion. Uses the platform API; no bundled dependencies.
const vscode = require("vscode");
const fs = require("node:fs/promises");
const os = require("node:os");
const path = require("node:path");
const { parseFiles, applyPatchFor } = require("./patch");

function settings() {
  const cfg = vscode.workspace.getConfiguration("aip");
  const apiUrl = cfg.get("apiUrl", "").replace(/\/$/, "");
  const orgId = cfg.get("orgId", "");
  const token = cfg.get("token", "");
  const projectId = cfg.get("projectId", "");
  if (!apiUrl || !orgId || !token) {
    throw new Error("set aip.apiUrl, aip.orgId and aip.token in Settings");
  }
  return { apiUrl, orgId, token, projectId };
}

async function request(method, path, body) {
  const { apiUrl, token } = settings();
  const res = await fetch(apiUrl + path, {
    method,
    headers: { Authorization: "Bearer " + token, ...(body ? { "Content-Type": "application/json" } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  });
  const text = await res.text();
  if (!res.ok) throw new Error("API " + res.status + ": " + text.slice(0, 300));
  return text ? JSON.parse(text) : {};
}

async function pickProject() {
  const { projectId } = settings();
  if (projectId) return projectId;
  const value = await vscode.window.showInputBox({ prompt: "Project id" });
  if (!value) throw new Error("project id is required");
  return value;
}

async function listTasks() {
  const project = await pickProject();
  const { orgId } = settings();
  const result = await request("GET", `/api/v1/orgs/${orgId}/projects/${project}/tasks`);
  const items = (result.data || []).map((task) => ({ label: task.ref + " — " + task.title, description: task.status, task }));
  const picked = await vscode.window.showQuickPick(items, { placeHolder: "Tasks" });
  if (picked) vscode.window.showInformationMessage(picked.label);
}

async function startRun() {
  const project = await pickProject();
  const { orgId } = settings();
  const task = await vscode.window.showInputBox({ prompt: "Task ref (T-12)" });
  if (!task) return;
  const agent = await vscode.window.showQuickPick(["development", "planning", "review"], { placeHolder: "Agent type" });
  if (!agent) return;
  const run = await request("POST", `/api/v1/orgs/${orgId}/projects/${project}/tasks/${task}/runs`, { agent_type: agent });
  vscode.window.showInformationMessage(`Run ${run.id} ${run.status}`);
}

async function showRun() {
  const { orgId } = settings();
  const id = await vscode.window.showInputBox({ prompt: "Run id" });
  if (!id) return;
  const run = await request("GET", `/api/v1/orgs/${orgId}/runs/${id}`);
  const doc = await vscode.workspace.openTextDocument({ content: JSON.stringify(run, null, 2), language: "json" });
  await vscode.window.showTextDocument(doc);
}

async function approve() {
  const { orgId } = settings();
  const id = await vscode.window.showInputBox({ prompt: "Approval id" });
  if (!id) return;
  const decision = await vscode.window.showQuickPick(["approved", "rejected"], { placeHolder: "Decision" });
  if (!decision) return;
  const reason = await vscode.window.showInputBox({ prompt: "Reason (required for manual evidence/rejections)", value: "" });
  await request("POST", `/api/v1/orgs/${orgId}/approvals/${id}/decision`, { decision, reason: reason || "" });
  vscode.window.showInformationMessage("Decision recorded");
}

async function showEvidence() {
  const { orgId } = settings();
  const id = await vscode.window.showInputBox({ prompt: "Run id" });
  if (!id) return;
  const run = await request("GET", `/api/v1/orgs/${orgId}/runs/${id}`);
  const checks = await request("GET", `/api/v1/orgs/${orgId}/runs/${id}/checks`);
  const lines = [
    `# Run ${run.id}`,
    "",
    `- status: **${run.status}**`,
    `- agent: ${run.agent_type || "unknown"}`,
    run.failure_code ? `- failure: \`${run.failure_code}\` ${run.failure_detail || ""}` : "- failure: none",
    "",
    "## Checks",
  ];
  for (const check of checks.data || []) {
    lines.push(`- **${check.name}**: ${check.status}`);
    if (check.output) lines.push("  ```\n" + String(check.output).split("\n").slice(0, 20).join("\n") + "\n  ```");
  }
  const doc = await vscode.workspace.openTextDocument({ content: lines.join("\n"), language: "markdown" });
  await vscode.window.showTextDocument(doc, { preview: false });
}

async function showDiff() {
  const { orgId } = settings();
  const id = await vscode.window.showInputBox({ prompt: "Run id" });
  if (!id) return;
  const run = await request("GET", `/api/v1/orgs/${orgId}/runs/${id}`);
  if (!run.diff) {
    vscode.window.showInformationMessage("This run has no diff.");
    return;
  }
  const folder = vscode.workspace.workspaceFolders && vscode.workspace.workspaceFolders[0];
  try {
    if (!folder) throw new Error("open the repository folder to see a side-by-side diff");
    const files = parseFiles(run.diff);
    if (files.length === 0) throw new Error("patch has no files");
    let selected = files[0].path;
    if (files.length > 1) {
      const picked = await vscode.window.showQuickPick(files.map((f) => f.path), { placeHolder: `Changed files (${files.length})` });
      if (!picked) return;
      selected = picked;
    }
    const base = await fs.readFile(path.join(folder.uri.fsPath, selected), "utf8").catch(() => "");
    const applied = applyPatchFor(base, run.diff, selected);
    const baseUri = vscode.Uri.file(path.join(folder.uri.fsPath, applied.file));
    const temp = path.join(os.tmpdir(), "aip-" + id + "-" + path.basename(applied.file));
    await fs.writeFile(temp, applied.content, "utf8");
    await vscode.commands.executeCommand("vscode.diff", baseUri, vscode.Uri.file(temp), `AIP ${id}: ${applied.file}`);
  } catch (err) {
    const doc = await vscode.workspace.openTextDocument({ content: run.diff, language: "diff" });
    await vscode.window.showTextDocument(doc, { preview: false });
    vscode.window.showWarningMessage("Side-by-side unavailable: " + String(err.message || err) + " (opened the patch)");
  }
}


function activate(context) {
  const wrap = (fn) => async () => {
    try {
      await fn();
    } catch (err) {
      vscode.window.showErrorMessage(String(err.message || err));
    }
  };
  context.subscriptions.push(
    vscode.commands.registerCommand("aip.listTasks", wrap(listTasks)),
    vscode.commands.registerCommand("aip.startRun", wrap(startRun)),
    vscode.commands.registerCommand("aip.showRun", wrap(showRun)),
    vscode.commands.registerCommand("aip.approve", wrap(approve)),
    vscode.commands.registerCommand("aip.showEvidence", wrap(showEvidence)),
    vscode.commands.registerCommand("aip.showDiff", wrap(showDiff)),
    vscode.commands.registerCommand("aip.watchRun", wrap(watchRun)),
  );
}

function deactivate() {}

module.exports = { activate, deactivate };
