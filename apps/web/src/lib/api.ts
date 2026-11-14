import { getAccessToken, getRefreshToken, clearSession, setSession } from "./session";
import { decodeClaims, type Claims } from "./auth";

export const API_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:18080";

export class ApiError extends Error {
  constructor(
    readonly status: number,
    readonly code: string,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

async function send(path: string, init: RequestInit = {}): Promise<Response> {
  const headers = new Headers(init.headers);
  if (!(init.body instanceof FormData)) headers.set("Content-Type", "application/json");
  const token = getAccessToken();
  if (token) headers.set("Authorization", `Bearer ${token}`);
  return fetch(`${API_URL}${path}`, { ...init, headers });
}

async function toError(res: Response): Promise<ApiError> {
  let code = "request_failed";
  let detail = res.statusText;
  try {
    const body = (await res.json()) as { code?: string; detail?: string };
    code = body.code ?? code;
    detail = body.detail ?? detail;
  } catch {
    // non-JSON error body
  }
  return new ApiError(res.status, code, detail);
}

// Refresh tokens rotate on the server, so two concurrent refreshes would make
// the second one look like token reuse and revoke the session. Single-flight it.
let refreshInFlight: Promise<boolean> | null = null;

async function doRefresh(): Promise<boolean> {
  const refresh = getRefreshToken();
  if (!refresh) return false;
  const res = await send("/api/v1/auth/refresh", {
    method: "POST",
    body: JSON.stringify({ refresh_token: refresh }),
  });
  if (!res.ok) {
    clearSession();
    return false;
  }
  setSession((await res.json()) as TokenPair);
  return true;
}

async function refreshSession(): Promise<boolean> {
  if (!refreshInFlight) {
    refreshInFlight = doRefresh().finally(() => {
      refreshInFlight = null;
    });
  }
  return refreshInFlight;
}

async function request<T>(path: string, init: RequestInit = {}, retry = true): Promise<T> {
  let res = await send(path, init);
  if (res.status === 401 && retry && (await refreshSession())) {
    res = await send(path, init);
  }
  if (!res.ok) throw await toError(res);
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export type TokenPair = {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  refresh_expires_in: number;
  token_type: string;
};

export type Project = {
  id: string;
  org_id: string;
  name: string;
  slug: string;
  description: string;
  status: string;
  required_checks?: string[];
  e2e_command?: string;
  visual_command?: string;
  mcp_servers?: string;
  policy_max_diff_bytes?: number;
  policy_forbidden_paths?: string[];
  policy_change_window?: string;
  policy_allowed_models?: string[];
  policy_allowed_tools?: string[];
  policy_required_approvals?: number;
  plugin_checks?: string;
  created_at: string;
  updated_at: string;
};

export type Acceptance = { given: string; when: string; then: string };

export type Task = {
  id: string;
  org_id: string;
  project_id: string;
  ref: string;
  title: string;
  description: string;
  acceptance: Acceptance[];
  status: string;
  priority: "low" | "med" | "high";
  tags: string[];
  depends_on?: string[];
  blocked_reason?: string;
  created_at: string;
  updated_at: string;
};

export async function login(email: string, password: string) {
  const pair = await request<TokenPair>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
  setSession(pair);
  return pair;
}

export async function register(input: {
  name: string;
  email: string;
  password: string;
  org_name: string;
}) {
  return request<{ user_id?: string; org_slug?: string; status?: string }>(
    "/api/v1/auth/register",
    { method: "POST", body: JSON.stringify(input) },
    false,
  );
}

export function createProject(orgID: string, input: { name: string; description: string }) {
  return request<Project>(`/api/v1/orgs/${orgID}/projects`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function listProjects(orgID: string) {
  return request<{ data: Project[]; next_cursor: string | null }>(
    `/api/v1/orgs/${orgID}/projects`,
  );
}

export function getProject(orgID: string, projectID: string) {
  return request<Project>(`/api/v1/orgs/${orgID}/projects/${projectID}`);
}

export function updateTask(
  orgID: string,
  projectID: string,
  taskID: string,
  input: { title: string; description: string; priority: string; assignee_id?: string | null },
) {
  return request<Task>(`/api/v1/orgs/${orgID}/projects/${projectID}/tasks/${taskID}`, {
    method: "PATCH",
    body: JSON.stringify(input),
  });
}

export function transitionTask(
  orgID: string,
  projectID: string,
  taskID: string,
  to: string,
  reason?: string,
) {
  return request<Task>(`/api/v1/orgs/${orgID}/projects/${projectID}/tasks/${taskID}/transitions`, {
    method: "POST",
    body: JSON.stringify({ to, reason: reason ?? null }),
  });
}

export function listTasks(orgID: string, projectID: string, status?: string) {
  const query = status ? `?status=${encodeURIComponent(status)}` : "";
  return request<{ data: Task[]; next_cursor: string | null }>(
    `/api/v1/orgs/${orgID}/projects/${projectID}/tasks${query}`,
  );
}

export function createTask(
  orgID: string,
  projectID: string,
  input: { title: string; description?: string; acceptance: Acceptance[]; priority?: "low" | "med" | "high"; tags?: string[]; depends_on?: string[] },
) {
  return request<Task>(`/api/v1/orgs/${orgID}/projects/${projectID}/tasks`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

/** Returns the current session's claims, refreshing once if needed. */
export async function logoutAll(): Promise<void> {
  await request<void>("/api/v1/auth/logout-all", { method: "POST" });
}

export async function logout(): Promise<void> {
  try {
    await request<void>("/api/v1/auth/logout", { method: "POST" });
  } finally {
    clearSession();
  }
}

export async function ensureSession(): Promise<Claims | null> {
  let token = getAccessToken();
  if (!token) {
    if (!(await refreshSession())) return null;
    token = getAccessToken();
  }
  return token ? decodeClaims(token) : null;
}

export type Run = {
  id: string;
  org_id: string;
  project_id: string;
  task_id: string;
  agent_type: string;
  status: string;
  model: string;
  current_phase?: string;
  failure_code?: string;
  failure_detail?: string;
  created_at: string;
  started_at?: string;
  finished_at?: string;
  plan?: { steps?: string[]; files?: string[]; risks?: string[] } | null;
  diff?: string;
  diff_bytes?: number;
  trace_id?: string;
  budget?: { tokens_in: number; tokens_out: number; cost_micros: number };
  review?: {
    findings?: { severity: string; file?: string; line?: number; body: string }[];
  } | null;
};

export type Approval = {
  id: string;
  run_id: string;
  type: string;
  status: string;
  requested_at: string;
  pull_request_id?: string;
  number?: number;
  url?: string;
};

export type RunEvent = {
  id: number;
  type: string;
  data: { actor?: string; payload?: Record<string, unknown> };
};

export function startRun(orgID: string, projectID: string, taskRef: string) {
  return request<Run>(
    `/api/v1/orgs/${orgID}/projects/${projectID}/tasks/${taskRef}/runs`,
    { method: "POST", body: JSON.stringify({ agent_type: "planning" }) },
  );
}

export function listApprovals(orgID: string) {
  return request<{ data: Approval[] }>(`/api/v1/orgs/${orgID}/approvals`);
}

export function decideApproval(
  orgID: string,
  approvalID: string,
  decision: "approved" | "rejected",
  reason: string,
) {
  return request<Run & { development_run_id?: string }>(
    `/api/v1/orgs/${orgID}/approvals/${approvalID}/decision`,
    { method: "POST", body: JSON.stringify({ decision, reason }) },
  );
}

export function getRun(orgID: string, runID: string) {
  return request<Run>(`/api/v1/orgs/${orgID}/runs/${runID}`);
}

/**
 * Reads the SSE stream with fetch (EventSource cannot send the Bearer token).
 * Parses `id:`, `event:` and `data:` frames and ignores comments/heartbeats.
 */
export async function streamRunEvents(
  orgID: string,
  runID: string,
  onEvent: (event: RunEvent) => void,
  signal: AbortSignal,
): Promise<void> {
  const token = getAccessToken();
  const res = await fetch(`${API_URL}/api/v1/orgs/${orgID}/runs/${runID}/events?after=0`, {
    headers: {
      Accept: "text/event-stream",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    signal,
  });
  if (!res.ok || !res.body) throw new ApiError(res.status, "stream_failed", "could not open the event stream");

  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buffer = "";

  for (;;) {
    const { done, value } = await reader.read();
    if (done) return;
    buffer += decoder.decode(value, { stream: true });

    let index = buffer.indexOf("\n\n");
    while (index >= 0) {
      const frame = buffer.slice(0, index);
      buffer = buffer.slice(index + 2);
      index = buffer.indexOf("\n\n");

      let id = 0;
      let type = "message";
      let data = "";
      for (const line of frame.split("\n")) {
        if (line.startsWith("id: ")) id = Number(line.slice(4));
        else if (line.startsWith("event: ")) type = line.slice(7);
        else if (line.startsWith("data: ")) data += line.slice(6);
      }
      if (id > 0 && data) {
        try {
          onEvent({ id, type, data: JSON.parse(data) as RunEvent["data"] });
        } catch {
          // ignore malformed frames; the stream must not break on one bad line
        }
      }
    }
  }
}

export type Repo = {
  id: string;
  provider: "github" | "gitlab";
  name: string;
  default_branch: string;
  created_at: string;
};

export function listRepos(orgID: string, projectID: string) {
  return request<{ data: Repo[] }>(`/api/v1/orgs/${orgID}/projects/${projectID}/repos`);
}

export type ConnectRepoInput = {
  provider: "github" | "gitlab";
  name: string;
  default_branch: string;
  webhook_secret: string;
};

export function connectRepo(orgID: string, projectID: string, input: ConnectRepoInput) {
  return request<Repo>(`/api/v1/orgs/${orgID}/projects/${projectID}/repos`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function repoURL(repo: Repo): string {
  return repo.provider === "gitlab"
    ? `https://gitlab.com/${repo.name}`
    : `https://github.com/${repo.name}`;
}

export function webhookURL(repo: Repo): string {
  return `${API_URL}/webhooks/${repo.provider}/${repo.id}`;
}

export type Metric = { value: number; unit: string; source: "measured" | "estimated" | "na" };

export type MetricsSummary = {
  days: number;
  runs_by_status: Record<string, number>;
  succeeded_runs: number;
  lead_time_minutes: Metric;
  approval_minutes: Metric;
  runs_per_task: Metric;
  merged_pull_requests: number;
  cost_usd: Metric;
  onboarding_hours: Metric;
  success_rate_percent: Metric;
  cost_per_accepted_change_usd: Metric;
  deployments_per_day: Metric;
  merged_trend: { date: string; total: number }[];
  lead_time_trend: { date: string; minutes: number }[];
  regressions: { metric: string; current: number; baseline: number; delta_percent: number }[];
  change_failure_rate_percent: Metric;
  mttr_minutes: Metric;
  first_pass_merge_percent: Metric;
  proof_validity_percent: Metric;
  dev_runs_per_task: Metric;
  escaped_defects_per_100_merges: Metric;
  projects: { project_id: string; name: string; merged: number; incidents: number; change_failure_rate_percent: number; mttr_minutes: number; onboard_hours?: number; task_hours?: number; run_hours?: number; pr_hours?: number }[];
};

export function getMetrics(orgID: string, days = 30) {
  return request<MetricsSummary>(`/api/v1/orgs/${orgID}/metrics?days=${days}`);
}

export type Blueprint = {
  id: string;
  name: string;
  version: number;
  status: "draft" | "published";
  published_at?: string;
  spec: {
    name?: string;
    description?: string;
    stack?: { backend?: { language?: string }; frontend?: { framework?: string } };
    repository?: { layout?: string };
    starter_tasks?: { title: string }[];
  };
  created_at: string;
};

export function listBlueprints(orgID: string) {
  return request<{ data: Blueprint[] }>(`/api/v1/orgs/${orgID}/blueprints`);
}

export function createBlueprint(orgID: string, name: string, spec: unknown) {
  return request<Blueprint>(`/api/v1/orgs/${orgID}/blueprints`, {
    method: "POST",
    body: JSON.stringify({ name, spec }),
  });
}

export function publishBlueprint(orgID: string, id: string) {
  return request<Blueprint>(`/api/v1/orgs/${orgID}/blueprints/${id}/publish`, { method: "POST" });
}

export function createBlueprintVersion(orgID: string, id: string) {
  return request<Blueprint>(`/api/v1/orgs/${orgID}/blueprints/${id}/versions`, { method: "POST" });
}

export function instantiateBlueprint(orgID: string, id: string) {
  return request<{ id: string; name: string; slug: string; status: string }>(
    `/api/v1/orgs/${orgID}/blueprints/${id}/instantiate`,
    { method: "POST" },
  );
}

export function listTaskRuns(orgID: string, projectID: string, taskRef: string) {
  return request<{ data: Run[] }>(
    `/api/v1/orgs/${orgID}/projects/${projectID}/tasks/${taskRef}/runs`,
  );
}

export type Invite = {
  id: string;
  email: string;
  role: string;
  expires_at: string;
  accepted_at?: string;
};

export function listInvites(orgID: string) {
  return request<{ data: Invite[] }>(`/api/v1/orgs/${orgID}/invites`);
}

export function createInvite(orgID: string, email: string, role: string) {
  return request<Invite & { token: string }>(`/api/v1/orgs/${orgID}/invites`, {
    method: "POST",
    body: JSON.stringify({ email, role }),
  });
}

export function acceptInvite(orgID: string, token: string) {
  return request<{ org_id: string; email: string; role: string }>(
    `/api/v1/orgs/${orgID}/invites/accept`,
    { method: "POST", body: JSON.stringify({ token }) },
  );
}

export type PullRequest = {
  id: string;
  provider: "github" | "gitlab";
  number: number;
  url: string;
  state: string;
  task_id: string;
};

export function listProjectPullRequests(orgID: string, projectID: string) {
  return request<{ data: PullRequest[] }>(
    `/api/v1/orgs/${orgID}/projects/${projectID}/pull-requests`,
  );
}

export function acceptInvitePublic(token: string, name: string, password: string) {
  return request<TokenPair>(`/api/v1/invites/accept`, {
    method: "POST",
    body: JSON.stringify({ token, name, password }),
  });
}

export type SearchHit = {
  type: "project" | "task" | "component" | "run" | "rule";
  id: string;
  title: string;
  subtitle?: string | null;
  url: string;
};

export function searchTasks(orgID: string, query: string, projectID?: string) {
  const params = new URLSearchParams({ q: query });
  if (projectID) params.set("project", projectID);
  return request<{ data: SearchHit[] }>(`/api/v1/orgs/${orgID}/search?${params.toString()}`);
}

export type OrgRepo = {
  id: string;
  project_id: string;
  project_name: string;
  provider: "github" | "gitlab";
  name: string;
  default_branch: string;
  created_at: string;
};

export function listOrgRepos(orgID: string) {
  return request<{ data: OrgRepo[] }>(`/api/v1/orgs/${orgID}/repos`);
}

export type OrgRun = {
  id: string;
  project_id: string;
  task_ref: string;
  task_title: string;
  agent_type: string;
  status: string;
  created_at: string;
  finished_at?: string;
};

export function listOrgRuns(orgID: string, status?: string) {
  const params = new URLSearchParams();
  if (status) params.set("status", status);
  const suffix = params.toString() ? `?${params.toString()}` : "";
  return request<{ data: OrgRun[] }>(`/api/v1/orgs/${orgID}/runs${suffix}`);
}

export type Member = {
  user_id: string;
  email: string;
  name: string;
  role: string;
  joined_at: string;
};

export function listMembers(orgID: string) {
  return request<{ data: Member[] }>(`/api/v1/orgs/${orgID}/members`);
}

export type AuditEntry = {
  id: number;
  actor_id?: string;
  actor_kind: string;
  action: string;
  target: string;
  created_at: string;
  metadata?: Record<string, unknown>;
  prev_hash?: string;
  hash?: string;
};

export function listAudit(orgID: string) {
  return request<{ data: AuditEntry[] }>(`/api/v1/orgs/${orgID}/audit`);
}

export type Rule = {
  id: string;
  name: string;
  level: "organization" | "project" | "module" | "task";
  project_id?: string;
  created_at: string;
};

export type RuleVersion = {
  version: number;
  body: string;
  published: boolean;
  published_at?: string;
  created_at: string;
};

export function listRules(orgID: string) {
  return request<{ data: Rule[] }>(`/api/v1/orgs/${orgID}/rules`);
}

export function createRule(orgID: string, input: { name: string; level: string; body: string }) {
  return request<Rule>(`/api/v1/orgs/${orgID}/rules`, { method: "POST", body: JSON.stringify(input) });
}

export function listRuleVersions(orgID: string, ruleID: string) {
  return request<{ data: RuleVersion[] }>(`/api/v1/orgs/${orgID}/rules/${ruleID}/versions`);
}

export function saveRuleDraft(orgID: string, ruleID: string, body: string) {
  return request<{ version: number }>(`/api/v1/orgs/${orgID}/rules/${ruleID}/versions`, {
    method: "POST",
    body: JSON.stringify({ body }),
  });
}

export function publishRuleVersion(orgID: string, ruleID: string, version: number) {
  return request<{ approval_id: string; status: string; version: number }>(
    `/api/v1/orgs/${orgID}/rules/${ruleID}/versions/${version}/publish`,
    { method: "POST" },
  );
}

export type RulePublishRequest = { id: string; rule_id: string; rule: string; version: number };

export function listRulePublishApprovals(orgID: string) {
  return request<{ data: RulePublishRequest[] }>(`/api/v1/orgs/${orgID}/rules/approvals`);
}

export function decideRulePublishApproval(orgID: string, approvalID: string, decision: "approved" | "rejected") {
  return request<{ published: boolean; version?: number }>(
    `/api/v1/orgs/${orgID}/rules/approvals/${approvalID}/decide`,
    { method: "POST", body: JSON.stringify({ decision }) },
  );
}

export type Environment = {
  id: string;
  name: "dev" | "staging" | "prod";
  url: string;
  created_at: string;
};

export type Deployment = {
  id: string;
  environment_id: string;
  environment: string;
  release_ref: string;
  status: string;
  deployed_by: string;
  approved_by?: string;
  started_at?: string;
  finished_at?: string;
};

export function listEnvironments(orgID: string, projectID: string) {
  return request<{ data: Environment[] }>(
    `/api/v1/orgs/${orgID}/projects/${projectID}/environments`,
  );
}

export function createEnvironment(orgID: string, projectID: string, name: string, url: string) {
  return request<Environment>(`/api/v1/orgs/${orgID}/projects/${projectID}/environments`, {
    method: "POST",
    body: JSON.stringify({ name, url }),
  });
}

export function listDeployments(orgID: string, environmentID: string) {
  return request<{ data: Deployment[] }>(
    `/api/v1/orgs/${orgID}/environments/${environmentID}/deployments`,
  );
}

export function deployRelease(orgID: string, environmentID: string, releaseRef: string) {
  return request<Deployment>(`/api/v1/orgs/${orgID}/environments/${environmentID}/deployments`, {
    method: "POST",
    body: JSON.stringify({ release_ref: releaseRef }),
  });
}

export function approveDeployment(orgID: string, deploymentID: string) {
  return request<Deployment>(`/api/v1/orgs/${orgID}/deployments/${deploymentID}/approve`, {
    method: "POST",
  });
}

export type Provider = {
  id: string;
  name: string;
  base_url: string;
  model: string;
  has_key: boolean;
  key_hint: string;
  enabled: boolean;
  created_at: string;
};

export function listProviders(orgID: string) {
  return request<{ data: Provider[] }>(`/api/v1/orgs/${orgID}/providers`);
}

export function createProvider(
  orgID: string,
  input: { name: string; base_url: string; model: string; api_key: string },
) {
  return request<Provider>(`/api/v1/orgs/${orgID}/providers`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export type ModelPrice = {
  provider: string;
  model: string;
  input_per_1k: number;
  output_per_1k: number;
};

export function listModelPrices(orgID: string) {
  return request<{ data: ModelPrice[] }>(`/api/v1/orgs/${orgID}/model-prices`);
}

export function upsertModelPrice(
  orgID: string,
  input: { provider: string; model: string; input_per_1k: number; output_per_1k: number },
) {
  return request<ModelPrice>(`/api/v1/orgs/${orgID}/model-prices`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function rotateProviderKey(orgID: string, providerID: string, apiKey: string) {
  return request<Provider>(`/api/v1/orgs/${orgID}/providers/${providerID}/rotate`, {
    method: "POST",
    body: JSON.stringify({ api_key: apiKey }),
  });
}

export function setProviderEnabled(orgID: string, providerID: string, enabled: boolean) {
  return request<Provider>(`/api/v1/orgs/${orgID}/providers/${providerID}`, {
    method: "PATCH",
    body: JSON.stringify({ enabled }),
  });
}

export function deleteProvider(orgID: string, providerID: string) {
  return request<void>(`/api/v1/orgs/${orgID}/providers/${providerID}`, { method: "DELETE" });
}

export type DesignToken = {
  id: string;
  name: string;
  value: string;
  category: "color" | "typography" | "spacing" | "radius" | "layout" | "other";
};

export function listDesignTokens(orgID: string) {
  return request<{ data: DesignToken[] }>(`/api/v1/orgs/${orgID}/design-tokens`);
}

export function createDesignToken(orgID: string, input: { name: string; value: string; category: string }) {
  return request<DesignToken>(`/api/v1/orgs/${orgID}/design-tokens`, { method: "POST", body: JSON.stringify(input) });
}

export function updateDesignToken(orgID: string, id: string, input: { value: string; category: string }) {
  return request<DesignToken>(`/api/v1/orgs/${orgID}/design-tokens/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function deleteDesignToken(orgID: string, id: string) {
  return request<void>(`/api/v1/orgs/${orgID}/design-tokens/${id}`, { method: "DELETE" });
}

export type ComponentEntry = {
  id: string;
  name: string;
  purpose: string;
  rules: string;
  status: "draft" | "published" | "approved" | "deprecated" | "forbidden";
  category: string;
  framework: string;
  package: string;
  import_path: string;
  version: string;
  docs: string;
  example: string;
};

export function listComponentEntries(orgID: string) {
  return request<{ data: ComponentEntry[] }>(`/api/v1/orgs/${orgID}/components`);
}

export function createComponentEntry(
  orgID: string,
  input: {
    name: string; purpose: string; rules: string; status: string;
    category: string; framework: string; package: string; import_path: string;
    version: string; docs: string; example: string;
  },
) {
  return request<ComponentEntry>(`/api/v1/orgs/${orgID}/components`, { method: "POST", body: JSON.stringify(input) });
}

export function updateComponentEntry(
  orgID: string,
  id: string,
  input: {
    purpose: string; rules: string; status: string;
    category: string; framework: string; package: string; import_path: string;
    version: string; docs: string; example: string;
  },
) {
  return request<ComponentEntry>(`/api/v1/orgs/${orgID}/components/${id}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}

export function deleteComponentEntry(orgID: string, id: string) {
  return request<void>(`/api/v1/orgs/${orgID}/components/${id}`, { method: "DELETE" });
}

export function seedDesignRegistry(orgID: string) {
  return request<{ tokens: number; components: number }>(`/api/v1/orgs/${orgID}/design/seed`, { method: "POST" });
}

export type CheckResult = {
  name: string;
  status: "pending" | "running" | "passed" | "failed";
  output: string;
  duration_ms: number;
  commit_sha: string;
};

export function listPullRequestChecks(orgID: string, prID: string) {
  return request<{ data: CheckResult[] }>(`/api/v1/orgs/${orgID}/pull-requests/${prID}/checks`);
}

export function listRunChecks(orgID: string, runID: string) {
  return request<{ data: CheckResult[] }>(`/api/v1/orgs/${orgID}/runs/${runID}/checks`);
}

export function updateProjectRequiredChecks(orgID: string, projectID: string, checks: string[]) {
  return request<Project>(`/api/v1/orgs/${orgID}/projects/${projectID}/required-checks`, {
    method: "PUT",
    body: JSON.stringify({ checks }),
  });
}

export type RunArtifact = {
  kind: string;
  key: string;
  bytes: number;
  sha256: string;
  created_at: string;
};

export function listRunArtifacts(orgID: string, runID: string) {
  return request<{ data: RunArtifact[] }>(`/api/v1/orgs/${orgID}/runs/${runID}/artifacts`);
}

export function updateMemberRole(orgID: string, userID: string, role: string) {
  return request<void>(`/api/v1/orgs/${orgID}/members/${userID}`, {
    method: "PATCH",
    body: JSON.stringify({ role }),
  });
}

export function removeMember(orgID: string, userID: string) {
  return request<void>(`/api/v1/orgs/${orgID}/members/${userID}`, { method: "DELETE" });
}

export type NotificationItem = {
  id: string;
  kind: string;
  title: string;
  body: string;
  target: string;
  read_at: string | null;
  created_at: string;
};

export function listNotifications(orgID: string, unreadOnly = false) {
  return request<{ data: NotificationItem[] }>(
    `/api/v1/orgs/${orgID}/notifications?unread=${unreadOnly}`,
  );
}

export function markNotificationsRead(orgID: string, ids: string[] = []) {
  return request<void>(`/api/v1/orgs/${orgID}/notifications/read`, {
    method: "POST",
    body: JSON.stringify({ ids }),
  });
}

export type PullRequestDetail = PullRequest & {
  repo_name: string;
  linked: { task_id: string; task_ref: string; task_title: string };
};

export function getPullRequest(orgID: string, prID: string) {
  return request<PullRequestDetail>(`/api/v1/orgs/${orgID}/pull-requests/${prID}`);
}

export type RunUsage = {
  data: { provider: string; model: string; input_tokens: number; output_tokens: number; cost_usd: number }[];
  totals: { input_tokens: number; output_tokens: number; cost_usd: number };
};

export function getRunUsage(orgID: string, runID: string) {
  return request<RunUsage>(`/api/v1/orgs/${orgID}/runs/${runID}/usage`);
}

export type Org = { id: string; slug: string; name: string; max_concurrent_runs?: number; legal_hold?: boolean; region?: string };

export function getOrg(id: string) {
  return request<Org>(`/api/v1/orgs/${id}`);
}

export function renameOrg(id: string, name: string) {
  return request<Org>(`/api/v1/orgs/${id}`, { method: "PATCH", body: JSON.stringify({ name }) });
}

export function deleteOrg(id: string, confirmSlug: string) {
  return request<void>(`/api/v1/orgs/${id}`, { method: "DELETE", body: JSON.stringify({ confirm_slug: confirmSlug }) });
}

export type DesignStandard = {
  id: string;
  name: string;
  version: number;
  status: "draft" | "published";
  spec: { schema_version: string; tokens: { name: string; value: string; category: string }[]; [key: string]: unknown };
  sha256: string;
  published_at?: string | null;
  created_at: string;
};

export function listDesignStandards(orgID: string) {
  return request<{ data: DesignStandard[] }>(`/api/v1/orgs/${orgID}/design-standards`);
}

export function createDesignStandard(orgID: string, name: string, spec: DesignStandard["spec"]) {
  return request<DesignStandard>(`/api/v1/orgs/${orgID}/design-standards`, {
    method: "POST",
    body: JSON.stringify({ name, spec }),
  });
}

export function createDesignStandardVersion(orgID: string, id: string, spec?: DesignStandard["spec"]) {
  return request<DesignStandard>(`/api/v1/orgs/${orgID}/design-standards/${id}/versions`, {
    method: "POST",
    body: spec ? JSON.stringify({ spec }) : undefined,
  });
}

export function publishDesignStandard(orgID: string, id: string) {
  return request<DesignStandard>(`/api/v1/orgs/${orgID}/design-standards/${id}/publish`, { method: "POST" });
}

export type ProjectStandard = {
  id: string;
  name: string;
  version: number;
  sha256: string;
  published_at?: string | null;
};

export function getProjectStandard(orgID: string, projectID: string) {
  return request<{ design_standard: ProjectStandard | null }>(
    `/api/v1/orgs/${orgID}/projects/${projectID}/standards`,
  );
}

export function setProjectStandard(orgID: string, projectID: string, designStandardID: string) {
  return request<Project>(`/api/v1/orgs/${orgID}/projects/${projectID}/standards`, {
    method: "PUT",
    body: JSON.stringify({ design_standard_id: designStandardID || null }),
  });
}

export type ProjectComponent = {
  id: string;
  name: string;
  category: string;
  status: string;
  import_path: string;
  package: string;
  version: string;
  mode: string;
  reason: string;
};

export function listProjectComponents(orgID: string, projectID: string) {
  return request<{ data: ProjectComponent[] }>(`/api/v1/orgs/${orgID}/projects/${projectID}/components`);
}

export type ComponentSelection = { component_id: string; mode: "allow" | "exception"; reason: string };

export function setProjectComponents(orgID: string, projectID: string, components: ComponentSelection[]) {
  return request<{ count: number }>(`/api/v1/orgs/${orgID}/projects/${projectID}/components`, {
    method: "PUT",
    body: JSON.stringify({ components }),
  });
}

export type TaskAttachment = {
  id: string;
  filename: string;
  content_type: string;
  size_bytes: number;
  sha256: string;
  created_at: string;
};

function attachmentBase(orgID: string, projectID: string, taskID: string) {
  return `/api/v1/orgs/${orgID}/projects/${projectID}/tasks/${taskID}/attachments`;
}

export function listTaskAttachments(orgID: string, projectID: string, taskID: string) {
  return request<{ data: TaskAttachment[] }>(attachmentBase(orgID, projectID, taskID));
}

export function uploadTaskAttachment(orgID: string, projectID: string, taskID: string, file: File) {
  const body = new FormData();
  body.append("file", file);
  return request<TaskAttachment>(attachmentBase(orgID, projectID, taskID), { method: "POST", body });
}

export async function downloadTaskAttachment(orgID: string, projectID: string, taskID: string, id: string) {
  const res = await send(`${attachmentBase(orgID, projectID, taskID)}/${id}`);
  if (!res.ok) throw await toError(res);
  return res.blob();
}

export function setProjectE2E(orgID: string, projectID: string, command: string) {
  return request<Project>(`/api/v1/orgs/${orgID}/projects/${projectID}/e2e`, {
    method: "PUT",
    body: JSON.stringify({ command }),
  });
}

export type PreflightCheck = {
  key: string;
  label: string;
  status: "ok" | "warn" | "fail";
  detail: string;
  fix?: string;
};

export function getProjectPreflight(orgID: string, projectID: string) {
  return request<{ ready: boolean; status: string; checks: PreflightCheck[] }>(
    `/api/v1/orgs/${orgID}/projects/${projectID}/preflight`,
  );
}

export function setProjectVisual(orgID: string, projectID: string, command: string) {
  return request<Project>(`/api/v1/orgs/${orgID}/projects/${projectID}/visual`, {
    method: "PUT",
    body: JSON.stringify({ command }),
  });
}

export type GitHubAppStatus = {
  configured: boolean;
  github_app?: { app_id: number; slug: string; install_url: string; installation_id: number; installed: boolean };
};

export function getGitHubApp(orgID: string) {
  return request<GitHubAppStatus>(`/api/v1/orgs/${orgID}/github-app`);
}

export function createGitHubAppManifest(orgID: string) {
  return request<{ action_url: string; state: string; manifest: unknown }>(
    `/api/v1/orgs/${orgID}/github-app/manifest`,
    { method: "POST" },
  );
}

export function setGitHubInstallation(orgID: string, installationID: number) {
  return request<GitHubAppStatus["github_app"]>(`/api/v1/orgs/${orgID}/github-app/installation`, {
    method: "PUT",
    body: JSON.stringify({ installation_id: installationID }),
  });
}

export function setProjectMCP(orgID: string, projectID: string, config: string) {
  return request<Project>(`/api/v1/orgs/${orgID}/projects/${projectID}/mcp`, {
    method: "PUT",
    body: JSON.stringify({ config }),
  });
}

export function protectProjectBranch(orgID: string, projectID: string) {
  return request<{ protected: boolean; repo: string; branch: string; checks: string[] }>(
    `/api/v1/orgs/${orgID}/projects/${projectID}/protection`,
    { method: "POST" },
  );
}

export async function getRunVisual(orgID: string, runID: string, name: string): Promise<Blob> {
  const res = await send(`/api/v1/orgs/${orgID}/runs/${runID}/visual?name=${encodeURIComponent(name)}`, {});
  if (!res.ok) throw await toError(res);
  return res.blob();
}

export async function getVisualBaseline(orgID: string, projectID: string, name: string): Promise<Blob> {
  let res = await send(`/api/v1/orgs/${orgID}/projects/${projectID}/visual-baseline?name=${encodeURIComponent(name)}`, {});
  if (res.status === 401 && (await refreshSession())) {
    res = await send(`/api/v1/orgs/${orgID}/projects/${projectID}/visual-baseline?name=${encodeURIComponent(name)}`, {});
  }
  if (!res.ok) throw await toError(res);
  return res.blob();
}

export type MCPTool = { server: string; name: string; description?: string };

export function getMCPTools(orgID: string, projectID: string) {
  return request<{ tools: MCPTool[]; run_id: string }>(
    `/api/v1/orgs/${orgID}/projects/${projectID}/mcp-tools`,
  );
}

export type MergeQueueItem = {
  id: number;
  number: number;
  status: string;
  error: string;
  run_id: string;
  task_id: string;
  created_at: string;
};

export function listMergeQueue(orgID: string, projectID: string) {
  return request<{ items: MergeQueueItem[] }>(`/api/v1/orgs/${orgID}/projects/${projectID}/merge-queue`);
}

export type PreviewEnvironment = {
  id: string; name: string; image: string; port: number; status: string;
  url: string; error: string; expires_at: string; created_at: string;
};

export function listPreviews(orgID: string, projectID: string) {
  return request<{ items: PreviewEnvironment[] }>(`/api/v1/orgs/${orgID}/projects/${projectID}/previews`);
}

export function createPreview(orgID: string, projectID: string, input: { name?: string; image: string; port: number; ttl_minutes: number }) {
  return request<PreviewEnvironment>(`/api/v1/orgs/${orgID}/projects/${projectID}/previews`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function destroyPreview(orgID: string, projectID: string, previewID: string) {
  return request<void>(`/api/v1/orgs/${orgID}/projects/${projectID}/previews/${previewID}`, { method: "DELETE" });
}

export type ProjectMember = { user_id: string; role: string; created_at?: string };

export function listProjectMembers(orgID: string, projectID: string) {
  return request<{ items: ProjectMember[] }>(`/api/v1/orgs/${orgID}/projects/${projectID}/members`);
}

export function setProjectMember(orgID: string, projectID: string, userID: string, role: string) {
  return request<ProjectMember>(`/api/v1/orgs/${orgID}/projects/${projectID}/members/${userID}`, {
    method: "PUT",
    body: JSON.stringify({ role }),
  });
}

export function removeProjectMember(orgID: string, projectID: string, userID: string) {
  return request<void>(`/api/v1/orgs/${orgID}/projects/${projectID}/members/${userID}`, { method: "DELETE" });
}

export function setProjectPolicy(orgID: string, projectID: string, maxDiffBytes: number, forbiddenPaths: string[]) {
  return request<Project>(`/api/v1/orgs/${orgID}/projects/${projectID}/policy`, {
    method: "PUT",
    body: JSON.stringify({ max_diff_bytes: maxDiffBytes, forbidden_paths: forbiddenPaths }),
  });
}

export async function exportAudit(orgID: string): Promise<{ blob: Blob; signature: string | null }> {
  const path = `/api/v1/orgs/${orgID}/audit/export`;
  let res = await send(path, {});
  if (res.status === 401 && (await refreshSession())) {
    res = await send(path, {});
  }
  if (!res.ok) throw await toError(res);
  return { blob: await res.blob(), signature: res.headers.get("X-AIP-Audit-Signature") };
}

export function setProjectChangeWindow(orgID: string, projectID: string, window: string) {
  return request<Project>(`/api/v1/orgs/${orgID}/projects/${projectID}/policy/window`, {
    method: "PUT",
    body: JSON.stringify({ window }),
  });
}

export type APIKey = { id: string; name: string; role: string; created_at: string; revoked_at?: string; last_used_at?: string; expires_at?: string | null };

export function listAPIKeys(orgID: string) {
  return request<{ items: APIKey[] }>(`/api/v1/orgs/${orgID}/api-keys`);
}

export function createAPIKey(orgID: string, name: string, role: string, expiresInDays: number) {
  return request<APIKey & { token: string }>(`/api/v1/orgs/${orgID}/api-keys`, {
    method: "POST",
    body: JSON.stringify({ name, role, expires_in_days: expiresInDays }),
  });
}

export function revokeAPIKey(orgID: string, id: string) {
  return request<void>(`/api/v1/orgs/${orgID}/api-keys/${id}`, { method: "DELETE" });
}

export type AuditWitness = { id: string; object_key: string; row_count: number; last_id: number; last_hash: string; created_at: string };

export function writeAuditWitness(orgID: string) {
  return request<AuditWitness>(`/api/v1/orgs/${orgID}/audit/witness`, { method: "POST" });
}

export function verifyAuditWitness(orgID: string) {
  return request<{ witness: AuditWitness; valid: boolean }>(`/api/v1/orgs/${orgID}/audit/witness`);
}

export function setLegalHold(orgID: string, hold: boolean) {
  return request<Org>(`/api/v1/orgs/${orgID}/legal-hold`, { method: "PUT", body: JSON.stringify({ hold }) });
}

export function setProjectAllowedModels(orgID: string, projectID: string, models: string[]) {
  return request<Project>(`/api/v1/orgs/${orgID}/projects/${projectID}/policy/models`, {
    method: "PUT",
    body: JSON.stringify({ models }),
  });
}

export function setProjectAllowedTools(orgID: string, projectID: string, tools: string[]) {
  return request<Project>(`/api/v1/orgs/${orgID}/projects/${projectID}/policy/tools`, {
    method: "PUT",
    body: JSON.stringify({ tools }),
  });
}

export function setProjectRequiredApprovals(orgID: string, projectID: string, required: number) {
  return request<Project>(`/api/v1/orgs/${orgID}/projects/${projectID}/policy/approvals`, {
    method: "PUT",
    body: JSON.stringify({ required }),
  });
}

export function scaffoldComponent(orgID: string, projectID: string, name: string, kind: string) {
  return request<Task>(`/api/v1/orgs/${orgID}/projects/${projectID}/scaffold/component`, {
    method: "POST",
    body: JSON.stringify({ name, kind }),
  });
}

export async function exportDesignTokens(orgID: string, format: "css" | "scss" | "ts"): Promise<string> {
  const path = `/api/v1/orgs/${orgID}/design-tokens/export?format=${format}`;
  let res = await send(path, {});
  if (res.status === 401 && (await refreshSession())) {
    res = await send(path, {});
  }
  if (!res.ok) throw await toError(res);
  return res.text();
}

export type ImportComponent = { name: string; import_path?: string; package?: string; framework?: string; category?: string; purpose?: string; rules?: string; version?: string };

export function importComponents(orgID: string, components: ImportComponent[]) {
  return request<{ created: number; skipped: number }>(`/api/v1/orgs/${orgID}/components/import`, {
    method: "POST",
    body: JSON.stringify({ components }),
  });
}

export function setProjectPluginChecks(orgID: string, projectID: string, checks: string) {
  return request<Project>(`/api/v1/orgs/${orgID}/projects/${projectID}/policy/plugins`, {
    method: "PUT",
    body: JSON.stringify({ checks }),
  });
}

export function importFigmaComponents(orgID: string, fileKey: string) {
  return request<{ created: number; skipped: number }>(`/api/v1/orgs/${orgID}/components/figma`, {
    method: "POST",
    body: JSON.stringify({ file_key: fileKey }),
  });
}

export type Incident = { id: string; project_id?: string | null; title: string; severity: string; status: string; opened_at: string; resolved_at?: string };

export function listIncidents(orgID: string) {
  return request<{ data: Incident[] }>(`/api/v1/orgs/${orgID}/incidents`);
}

export function createIncident(orgID: string, input: { title: string; severity: string; project_id?: string }) {
  return request<Incident>(`/api/v1/orgs/${orgID}/incidents`, { method: "POST", body: JSON.stringify(input) });
}

export function resolveIncident(orgID: string, id: string) {
  return request<Incident>(`/api/v1/orgs/${orgID}/incidents/${id}/resolve`, { method: "POST" });
}

export function deleteProject(orgID: string, projectID: string, confirmName: string) {
  return request<void>(`/api/v1/orgs/${orgID}/projects/${projectID}`, {
    method: "DELETE",
    body: JSON.stringify({ confirm_name: confirmName }),
  });
}

export function setOrgRegion(id: string, region: string) {
  return request<Org>(`/api/v1/orgs/${id}/region`, { method: "PUT", body: JSON.stringify({ region }) });
}

export type ScimToken = { id: string; name: string; created_at: string; revoked_at?: string | null };

export function listScimTokens(orgID: string) {
  return request<{ data: ScimToken[] }>(`/api/v1/orgs/${orgID}/scim-tokens`);
}

export function createScimToken(orgID: string, name: string) {
  return request<ScimToken & { token: string }>(`/api/v1/orgs/${orgID}/scim-tokens`, {
    method: "POST",
    body: JSON.stringify({ name }),
  });
}

export function revokeScimToken(orgID: string, id: string) {
  return request<void>(`/api/v1/orgs/${orgID}/scim-tokens/${id}`, { method: "DELETE" });
}

export type SSOConfig = { entity_id: string; metadata_url: string; sso_url: string; certificate_pem: string; enabled: boolean; updated_at: string };

export function getSSO(orgID: string) {
  return request<SSOConfig>(`/api/v1/orgs/${orgID}/sso`);
}

export function saveSSO(orgID: string, cfg: { entity_id: string; metadata_url: string; sso_url: string; certificate_pem: string; enabled: boolean }) {
  return request<SSOConfig>(`/api/v1/orgs/${orgID}/sso`, { method: "PUT", body: JSON.stringify(cfg) });
}

export function fetchSSO(orgID: string, metadataURL: string) {
  return request<SSOConfig>(`/api/v1/orgs/${orgID}/sso/fetch`, {
    method: "POST",
    body: JSON.stringify({ metadata_url: metadataURL }),
  });
}

export type OrgCrypto = { kms_url: string; key_id: string; status: string; updated_at: string };

export function getOrgCrypto(orgID: string) {
  return request<OrgCrypto>(`/api/v1/orgs/${orgID}/crypto`);
}

export function setOrgCrypto(orgID: string, kmsURL: string, keyID: string, token: string) {
  return request<OrgCrypto>(`/api/v1/orgs/${orgID}/crypto`, {
    method: "PUT",
    body: JSON.stringify({ kms_url: kmsURL, key_id: keyID, token }),
  });
}

export function disableOrgCrypto(orgID: string) {
  return request<void>(`/api/v1/orgs/${orgID}/crypto`, { method: "DELETE" });
}

export type AgentPlugin = {
  id: string;
  name: string;
  version: string;
  image: string;
  command: string;
  agent_types: string[];
  created_at: string;
};

export function listAgentPlugins(orgID: string) {
  return request<{ items: AgentPlugin[] }>(`/api/v1/orgs/${orgID}/agent-plugins`);
}

export function createAgentPlugin(orgID: string, input: { name: string; version: string; image: string; command: string; agent_types: string[] }) {
  return request<AgentPlugin>(`/api/v1/orgs/${orgID}/agent-plugins`, {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function selectAgentPlugin(orgID: string, projectID: string, pluginID: string) {
  return request<void>(`/api/v1/orgs/${orgID}/projects/${projectID}/agent-plugin`, {
    method: "PUT",
    body: JSON.stringify({ plugin_id: pluginID }),
  });
}

export type DRStatus = {
  wal_failed: number;
  wal_archived: number;
  wal_age_seconds: number;
  base_age_seconds: number;
  base_backup: string;
  wal_ok: boolean;
  base_ok: boolean;
  restore_succeeded: number;
  restore_failed: number;
  last_restore_age_seconds: number;
  avg_restore_seconds: number;
  offsite_age_seconds: number;
};

export type RestoreRequest = {
  id: string;
  target_time: string;
  status: string;
  detail: string;
  created_at: string;
};

export function getDRStatus(orgID: string) {
  return request<DRStatus>(`/api/v1/orgs/${orgID}/dr`);
}

export function listRestoreRequests(orgID: string) {
  return request<{ data: RestoreRequest[] }>(`/api/v1/orgs/${orgID}/restores`);
}

export function createRestoreRequest(orgID: string, targetTime: string) {
  return request<{ id: string; status: string }>(`/api/v1/orgs/${orgID}/restores`, {
    method: "POST",
    body: JSON.stringify({ target_time: targetTime }),
  });
}
