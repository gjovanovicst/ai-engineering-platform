"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { EnvironmentsPanel } from "@/components/environments-panel";
import { PreviewsPanel } from "@/components/previews-panel";
import { ProjectMembersPanel } from "@/components/project-members-panel";
import { PullRequestsPanel } from "@/components/pull-requests-panel";
import { RepoPanel } from "@/components/repo-panel";
import { CommandForm } from "@/components/command-form";
import { TaskBoard } from "@/components/task-board";
import { TaskComposer } from "@/components/task-composer";
import { TaskDrawer } from "@/components/task-drawer";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { SidePanel } from "@/components/ui/side-panel";
import { Tabs } from "@/components/ui/tabs";
import {
  ApiError,
  ensureSession,
  getProject,
  getProjectPreflight,
  getProjectStandard,
  listComponentEntries,
  listDesignStandards,
  listProjectComponents,
  setProjectComponents,
  listTasks,
  setProjectStandard,
  startRun,
  setProjectE2E,
  setProjectTestCommand,
  setProjectVisual,
  setProjectMCP,
  setProjectPolicy,
  setProjectChangeWindow,
  setProjectAllowedModels,
  setProjectAllowedTools,
  setProjectRequiredApprovals,
  setProjectPluginChecks,
  getVisualBaseline,
  getMCPTools,
  scaffoldComponent,
  deleteProject,
  type MCPTool,
  protectProjectBranch,
  updateProjectRequiredChecks,
  type Task,
} from "@/lib/api";
import type { Claims } from "@/lib/auth";

const statusStyles: Record<string, string> = {
  draft: "bg-bg-hover text-text-muted",
  ready: "bg-status-warning/10 text-status-warning",
  planning: "bg-ai-subtle text-ai",
  in_progress: "bg-brand-subtle text-brand",
  review: "bg-status-warning/10 text-status-warning",
  done: "bg-status-success/10 text-status-success",
  blocked: "bg-status-danger/10 text-status-danger",
  canceled: "bg-bg-hover text-text-muted",
};

export function ProjectView({ projectId }: { projectId: string }) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [claims, setClaims] = useState<Claims | null | undefined>(undefined);
  const [showForm, setShowForm] = useState(false);
  const [editingTask, setEditingTask] = useState<Task | null>(null);
  const [formError, setFormError] = useState<string | null>(null);
  const [checks, setChecks] = useState("");
  const [checksError, setChecksError] = useState<string | null>(null);
  const [mcpConfig, setMCPConfig] = useState("");
  const [mcpError, setMCPError] = useState<string | null>(null);
  const [protection, setProtection] = useState<string | null>(null);
  const [baselineName, setBaselineName] = useState("");
  const [baselineError, setBaselineError] = useState<string | null>(null);
  const [mcpTools, setMCPTools] = useState<MCPTool[] | null>(null);
  const [mcpToolsError, setMCPToolsError] = useState<string | null>(null);
  const [mcpToolsLoading, setMCPToolsLoading] = useState(false);
  const [scaffoldName, setScaffoldName] = useState("");
  const [scaffoldKind, setScaffoldKind] = useState("react");
  const [scaffoldNote, setScaffoldNote] = useState<string | null>(null);
  const [deleteConfirm, setDeleteConfirm] = useState("");
  const [deleteError, setDeleteError] = useState<string | null>(null);
  const [policyMax, setPolicyMax] = useState("0");
  const [policyPaths, setPolicyPaths] = useState("");
  const [policyError, setPolicyError] = useState<string | null>(null);
  const [policyWindow, setPolicyWindow] = useState("");
  const [windowError, setWindowError] = useState<string | null>(null);
  const [allowedModels, setAllowedModels] = useState("");
  const [modelsError, setModelsError] = useState<string | null>(null);
  const [allowedTools, setAllowedTools] = useState("");
  const [toolsError, setToolsError] = useState<string | null>(null);
  const [requiredApprovals, setRequiredApprovals] = useState("1");
  const [approvalsError, setApprovalsError] = useState<string | null>(null);
  const [pluginChecks, setPluginChecks] = useState("[]");
  const [pluginError, setPluginError] = useState<string | null>(null);
  const [standardID, setStandardID] = useState("");
  const [standardError, setStandardError] = useState<string | null>(null);
  const [selectedComponents, setSelectedComponents] = useState<Record<string, { mode: "allow" | "exception"; reason: string }>>({});
  const [componentsError, setComponentsError] = useState<string | null>(null);
  const [selectedTask, setSelectedTask] = useState<Task | null>(null);
  const [tab, setTab] = useState<"overview" | "board" | "design" | "delivery">("overview");
  const [deliveryTab, setDeliveryTab] = useState<"checks" | "repos" | "environments" | "prs" | "access">("checks");

  useEffect(() => {
    ensureSession().then((result) => {
      if (!result) {
        router.replace("/login");
        return;
      }
      setClaims(result);
    });
  }, [router]);

  const saveChecks = useMutation({
    mutationFn: (names: string[]) => updateProjectRequiredChecks(claims!.org, projectId, names),
    onSuccess: () => {
      setChecksError(null);
      queryClient.invalidateQueries({ queryKey: ["project", claims?.org, projectId] });
    },
    onError: (err) => setChecksError(err instanceof ApiError ? err.message : "Could not save the checks"),
  });

  const preflight = useQuery({
    queryKey: ["preflight", claims?.org, projectId],
    enabled: Boolean(claims?.org),
    queryFn: () => getProjectPreflight(claims!.org, projectId),
  });
  const project = useQuery({
    queryKey: ["project", claims?.org, projectId],
    enabled: Boolean(claims?.org),
    queryFn: () => getProject(claims!.org, projectId),
  });
  const mcpServers = (() => {
    try {
      const parsed: unknown = JSON.parse(project.data?.mcp_servers || "[]");
      return Array.isArray(parsed) ? parsed.filter((item): item is { name: string; command: string } =>
        typeof item === "object" && item !== null && "name" in item && "command" in item) : [];
    } catch {
      return [];
    }
  })();

  const standards = useQuery({
    queryKey: ["design-standards", claims?.org],
    enabled: Boolean(claims?.org),
    queryFn: () => listDesignStandards(claims!.org),
  });
  const currentStandard = useQuery({
    queryKey: ["project-standard", claims?.org, projectId],
    enabled: Boolean(claims?.org),
    queryFn: () => getProjectStandard(claims!.org, projectId),
  });
  const saveStandard = useMutation({
    mutationFn: (id: string) => setProjectStandard(claims!.org, projectId, id),
    onSuccess: () => {
      setStandardError(null);
      queryClient.invalidateQueries({ queryKey: ["project-standard", claims?.org, projectId] });
      queryClient.invalidateQueries({ queryKey: ["project", claims?.org, projectId] });
    },
    onError: (err) => setStandardError(err instanceof ApiError ? err.message : "Could not save the standard"),
  });
  useEffect(() => {
    setStandardID(currentStandard.data?.design_standard?.id ?? "");
  }, [currentStandard.data?.design_standard?.id]);

  const projectComponents = useQuery({
    queryKey: ["project-components", claims?.org, projectId],
    enabled: Boolean(claims?.org),
    queryFn: () => listProjectComponents(claims!.org, projectId),
  });
  const componentEntries = useQuery({
    queryKey: ["component-entries", claims?.org],
    enabled: Boolean(claims?.org),
    queryFn: () => listComponentEntries(claims!.org),
  });
  useEffect(() => {
    const next: Record<string, { mode: "allow" | "exception"; reason: string }> = {};
    for (const item of projectComponents.data?.data ?? []) {
      next[item.id] = { mode: item.mode === "exception" ? "exception" : "allow", reason: item.reason };
    }
    setSelectedComponents(next);
  }, [projectComponents.data?.data]);
  const saveComponents = useMutation({
    mutationFn: (selected: Record<string, { mode: "allow" | "exception"; reason: string }>) =>
      setProjectComponents(
        claims!.org,
        projectId,
        Object.entries(selected).map(([id, value]) => ({ component_id: id, mode: value.mode, reason: value.reason })),
      ),
    onSuccess: () => {
      setComponentsError(null);
      queryClient.invalidateQueries({ queryKey: ["project-components", claims?.org, projectId] });
    },
    onError: (err) => setComponentsError(err instanceof ApiError ? err.message : "Could not save components"),
  });
  const allowedComponents = (componentEntries.data?.data ?? []).filter((item) => item.status === "approved" || item.status === "published");

  const requiredKey = project.data?.required_checks?.join(",") ?? "";
  const saveVisual = useMutation({
    mutationFn: (command: string) => setProjectVisual(claims!.org, projectId, command),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["project", claims?.org, projectId] });
    },
  });
  const saveMCP = useMutation({
    mutationFn: (config: string) => setProjectMCP(claims!.org, projectId, config),
    onSuccess: () => {
      setMCPError(null);
      queryClient.invalidateQueries({ queryKey: ["project", claims?.org, projectId] });
    },
    onError: (err) => setMCPError(err instanceof ApiError ? err.message : "Could not save the MCP config"),
  });
  async function loadMCPTools() {
    setMCPToolsLoading(true);
    setMCPToolsError(null);
    try {
      const result = await getMCPTools(claims!.org, projectId);
      setMCPTools(result.tools);
    } catch (err) {
      setMCPToolsError(err instanceof ApiError ? err.message : "Could not load MCP tools");
    } finally {
      setMCPToolsLoading(false);
    }
  }

  async function openBaseline() {
    setBaselineError(null);
    try {
      const blob = await getVisualBaseline(claims!.org, projectId, baselineName.trim());
      const url = URL.createObjectURL(blob);
      window.open(url, "_blank", "noopener");
      setTimeout(() => URL.revokeObjectURL(url), 60000);
    } catch (err) {
      setBaselineError(err instanceof ApiError ? err.message : "Could not load the baseline");
    }
  }
  const protect = useMutation({
    mutationFn: () => protectProjectBranch(claims!.org, projectId),
    onSuccess: (result) => setProtection(`${result.repo}:${result.branch} protected (${result.checks.join(", ") || "no checks"})`),
    onError: (err) => setProtection(err instanceof ApiError ? err.message : "Could not protect the branch"),
  });
  const savePolicy = useMutation({
    mutationFn: () => setProjectPolicy(claims!.org, projectId, Number(policyMax) || 0, policyPaths.split(",").map((p) => p.trim()).filter(Boolean)),
    onSuccess: () => { setPolicyError(null); queryClient.invalidateQueries({ queryKey: ["project", claims?.org, projectId] }); },
    onError: (err) => setPolicyError(err instanceof ApiError ? err.message : "Could not save the policy"),
  });
  const saveWindow = useMutation({
    mutationFn: () => setProjectChangeWindow(claims!.org, projectId, policyWindow.trim()),
    onSuccess: () => { setWindowError(null); queryClient.invalidateQueries({ queryKey: ["project", claims?.org, projectId] }); },
    onError: (err) => setWindowError(err instanceof ApiError ? err.message : "Could not save the change window"),
  });
  const saveModels = useMutation({
    mutationFn: () => setProjectAllowedModels(claims!.org, projectId, allowedModels.split(",").map((m) => m.trim()).filter(Boolean)),
    onSuccess: () => { setModelsError(null); queryClient.invalidateQueries({ queryKey: ["project", claims?.org, projectId] }); },
    onError: (err) => setModelsError(err instanceof ApiError ? err.message : "Could not save the allowed models"),
  });
  const saveTools = useMutation({
    mutationFn: () => setProjectAllowedTools(claims!.org, projectId, allowedTools.split(",").map((t) => t.trim()).filter(Boolean)),
    onSuccess: () => { setToolsError(null); queryClient.invalidateQueries({ queryKey: ["project", claims?.org, projectId] }); },
    onError: (err) => setToolsError(err instanceof ApiError ? err.message : "Could not save the allowed tools"),
  });
  const saveApprovals = useMutation({
    mutationFn: () => setProjectRequiredApprovals(claims!.org, projectId, Number(requiredApprovals) || 1),
    onSuccess: () => { setApprovalsError(null); queryClient.invalidateQueries({ queryKey: ["project", claims?.org, projectId] }); },
    onError: (err) => setApprovalsError(err instanceof ApiError ? err.message : "Could not save the approval rule"),
  });
  const scaffold = useMutation({
    mutationFn: () => scaffoldComponent(claims!.org, projectId, scaffoldName.trim(), scaffoldKind),
    onSuccess: (task) => {
      setScaffoldName("");
      setScaffoldNote(`created ${task.ref}: ${task.title}`);
      queryClient.invalidateQueries({ queryKey: ["tasks", claims?.org, projectId] });
    },
    onError: (err) => setScaffoldNote(err instanceof ApiError ? err.message : "Could not create the component task"),
  });
  const savePlugins = useMutation({
    mutationFn: () => setProjectPluginChecks(claims!.org, projectId, pluginChecks.trim() || "[]"),
    onSuccess: () => { setPluginError(null); queryClient.invalidateQueries({ queryKey: ["project", claims?.org, projectId] }); },
    onError: (err) => setPluginError(err instanceof ApiError ? err.message : "Could not save the plugin checks"),
  });
  const removeProject = useMutation({
    mutationFn: () => deleteProject(claims!.org, projectId, deleteConfirm),
    onSuccess: () => router.push("/projects"),
    onError: (err) => setDeleteError(err instanceof ApiError ? err.message : "Could not delete the project"),
  });
  const saveE2E = useMutation({
    mutationFn: (command: string) => setProjectE2E(claims!.org, projectId, command),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["project", claims?.org, projectId] });
    },
  });
  const saveTestCommand = useMutation({
    mutationFn: (command: string) => setProjectTestCommand(claims!.org, projectId, command),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["project", claims?.org, projectId] });
    },
  });
  useEffect(() => {
    setChecks(requiredKey);
  }, [requiredKey]);
  useEffect(() => {
    setMCPConfig(project.data?.mcp_servers ?? "");
  }, [project.data?.mcp_servers]);
  useEffect(() => {
    setPolicyMax(String(project.data?.policy_max_diff_bytes ?? 0));
    setPolicyPaths((project.data?.policy_forbidden_paths ?? []).join(", "));
    setPolicyWindow(project.data?.policy_change_window ?? "");
    setAllowedModels((project.data?.policy_allowed_models ?? []).join(", "));
    setAllowedTools((project.data?.policy_allowed_tools ?? []).join(", "));
    setRequiredApprovals(String(project.data?.policy_required_approvals ?? 1));
    setPluginChecks(project.data?.plugin_checks ?? "[]");
  }, [project.data?.policy_max_diff_bytes, project.data?.policy_forbidden_paths, project.data?.policy_change_window, project.data?.policy_allowed_models, project.data?.policy_allowed_tools, project.data?.policy_required_approvals, project.data?.plugin_checks]);

  const tasks = useQuery({
    queryKey: ["tasks", claims?.org, projectId],
    enabled: Boolean(claims?.org),
    queryFn: () => listTasks(claims!.org, projectId),
  });

  const runTask = useMutation({
    mutationFn: (ref: string) => startRun(claims!.org, projectId, ref),
    onSuccess: (run) => router.push(`/projects/${projectId}/runs/${run.id}`),
    onError: (err) =>
      setFormError(err instanceof ApiError ? err.message : "Could not start the run"),
  });

  const taskItems = tasks.data?.data ?? [];
  const readyTasks = taskItems.filter((task) => task.status === "ready").length;
  const activeTasks = taskItems.filter((task) => ["planning", "in_progress", "review"].includes(task.status)).length;
  const blockedTasks = taskItems.filter((task) => task.status === "blocked").length;

  return (
    <div className="page-stack">
      <nav aria-label="Breadcrumb" className="mb-1 text-xs text-text-muted">
        <Link href="/projects" className="hover:text-text-secondary">
          Projects
        </Link>
        <span> / </span>
        <span className="text-text-secondary">{project.data?.name ?? "…"}</span>
      </nav>

      <header className="page-hero mb-6 flex items-end justify-between gap-4">
        <div>
          <h1 className="text-[22px] font-semibold">{project.data?.name ?? "Project"}</h1>
          <p className="mt-1 text-[13px] text-text-secondary">
            {project.data?.description || "Tasks for this project"}
          </p>
        </div>
        <Button onClick={() => setShowForm(true)}>+ New task</Button>
      </header>

      <Tabs
        ariaLabel="Project sections"
        value={tab}
        onChange={(id) => setTab(id as typeof tab)}
        items={[
          { id: "overview", label: "Overview" },
          { id: "board", label: "Board", count: taskItems.length },
          { id: "design", label: "Design" },
          { id: "delivery", label: "Delivery" },
        ]}
      />

      {tab === "overview" ? (
      <section className="mb-5 grid gap-3 sm:grid-cols-2 xl:grid-cols-4" aria-label="Project delivery summary">
        {[{label:"Total tasks",value:taskItems.length,note:"Tracked work items"},{label:"Ready for agent",value:readyTasks,note:"Can be executed"},{label:"In progress",value:activeTasks,note:"Planning, coding or review"},{label:"Blocked",value:blockedTasks,note:"Needs a decision"}].map((item) => <div key={item.label} className="metric-panel"><span>{item.label}</span><strong>{item.value}</strong><small>{item.note}</small></div>)}
      </section>
      ) : null}

      {showForm && claims ? (
        <TaskComposer
          orgID={claims.org}
          projectID={projectId}
          tasks={taskItems}
          onClose={() => setShowForm(false)}
          onSaved={() => {
            setShowForm(false);
            setFormError(null);
            queryClient.invalidateQueries({ queryKey: ["tasks", claims.org, projectId] });
          }}
        />
      ) : null}

      {editingTask && claims ? (
        <TaskComposer
          orgID={claims.org}
          projectID={projectId}
          tasks={taskItems}
          task={editingTask}
          onClose={() => setEditingTask(null)}
          onSaved={() => {
            setEditingTask(null);
            setSelectedTask(null);
            queryClient.invalidateQueries({ queryKey: ["tasks", claims.org, projectId] });
          }}
        />
      ) : null}

      {formError ? (
        <p role="alert" className="mb-3 rounded-lg border border-status-danger/30 bg-status-danger/10 p-3 text-sm text-status-danger">
          {formError}
        </p>
      ) : null}

      {tab === "overview" ? (<>
      {tasks.isPending ? (
        <div className="space-y-2" aria-label="Loading tasks">
          {[0, 1, 2, 3].map((i) => (
            <div key={i} className="h-14 animate-pulse rounded-lg bg-bg-hover" />
          ))}
        </div>
      ) : null}

      {tasks.isError ? (
        <div role="alert" className="rounded-lg border border-status-danger/30 bg-status-danger/10 p-4 text-sm">
          <p className="font-medium text-status-danger">
            {tasks.error instanceof ApiError ? tasks.error.message : "Failed to load tasks"}
          </p>
          <Button variant="secondary" className="mt-3" onClick={() => tasks.refetch()}>
            Retry
          </Button>
        </div>
      ) : null}

      {tasks.isSuccess && tasks.data.data.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border bg-bg-surface p-12 text-center">
          <p className="font-medium">No tasks yet</p>
          <p className="mt-1 text-sm text-text-secondary">
            Create your first task with acceptance criteria to start the AI workflow.
          </p>
          <Button className="mt-4" onClick={() => setShowForm(true)}>
            + New task
          </Button>
        </div>
      ) : null}

      {tasks.isSuccess && tasks.data.data.length > 0 ? (
        <ul className="divide-y divide-border overflow-hidden rounded-lg border border-border bg-bg-surface">
          {tasks.data.data.slice(0, 8).map((task) => (
            <li key={task.id} className="flex items-center gap-4 px-4 py-3">
              <span className="w-14 font-mono text-xs text-text-muted">{task.ref}</span>
              <button
                type="button"
                onClick={() => setSelectedTask(task)}
                className="flex-1 text-left text-sm hover:text-brand"
              >
                {task.title}
              </button>
              <span className="rounded bg-bg-hover px-1.5 py-0.5 text-[11px] uppercase text-text-muted">
                {task.priority}
              </span>
              <span
                className={`rounded-md px-2 py-0.5 text-xs font-medium ${
                  statusStyles[task.status] ?? "bg-bg-hover text-text-muted"
                }`}
              >
                {task.status}
              </span>
              {task.status === "ready" ? (
                <Button
                  variant="secondary"
                  className="h-7 px-2 text-xs"
                  loading={runTask.isPending && runTask.variables === task.ref}
                  onClick={() => runTask.mutate(task.ref)}
                >
                  Run agent
                </Button>
              ) : null}
            </li>
          ))}
        </ul>
      ) : null}
      {tasks.isSuccess && tasks.data.data.length > 8 ? (
        <Button variant="secondary" className="mt-3" onClick={() => setTab("board")}>
          View all {tasks.data.data.length} tasks in the board
        </Button>
      ) : null}
      </>) : null}

      {tab === "design" && claims ? (
        <section className="order-2 mt-5 rounded-lg border border-border bg-bg-surface p-4">
          <h2 className="text-sm font-semibold">Component allowlist</h2>
          <p className="mt-1 text-xs text-text-secondary">
            Approved components agents may use in this project. Forbidden/deprecated imports fail the design check.
          </p>
          <div className="mt-3 grid gap-1.5 md:grid-cols-2">
            {allowedComponents.map((component) => (
              <div key={component.id} className="rounded-md border border-border p-2 text-xs">
                <label className="flex items-center gap-2">
                  <input
                    type="checkbox"
                    checked={Boolean(selectedComponents[component.id])}
                    onChange={(event) =>
                      setSelectedComponents((current) => {
                        const next = { ...current };
                        if (event.target.checked) next[component.id] = { mode: "allow", reason: "" };
                        else delete next[component.id];
                        return next;
                      })
                    }
                  />
                  <span className="font-medium">{component.name}</span>
                  <span className="text-text-muted">{component.category}</span>
                  <span className="ml-auto font-mono text-[10px] text-text-muted">{component.import_path || component.package}</span>
                </label>
                {selectedComponents[component.id] ? (
                  <div className="mt-2 flex flex-wrap items-center gap-2">
                    <select
                      aria-label={`${component.name} mode`}
                      value={selectedComponents[component.id].mode}
                      onChange={(event) =>
                        setSelectedComponents((current) => ({
                          ...current,
                          [component.id]: { ...current[component.id], mode: event.target.value as "allow" | "exception" },
                        }))
                      }
                      className="h-7 rounded-md border border-border bg-bg-surface px-2 text-[11px]"
                    >
                      <option value="allow">allow</option>
                      <option value="exception">exception</option>
                    </select>
                    <input
                      aria-label={`${component.name} exception reason`}
                      placeholder={selectedComponents[component.id].mode === "exception" ? "Reason (required)" : "Reason (optional)"}
                      value={selectedComponents[component.id].reason}
                      onChange={(event) =>
                        setSelectedComponents((current) => ({
                          ...current,
                          [component.id]: { ...current[component.id], reason: event.target.value },
                        }))
                      }
                      className="h-7 min-w-[200px] flex-1 rounded-md border border-border bg-bg-surface px-2 text-[11px]"
                    />
                  </div>
                ) : null}
              </div>
            ))}
          </div>
          <div className="mt-3 flex items-center gap-2">
            <Button variant="secondary" loading={saveComponents.isPending} onClick={() => saveComponents.mutate(selectedComponents)}>
              Save allowlist
            </Button>
            <span className="text-xs text-text-muted">{Object.keys(selectedComponents).length} selected</span>
          </div>
          {componentsError ? <p role="alert" className="mt-2 text-xs text-status-danger">{componentsError}</p> : null}
        </section>
      ) : null}

      {tab === "design" && claims ? (
        <section className="order-1 rounded-lg border border-border bg-bg-surface p-4">
          <h2 className="text-sm font-semibold">Scaffold component</h2>
          <p className="mt-1 text-xs text-text-secondary">
            Creates a task with the expected files, tokens, registry and test requirements for a new UI component.
          </p>
          <div className="mt-3 flex flex-wrap items-end gap-2">
            <input aria-label="Component name" value={scaffoldName} onChange={(e) => setScaffoldName(e.target.value)} placeholder="ButtonGroup" className="h-9 min-w-[200px] flex-1 rounded-md border border-border bg-bg-surface px-3 text-xs" />
            <select aria-label="Component kind" value={scaffoldKind} onChange={(e) => setScaffoldKind(e.target.value)} className="h-9 rounded-md border border-border bg-bg-surface px-2 text-xs">
              <option value="react">React</option><option value="vue">Vue</option><option value="angular">Angular</option>
            </select>
            <Button variant="secondary" loading={scaffold.isPending} onClick={() => scaffold.mutate()}>Create task</Button>
          </div>
          {scaffoldNote ? <p className="mt-2 text-xs text-text-secondary">{scaffoldNote}</p> : null}
        </section>
      ) : null}

      {tab === "design" && claims ? (
        <section className="order-1 rounded-lg border border-border bg-bg-surface p-4">
          <h2 className="text-sm font-semibold">Design standard</h2>
          <p className="mt-1 text-xs text-text-secondary">
            Published standards this project inherits; agents will receive its tokens and component rules.
          </p>
          <div className="mt-2 flex flex-wrap items-end gap-2">
            <select
              aria-label="Design standard"
              value={standardID}
              onChange={(event) => setStandardID(event.target.value)}
              className="h-9 min-w-[260px] flex-1 rounded-md border border-border bg-bg-surface px-3 text-sm"
            >
              <option value="">No standard</option>
              {(standards.data?.data ?? [])
                .filter((standard) => standard.status === "published")
                .map((standard) => (
                  <option key={standard.id} value={standard.id}>
                    {standard.name} · v{standard.version}
                  </option>
                ))}
            </select>
            <Button variant="secondary" loading={saveStandard.isPending} onClick={() => saveStandard.mutate(standardID)}>
              Save
            </Button>
          </div>
          {standardError ? <p role="alert" className="mt-2 text-xs text-status-danger">{standardError}</p> : null}
          <p className="mt-2 text-xs text-text-muted">
            Current:{" "}
            {currentStandard.data?.design_standard
              ? `${currentStandard.data.design_standard.name} v${currentStandard.data.design_standard.version} (${currentStandard.data.design_standard.sha256.slice(0, 12)}…)`
              : "none"}
          </p>
        </section>
      ) : null}

      {tab === "delivery" ? (
        <Tabs
          ariaLabel="Delivery sections"
          variant="pills"
          value={deliveryTab}
          onChange={(id) => setDeliveryTab(id as typeof deliveryTab)}
          items={[
            { id: "checks", label: "Checks" },
            { id: "repos", label: "Repositories" },
            { id: "environments", label: "Environments" },
            { id: "prs", label: "Pull requests" },
            { id: "access", label: "Access" },
          ]}
        />
      ) : null}

      {tab === "delivery" && deliveryTab === "checks" && claims ? (
        <section className="rounded-lg border border-border bg-bg-surface p-4">
          <h2 className="text-sm font-semibold">Required checks</h2>
          <p className="mt-1 text-xs text-text-secondary">
            Comma-separated check names that must pass before a PR can be merged (for example <code>ci</code> or <code>test</code>).
          </p>
          <div className="mt-2 flex flex-wrap items-end gap-2">
            <input
              aria-label="Required checks"
              value={checks}
              onChange={(e) => setChecks(e.target.value)}
              placeholder="ci, test"
              className="h-9 min-w-[240px] flex-1 rounded-md border border-border bg-bg-surface px-3 text-sm"
            />
            <Button
              variant="secondary"
              loading={saveChecks.isPending}
              onClick={() => saveChecks.mutate(checks.split(",").map((c) => c.trim()).filter(Boolean))}
            >
              Save
            </Button>
          </div>
          {checksError ? <p role="alert" className="mt-2 text-xs text-status-danger">{checksError}</p> : null}
          <p className="mt-2 text-xs text-text-muted">
            Current: {(project.data?.required_checks ?? []).join(", ") || "none"}
          </p>
        </section>
      ) : null}

      {tab === "delivery" && deliveryTab === "checks" && claims ? (
        <CommandForm
          id="e2e-command"
          title="E2E command"
          description="Runs in an isolated sandbox after the unit tests. A failure blocks the PR. Leave empty to disable."
          placeholder="npm run e2e"
          initial={project.data?.e2e_command ?? ""}
          onSave={(command) => saveE2E.mutateAsync(command)}
        />
      ) : null}

      {tab === "delivery" && deliveryTab === "checks" && claims ? (
        <CommandForm
          id="test-command"
          title="Test command"
          description="The gate command the agent must pass (unit and integration tests). A failure blocks the PR. Leave empty to use the global AIP_TEST_COMMAND."
          placeholder="go test ./... && cd apps/web && pnpm test"
          initial={project.data?.test_command ?? ""}
          onSave={(command) => saveTestCommand.mutateAsync(command)}
        />
      ) : null}

      {tab === "delivery" && deliveryTab === "checks" && claims ? (
        <section className="mt-5 rounded-lg border border-border bg-bg-surface p-4">
          <h2 className="text-sm font-semibold">MCP servers</h2>
          <p className="mt-1 text-xs text-text-secondary">
            JSON list of trusted servers started in the run sandbox: [&quot;name&quot;: &quot;fs&quot;, &quot;command&quot;: &quot;mcp-fs&quot;].
          </p>
          {mcpServers.length > 0 ? (
            <ul className="mt-2 flex flex-wrap gap-2 text-xs">
              {mcpServers.map((server) => (
                <li key={server.name} className="rounded border border-border px-2 py-1">
                  <span className="font-medium">{server.name}</span>
                  <span className="ml-1 text-text-muted">{server.command}</span>
                </li>
              ))}
            </ul>
          ) : null}
          <textarea
            aria-label="MCP servers"
            value={mcpConfig}
            onChange={(e) => setMCPConfig(e.target.value)}
            rows={3}
            placeholder='[{"name":"fs","command":"mcp-fs","args":["--root","/workspace"]}]'
            className="mt-2 w-full rounded-md border border-border bg-bg-surface px-3 py-2 font-mono text-xs"
          />
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <Button variant="secondary" loading={saveMCP.isPending} onClick={() => saveMCP.mutate(mcpConfig)}>Save</Button>
            <Button variant="secondary" loading={mcpToolsLoading} onClick={loadMCPTools}>Load tools from last run</Button>
            {mcpError ? <p role="alert" className="text-xs text-status-danger">{mcpError}</p> : null}
            {mcpToolsError ? <p role="alert" className="text-xs text-status-danger">{mcpToolsError}</p> : null}
          </div>
          {mcpTools !== null ? (
            mcpTools.length > 0 ? (
              <ul className="mt-2 flex flex-wrap gap-2 text-xs">
                {mcpTools.map((tool) => (
                  <li key={`${tool.server}/${tool.name}`} className="rounded border border-border px-2 py-1">
                    <span className="text-text-muted">{tool.server}/</span>
                    <span className="font-medium">{tool.name}</span>
                  </li>
                ))}
              </ul>
            ) : (
              <p className="mt-2 text-xs text-text-muted">No MCP tools in the last run.</p>
            )
          ) : null}
        </section>
      ) : null}

      {tab === "delivery" && deliveryTab === "checks" && claims ? (
        <CommandForm
          id="visual-command"
          title="Visual command"
          description="Bring your own screenshot diff (Playwright/Percy). A failure records a blocking visual check."
          placeholder="npx playwright test --project=visual"
          initial={project.data?.visual_command ?? ""}
          onSave={(command) => saveVisual.mutateAsync(command)}
        >
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <input
              aria-label="Baseline name"
              value={baselineName}
              onChange={(e) => setBaselineName(e.target.value)}
              placeholder="baseline name (e.g. home)"
              className="h-9 rounded-md border border-border bg-bg-surface px-3 text-xs"
            />
            <Button variant="secondary" onClick={openBaseline}>Open baseline</Button>
            <Button variant="secondary" loading={protect.isPending} onClick={() => protect.mutate()}>Protect default branch</Button>
            {protection ? <span className="text-xs text-text-secondary">{protection}</span> : null}
            {baselineError ? <span className="text-xs text-status-danger">{baselineError}</span> : null}
          </div>
        </CommandForm>
      ) : null}

      {tab === "delivery" && deliveryTab === "checks" && claims ? (
        <section className="mt-5 rounded-lg border border-border bg-bg-surface p-4">
          <h2 className="text-sm font-semibold">Diff policy</h2>
          <p className="mt-1 text-xs text-text-secondary">0 disables the size limit. Forbidden paths are relative to the repository.</p>
          <div className="mt-2 flex flex-wrap items-end gap-2">
            <input aria-label="Max diff bytes" value={policyMax} onChange={(e) => setPolicyMax(e.target.value)} placeholder="0" className="h-9 w-32 rounded-md border border-border bg-bg-surface px-3 text-xs" />
            <input aria-label="Forbidden paths" value={policyPaths} onChange={(e) => setPolicyPaths(e.target.value)} placeholder="secrets, infra/prod" className="h-9 min-w-[240px] flex-1 rounded-md border border-border bg-bg-surface px-3 text-xs" />
            <Button variant="secondary" loading={savePolicy.isPending} onClick={() => savePolicy.mutate()}>Save policy</Button>
          </div>
          {policyError ? <p role="alert" className="mt-2 text-xs text-status-danger">{policyError}</p> : null}
          <div className="mt-4 border-t border-border pt-3">
            <h3 className="text-xs font-semibold">Merge window</h3>
            <p className="mt-1 text-xs text-text-secondary">
              JSON: {'{"days":[1,2,3,4,5],"start":"09:00","end":"17:00","tz":"Europe/Belgrade"}'}. Empty means always.
            </p>
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <input aria-label="Change window" value={policyWindow} onChange={(e) => setPolicyWindow(e.target.value)} placeholder="empty = always" className="h-9 min-w-[280px] flex-1 rounded-md border border-border bg-bg-surface px-3 font-mono text-xs" />
              <Button variant="secondary" loading={saveWindow.isPending} onClick={() => saveWindow.mutate()}>Save window</Button>
            </div>
            {windowError ? <p role="alert" className="mt-2 text-xs text-status-danger">{windowError}</p> : null}
          </div>
          <div className="mt-4 border-t border-border pt-3">
            <h3 className="text-xs font-semibold">Allowed models</h3>
            <p className="mt-1 text-xs text-text-secondary">Comma-separated model names; empty means any model.</p>
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <input aria-label="Allowed models" value={allowedModels} onChange={(e) => setAllowedModels(e.target.value)} placeholder="claude-sonnet-4, gpt-5" className="h-9 min-w-[280px] flex-1 rounded-md border border-border bg-bg-surface px-3 font-mono text-xs" />
              <Button variant="secondary" loading={saveModels.isPending} onClick={() => saveModels.mutate()}>Save models</Button>
            </div>
            {modelsError ? <p role="alert" className="mt-2 text-xs text-status-danger">{modelsError}</p> : null}
          </div>
          <div className="mt-4 border-t border-border pt-3">
            <h3 className="text-xs font-semibold">Allowed MCP tools</h3>
            <p className="mt-1 text-xs text-text-secondary">Comma-separated exact names or prefixes ending in * (e.g. mcp__fs__read, mcp__git__*). Empty means all.</p>
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <input aria-label="Allowed MCP tools" value={allowedTools} onChange={(e) => setAllowedTools(e.target.value)} placeholder="mcp__fs__read, mcp__git__*" className="h-9 min-w-[280px] flex-1 rounded-md border border-border bg-bg-surface px-3 font-mono text-xs" />
              <Button variant="secondary" loading={saveTools.isPending} onClick={() => saveTools.mutate()}>Save tools</Button>
            </div>
            {toolsError ? <p role="alert" className="mt-2 text-xs text-status-danger">{toolsError}</p> : null}
          </div>
          <div className="mt-4 border-t border-border pt-3">
            <h3 className="text-xs font-semibold">Required approvals</h3>
            <p className="mt-1 text-xs text-text-secondary">Distinct approvers needed before a PR merge (1–3).</p>
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <select aria-label="Required approvals" value={requiredApprovals} onChange={(e) => setRequiredApprovals(e.target.value)} className="h-9 rounded-md border border-border bg-bg-surface px-2 text-xs">
                <option value="1">1</option><option value="2">2</option><option value="3">3</option>
              </select>
              <Button variant="secondary" loading={saveApprovals.isPending} onClick={() => saveApprovals.mutate()}>Save rule</Button>
            </div>
            {approvalsError ? <p role="alert" className="mt-2 text-xs text-status-danger">{approvalsError}</p> : null}
          </div>
          <div className="mt-4 border-t border-border pt-3">
            <h3 className="text-xs font-semibold">Plugin checks</h3>
            <p className="mt-1 text-xs text-text-secondary">JSON list of named sandbox commands (name + command fields). Max 5.</p>
            <textarea aria-label="Plugin checks" value={pluginChecks} onChange={(e) => setPluginChecks(e.target.value)} rows={3} className="mt-2 w-full rounded-md border border-border bg-bg-surface px-3 py-2 font-mono text-xs" />
            <div className="mt-2 flex items-center gap-2">
              <Button variant="secondary" loading={savePlugins.isPending} onClick={() => savePlugins.mutate()}>Save plugins</Button>
              {pluginError ? <span role="alert" className="text-xs text-status-danger">{pluginError}</span> : null}
            </div>
          </div>
        </section>
      ) : null}

      {tab === "delivery" && deliveryTab === "checks" && claims ? (
        <section className="mt-5 rounded-lg border border-border bg-bg-surface p-4">
          <div className="flex flex-wrap items-start justify-between gap-2">
            <div>
              <h2 className="text-sm font-semibold">Preflight</h2>
              <p className="mt-1 text-xs text-text-secondary">
                What must be true before an agent can run safely in this project.
              </p>
            </div>
            <Button variant="secondary" loading={preflight.isFetching} onClick={() => preflight.refetch()}>Check again</Button>
          </div>
          {preflight.isError ? <p role="alert" className="mt-3 text-xs text-status-danger">Could not load preflight.</p> : null}
          {preflight.isSuccess ? (
            <ul className="mt-3 divide-y divide-border rounded-md border border-border">
              {preflight.data.checks.map((check) => (
                <li key={check.key} className="flex flex-wrap items-center gap-2 px-3 py-2 text-xs">
                  <span className={`rounded px-1.5 py-0.5 text-[10px] uppercase ${
                    check.status === "ok"
                      ? "bg-status-success/10 text-status-success"
                      : check.status === "warn"
                        ? "bg-status-warning/10 text-status-warning"
                        : "bg-status-danger/10 text-status-danger"
                  }`}>{check.status}</span>
                  <span className="font-medium">{check.label}</span>
                  <span className="flex-1 text-text-secondary">{check.detail}</span>
                  {check.fix ? <span className="text-text-muted">{check.fix}</span> : null}
                </li>
              ))}
            </ul>
          ) : null}
        </section>
      ) : null}

      {tab === "board" && claims && tasks.data ? (
        <TaskBoard
          orgID={claims.org}
          projectID={projectId}
          tasks={tasks.data.data}
          onOpen={(task) => setSelectedTask(task)}
        />
      ) : null}

      {tab === "delivery" && deliveryTab === "repos" && claims ? (
        <RepoPanel
          orgID={claims.org}
          projectID={projectId}
          canManage={claims.role === "owner" || claims.role === "admin"}
        />
      ) : null}
      {tab === "delivery" && deliveryTab === "environments" && claims ? <><EnvironmentsPanel orgID={claims.org} projectID={projectId} /><PreviewsPanel orgID={claims.org} projectID={projectId} /></> : null}
      {tab === "delivery" && deliveryTab === "prs" && claims ? <PullRequestsPanel orgID={claims.org} projectID={projectId} /> : null}
      {tab === "delivery" && deliveryTab === "access" && claims ? <ProjectMembersPanel orgID={claims.org} projectID={projectId} /> : null}

      {claims && selectedTask ? (
        <TaskDrawer
          orgID={claims.org}
          projectID={projectId}
          task={selectedTask}
          onClose={() => setSelectedTask(null)}
          onEdit={(task) => {
            setSelectedTask(null);
            setEditingTask(task);
          }}
        />
      ) : null}
      {tab === "delivery" && claims ? (
        <section className="order-2 mt-5 rounded-lg border border-status-danger/40 bg-status-danger/5 p-4">
          <h2 className="text-sm font-semibold text-status-danger">Delete project</h2>
          <p className="mt-1 text-xs text-text-secondary">This removes the project, tasks, runs, repos and incidents. Type the project name to confirm.</p>
          <div className="mt-3 flex flex-wrap items-center gap-2">
            <input aria-label="Confirm project name" value={deleteConfirm} onChange={(e) => setDeleteConfirm(e.target.value)} placeholder={project.data?.name ?? "project name"} className="h-9 min-w-[220px] flex-1 rounded-md border border-border bg-bg-surface px-3 text-xs" />
            <Button variant="secondary" loading={removeProject.isPending} onClick={() => removeProject.mutate()}>Delete project</Button>
          </div>
          {deleteError ? <p role="alert" className="mt-2 text-xs text-status-danger">{deleteError}</p> : null}
        </section>
      ) : null}

    </div>
  );
}
