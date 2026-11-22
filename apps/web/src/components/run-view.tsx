"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";

import { useQuery } from "@tanstack/react-query";

import { DiffViewer } from "@/components/diff-viewer";
import { NextAction, type NextActionItem } from "@/components/next-action";
import { HelpTip } from "@/components/ui/help-tip";
import { PromptDialog } from "@/components/ui/prompt-dialog";
import { Button } from "@/components/ui/button";
import {
  ApiError,
  ensureSession,
  decideApproval,
  getRun,
  listApprovals,
  getRunUsage,
  getRunVisual,
  getVisualBaseline,
  listRunArtifacts,
  startRun,
  retryRun,
  fixReview,
  troubleshootRun,
  listReviewFindings,
  decideReviewFinding,
  getTask,
  createTask,
  streamRunEvents,
  transitionTask,
  type Approval,
  type Run,
  type RunArtifact,
  type ReviewFindingRecord,
  type Task,
  type RunEvent,
} from "@/lib/api";
import type { Claims } from "@/lib/auth";

const statusStyles: Record<string, string> = {
  queued: "bg-bg-hover text-text-muted",
  running: "bg-brand-subtle text-brand",
  awaiting_approval: "bg-status-warning/10 text-status-warning",
  succeeded: "bg-status-success/10 text-status-success",
  failed: "bg-status-danger/10 text-status-danger",
  canceled: "bg-bg-hover text-text-muted",
};

export function RunView({ projectId, runId }: { projectId: string; runId: string }) {
  const router = useRouter();
  const [claims, setClaims] = useState<Claims | null | undefined>(undefined);
  const [run, setRun] = useState<Run | null>(null);
  const [events, setEvents] = useState<RunEvent[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [retryingDev, setRetryingDev] = useState(false);
  const [troubleshooting, setTroubleshooting] = useState(false);
  const [fixing, setFixing] = useState(false);
  const [approval, setApproval] = useState<Approval | null>(null);
  const [rejectReason, setRejectReason] = useState("");
  const [showReject, setShowReject] = useState(false);
  const [deciding, setDeciding] = useState(false);
  const [artifacts, setArtifacts] = useState<RunArtifact[]>([]);
  const [visualName, setVisualName] = useState("");
  const [baselineURL, setBaselineURL] = useState<string | null>(null);
  const [currentURL, setCurrentURL] = useState<string | null>(null);
  const [visualError, setVisualError] = useState<string | null>(null);
  const [visualLoading, setVisualLoading] = useState(false);
  const usage = useQuery({
    queryKey: ["run-usage", claims?.org, runId],
    enabled: Boolean(claims?.org),
    queryFn: () => getRunUsage(claims!.org, runId),
  });
  // The findings' lifecycle: a blocker stays open until a human dismisses it
  // or a later review stops reporting it.
  const findingsQuery = useQuery({
    queryKey: ["review-findings", claims?.org, projectId, run?.task_id],
    enabled: Boolean(claims?.org && run?.task_id),
    queryFn: () => listReviewFindings(claims!.org, projectId, run!.task_id!),
    // A fix run finishes minutes later and the following review marks a finding
    // fixed; the list must reflect it without a manual refresh.
    refetchInterval: 8000,
  });
  // A closed task cannot take a fix: its branch is gone with the pull request,
  // so a finding becomes a follow-up task instead.
  const taskQuery = useQuery({
    queryKey: ["task", claims?.org, projectId, run?.task_id],
    enabled: Boolean(claims?.org && run?.task_id),
    queryFn: () => getTask(claims!.org, projectId, run!.task_id!),
    refetchInterval: 15000,
  });
  const taskClosed = taskQuery.data?.status === "done" || taskQuery.data?.status === "canceled";
  const findingRecords = findingsQuery.data?.data ?? [];
  const openFindings = findingRecords.filter((f) => f.status === "open");
  const resolvedFindings = findingRecords.filter((f) => f.status !== "open");
  const openBlockers = findingRecords.filter((f) => f.status === "open" && f.severity === "blocker").length;
  const retries = events.filter((event) => event.type === "run.retry" || event.type === "test.flaky_recovered");
  const manifestEvent = events.find((event) => event.type === "context.manifest");
  const manifestSources = (manifestEvent?.data.payload?.sources ?? []) as Array<{
    kind: string;
    id: string;
    version: string;
    sha256?: string;
  }>;
  const manifestPrecedence = (manifestEvent?.data.payload?.precedence ?? []) as string[];
  const snapshotEvent = events.find((event) => event.type === "rules.snapshot");
  const snapshotRules = (snapshotEvent?.data.payload?.rules ?? []) as Array<{
    level?: string;
    name?: string;
    rule_id?: string;
    version?: number;
  }>;
  const [dismissing, setDismissing] = useState<ReviewFindingRecord | null>(null);
  const [decisionNote, setDecisionNote] = useState<string | null>(null);
  const [developmentRunID, setDevelopmentRunID] = useState<string | null>(null);
  const logRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    ensureSession().then(async (result) => {
      if (!result) {
        router.replace("/login");
        return;
      }
      setClaims(result);
      try {
        setRun(await getRun(result.org, runId));
      } catch {
        setError("Could not load the run");
      }
    });
  }, [router, runId]);

  useEffect(() => {
    if (!claims?.org) return;
    listRunArtifacts(claims.org, runId)
      .then((result) => setArtifacts(result.data))
      .catch(() => setArtifacts([]));
  }, [claims?.org, runId, run?.status]);

  useEffect(() => {
    if (!claims) return;
    const controller = new AbortController();

    streamRunEvents(
      claims.org,
      runId,
      (event) => {
        setEvents((current) => [...current, event]);
        const status = event.data.payload?.status;
        if (typeof status === "string") {
          setRun((current) => (current ? { ...current, status } : current));
          // A planning run parks in awaiting_approval, which is not terminal:
          // the stream stays open, so the plan only arrived on reload. Refresh
          // the run here so the plan and the approval panel appear live.
          if (status === "awaiting_approval") {
            getRun(claims.org, runId)
              .then((next) => {
                if (!controller.signal.aborted) setRun(next);
              })
              .catch(() => {
                // keep the last known state
              });
          }
        }
      },
      controller.signal,
    )
      .then(async () => {
        if (controller.signal.aborted) return;
        // The stream closes on a terminal state; refresh to pick up plan/diff.
        try {
          setRun(await getRun(claims.org, runId));
        } catch {
          // keep the last known state
        }
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted) return;
        setError(err instanceof Error ? err.message : "The event stream stopped");
      });

    return () => controller.abort();
  }, [claims, runId]);

  useEffect(() => {
    logRef.current?.scrollTo({ top: logRef.current.scrollHeight });
  }, [events]);

  useEffect(() => {
    if (!claims) {
      setApproval(null);
      return;
    }
    listApprovals(claims.org)
      .then(({ data }) => setApproval(data.find((item) => item.run_id === runId) ?? null))
      .catch(() => setApproval(null));
  }, [claims, run?.status, runId]);

  async function decide(decision: "approved" | "rejected") {
    if (!claims || !approval) return;
    if (decision === "rejected" && rejectReason.trim() === "") {
      setError("A reason is required when rejecting a plan");
      return;
    }
    setDeciding(true);
    setError(null);
    try {
      const updated = await decideApproval(claims.org, approval.id, decision, rejectReason.trim());
      setRun(updated);
      if (updated.development_run_id) setDevelopmentRunID(updated.development_run_id);
      setApproval(null);
      setShowReject(false);
      setDecisionNote(decision === "approved" ? "Plan approved — the task moved to in progress." : "Plan rejected — the task is blocked.");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not record the decision");
    } finally {
      setDeciding(false);
    }
  }

  async function retryDevelopment() {
    if (!claims || !run) return;
    setRetryingDev(true);
    setError(null);
    try {
      const next = await retryRun(claims.org, run.id);
      router.push(`/projects/${run.project_id}/runs/${next.id}`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not retry the development run");
    } finally {
      setRetryingDev(false);
    }
  }

  async function createFollowUp(finding: ReviewFindingRecord) {
    if (!claims?.org) return;
    try {
      const title = finding.message.split("\n")[0].slice(0, 90);
      const created = await createTask(claims.org, projectId, {
        title,
        description: finding.message + "\n\nFollow-up from the review of " + (taskQuery.data?.ref ?? "the task") + ".",
        acceptance: [],
        priority: "low",
      });
      setDecisionNote(`Follow-up ${created.ref} created — it carries the finding.`);
      await router.push(`/projects/${projectId}`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not create the follow-up task");
    }
  }

  async function fixFinding(finding: ReviewFindingRecord) {
    if (!claims?.org) return;
    try {
      // The finding belongs to the review that reported it, not to the run page
      // the human happens to be on.
      const next = await fixReview(claims.org, finding.run_id, [finding.id]);
      setDevelopmentRunID(next.id);
      // Open the new run so the stream and the status are visible immediately.
      router.push(`/projects/${projectId}/runs/${next.id}`);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Could not start the fix run";
      setError(message.includes("already active") ? "A run is already active for this task — wait for it to finish, then fix the next finding." : message);
    }
  }

  async function resolveFinding(finding: ReviewFindingRecord, decision: "dismiss" | "reopen", reason = "") {
    if (!claims?.org) return;
    try {
      await decideReviewFinding(claims.org, projectId, finding.task_id, finding.id, decision, reason);
      await findingsQuery.refetch();
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not update the finding");
    }
  }

  async function troubleshoot() {
    if (!claims || !run) return;
    setTroubleshooting(true);
    setError(null);
    try {
      const next = await troubleshootRun(claims.org, run.id);
      router.push(`/projects/${run.project_id}/runs/${next.id}`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not start the troubleshoot run");
    } finally {
      setTroubleshooting(false);
    }
  }

  async function fixAllOpen() {
    if (!claims || openFindings.length === 0) return;
    setFixing(true);
    setError(null);
    try {
      // One run carries every open finding: one review after it, not one per fix.
      const next = await fixReview(claims.org, openFindings[0].run_id, openFindings.map((f) => f.id));
      setDevelopmentRunID(next.id);
      router.push(`/projects/${projectId}/runs/${next.id}`);
    } catch (err) {
      const message = err instanceof ApiError ? err.message : "Could not start the fix run";
      setError(message.includes("already active") ? "A run is already active for this task — wait for it to finish." : message);
    } finally {
      setFixing(false);
    }
  }

  async function fixReviewFindings() {
    if (!claims || !run) return;
    setFixing(true);
    setError(null);
    try {
      const next = await fixReview(claims.org, run.id);
      router.push(`/projects/${run.project_id}/runs/${next.id}`);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not start the fix run");
    } finally {
      setFixing(false);
    }
  }

  const nextActions: NextActionItem[] = [];
  let nextTitle = "";
  let nextDescription = "";
  if (run && (approval || run.status === "awaiting_approval")) {
    nextTitle = "Approve the plan";
    nextDescription = "The planning run is waiting for a human decision. Approving starts development; rejecting blocks the task.";
    nextActions.push({ label: "Approve plan", onClick: () => void decide("approved") });
    nextActions.push({ label: "Reject plan", variant: "secondary", onClick: () => setShowReject(true) });
  } else if (run?.status === "failed") {
    if (run.agent_type === "review") {
      nextTitle = "Fix the review findings";
      nextDescription = "The review run found blockers. A fix run addresses them and re-runs the gates.";
      nextActions.push({ label: "Fix review findings", onClick: () => void fixReviewFindings() });
    } else {
      nextTitle = "Retry the development run";
      nextDescription = "The run failed. Retrying re-queues it and puts the task back in progress.";
      nextActions.push({ label: "Retry development", loading: retryingDev, onClick: () => void retryDevelopment() });
      nextActions.push({ label: "Troubleshoot", variant: "secondary", loading: troubleshooting, onClick: () => void troubleshoot() });
    }
  } else if (run?.status === "canceled") {
    nextTitle = run.fix_of_run_id ? "Restart the canceled fix" : "Restart the canceled run";
    nextDescription = "The run was canceled. Restarting re-queues the same approved plan; nothing is lost.";
    nextActions.push({ label: "Retry", loading: retryingDev, onClick: () => void retryDevelopment() });
  } else if (run?.status === "succeeded") {
    nextTitle = "The gates passed — review and merge";
    nextDescription = "Open the project's pull requests to review the diff. A human merges; the platform never merges for you.";
    nextActions.push({ label: "Go to the project", onClick: () => router.push(`/projects/${projectId}`) });
  }

  return (
    <div>
      <nav aria-label="Breadcrumb" className="mb-1 text-xs text-text-muted">
        <Link href="/projects" className="hover:text-text-secondary">
          Projects
        </Link>
        <span> / </span>
        <Link href={`/projects/${projectId}`} className="hover:text-text-secondary">
          Project
        </Link>
        <span> / </span>
        <span className="text-text-secondary">Run</span>
      </nav>

      <header className="mb-6 flex items-end justify-between gap-4">
        <div>
          <h1 className="font-mono text-[20px] font-semibold">Run {runId.slice(0, 8)}</h1>
          <p className="mt-1 text-[13px] text-text-secondary">
            {run?.agent_type ?? "agent"} · {run?.model || "default model"}
          </p>
        </div>
        <span
          className={`rounded-md px-2.5 py-1 text-xs font-medium ${
            statusStyles[run?.status ?? "queued"] ?? "bg-bg-hover text-text-muted"
          }`}
        >
          {run?.status ?? "queued"}
        </span>
      </header>

      {/* The scroll container has a top padding; a sticky card at top-0 stops
          inside it, leaving a visible band. Offset by exactly that padding. */}
      {run?.status === "queued" ? (
        <p role="status" className="mb-4 rounded-md border border-brand/40 bg-brand-subtle/40 px-3 py-2 text-xs text-text-secondary">
          Queued — the worker picks it up in a moment. The log fills as it runs.
        </p>
      ) : null}

      {nextActions.length > 0 ? (
        <div className="sticky top-[calc(var(--content-pad-top)*-1)] z-20 mb-9">
          <NextAction title={nextTitle} description={nextDescription} actions={nextActions} />
        </div>
      ) : null}

      {run ? (
        <>
          <section className="run-summary-grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4" aria-label="Execution summary">
            <div className="run-summary-card"><span>Execution status</span><strong className="capitalize">{run.status.replaceAll("_", " ")}</strong><small>{events.length} live events received</small></div>
            <div className="run-summary-card"><span>Agent</span><strong>{run.agent_type || "Development"}</strong><small>{run.model || "Default model"}</small></div>
            <div className="run-summary-card"><span>Estimated / recorded cost</span><strong>${(usage.data?.totals.cost_usd ?? 0).toFixed(4)}</strong><small>{usage.data?.totals.input_tokens ?? 0} input tokens</small></div>
            <div className="run-summary-card"><span>Review gate <HelpTip term="review" /></span><strong>{approval ? "Needs your review" : run.status === "succeeded" ? "Ready" : "Tracked"}</strong><small>{approval ? "Human approval required" : "Changes remain auditable"}</small></div>
          </section>
          {run.trace_id ? (
            <p className="mt-2 text-xs text-text-secondary">
              Trace:{" "}
              {process.env.NEXT_PUBLIC_TRACE_UI_URL ? (
                <a className="text-brand hover:underline" href={process.env.NEXT_PUBLIC_TRACE_UI_URL.replace("{trace_id}", run.trace_id)} target="_blank" rel="noreferrer">
                  {run.trace_id}
                </a>
              ) : (
                <code className="font-mono">{run.trace_id}</code>
              )}
            </p>
          ) : null}
          <section className="run-stage-track" aria-label="Execution stages">
            {[
              { label: "Plan", match: /plan|rules\.snapshot/i },
              { label: "Approval", match: /approval|approved/i },
              { label: "Develop", match: /develop|write|tool/i },
              { label: "Test", match: /test|check|build/i },
              { label: "Review", match: /review|pull.?request|merge/i },
            ].map((stage, index, all) => {
              const found = events.some((event) => stage.match.test(event.type));
              const terminal = ["succeeded", "failed", "canceled"].includes(run.status);
              const done = found || (run.status === "succeeded" && index < all.length);
              const current = !done && !terminal && (index === 0 || all.slice(0, index).some((prior) => events.some((event) => prior.match.test(event.type))));
              return <div key={stage.label} className={`run-stage ${done ? "run-stage-complete" : current ? "run-stage-current" : ""}`}><span className="run-stage-marker">{done ? "✓" : index + 1}</span><span><b>{stage.label}</b><small>{done ? "Recorded" : current ? "In progress" : "Waiting"}</small></span></div>;
            })}
          </section>
        </>
      ) : null}

      {run?.status === "failed" ? (
        <section aria-label="Run failure" className="mb-6 rounded-lg border border-status-danger/40 bg-status-danger/10 p-4">
          <header className="mb-2 flex flex-wrap items-center justify-between gap-3">
            <div>
              <h2 className="text-sm font-semibold text-status-danger">Run failed</h2>
              <p className="text-xs text-text-secondary">{run.failure_code || "unknown failure"}</p>
            </div>
            {claims ? (
              <p className="text-xs text-text-muted">The next action is below.</p>
            ) : null}
          </header>
          {run.failure_detail ? (
            <pre className="max-h-60 overflow-auto whitespace-pre-wrap rounded-md border border-border bg-bg-surface p-3 font-mono text-[11px] text-text-secondary">
              {run.failure_detail}
            </pre>
          ) : null}
        </section>
      ) : null}

      {error ? (
        <div role="alert" className="mb-4 rounded-lg border border-status-danger/30 bg-status-danger/10 p-3 text-sm text-status-danger">
          {error}
        </div>
      ) : null}

      {decisionNote ? (
        <div className="mb-4 flex items-center justify-between rounded-lg border border-status-success/30 bg-status-success/10 p-3 text-sm text-status-success">
          <span>{decisionNote}</span>
          {developmentRunID ? (
            <Link href={`/projects/${projectId}/runs/${developmentRunID}`} className="font-medium underline">
              Open development run →
            </Link>
          ) : null}
        </div>
      ) : null}

      {approval?.type === "pr" ? (
        <section className="mb-6 rounded-lg border border-ai/40 bg-ai-subtle/40 p-4">
          <header className="mb-2 flex items-center justify-between">
            <div>
              <h2 className="text-sm font-semibold">Pull request approval</h2>
              <p className="text-xs text-text-secondary">
                Merge is blocked until a human approves this pull/merge request.
              </p>
              {openBlockers > 0 ? (
                <p className="mt-1 text-xs text-status-warning">
                  {openBlockers} open review blocker{openBlockers === 1 ? "" : "s"} — fix or dismiss them before the merge.
                </p>
              ) : null}
            </div>
            <div className="flex gap-2">
              <Button loading={deciding} disabled={openBlockers > 0} onClick={() => decide("approved")}>
                Approve merge
              </Button>
              <Button variant="secondary" disabled={deciding} onClick={() => setShowReject((v) => !v)}>
                Reject
              </Button>
            </div>
          </header>
          {approval.url ? (
            <a
              href={approval.url}
              target="_blank"
              rel="noreferrer"
              className="text-sm text-brand hover:underline"
            >
              View pull request #{approval.number} →
            </a>
          ) : null}
          <label htmlFor="decision-note" className="mt-3 block text-xs font-medium text-text-secondary">
            Decision note
          </label>
          <textarea
            id="decision-note"
            value={rejectReason}
            onChange={(e) => setRejectReason(e.target.value)}
            rows={2}
            placeholder="Required when the acceptance proof contains manual evidence"
            className="mt-1 w-full rounded-md border border-border bg-bg-surface px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand"
          />
          {showReject ? (
            <div className="mt-3">
              <textarea
                aria-label="Reason for rejection"
                value={rejectReason}
                onChange={(e) => setRejectReason(e.target.value)}
                rows={2}
                className="w-full rounded-md border border-border bg-bg-surface px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand"
              />
              <Button variant="danger" className="mt-2" loading={deciding} onClick={() => decide("rejected")}>
                Confirm rejection
              </Button>
            </div>
          ) : null}
        </section>
      ) : null}

      {approval?.type === "plan" && run?.status === "awaiting_approval" ? (
        <section className="mb-6 rounded-lg border border-ai/40 bg-ai-subtle/40 p-4">
          <header className="mb-3 flex items-center justify-between">
            <div>
              <h2 className="text-sm font-semibold">Plan approval</h2>
              <p className="text-xs text-text-secondary">
                The planning agent is waiting for a human decision.
              </p>
            </div>
            {approval ? (
              <div className="flex gap-2">
                <Button loading={deciding} onClick={() => decide("approved")}>
                  Approve plan
                </Button>
                <Button variant="secondary" disabled={deciding} onClick={() => setShowReject((v) => !v)}>
                  Reject
                </Button>
              </div>
            ) : (
              <span className="text-xs text-text-muted">loading approval…</span>
            )}
          </header>

          {run.plan ? (
            <div className="grid gap-4 md:grid-cols-2">
              <div>
                <h3 className="mb-2 text-xs font-semibold uppercase text-text-muted">Steps</h3>
                <ol className="list-decimal space-y-1 pl-5 text-sm">
                  {(run.plan.steps ?? []).map((step) => (
                    <li key={step}>{step}</li>
                  ))}
                </ol>
              </div>
              <div className="space-y-4">
                <div>
                  <h3 className="mb-2 text-xs font-semibold uppercase text-text-muted">Files</h3>
                  <ul className="space-y-1 font-mono text-xs text-text-secondary">
                    {(run.plan.files ?? []).map((file) => (
                      <li key={file}>{file}</li>
                    ))}
                  </ul>
                </div>
                {(run.plan.risks ?? []).length > 0 ? (
                  <div>
                    <h3 className="mb-2 text-xs font-semibold uppercase text-text-muted">Risks</h3>
                    <ul className="list-disc space-y-1 pl-5 text-sm text-status-warning">
                      {(run.plan.risks ?? []).map((risk) => (
                        <li key={risk}>{risk}</li>
                      ))}
                    </ul>
                  </div>
                ) : null}
              </div>
            </div>
          ) : (
            <p className="text-sm text-text-secondary">The plan payload is not available.</p>
          )}

          {showReject ? (
            <div className="mt-4">
              <label htmlFor="reject-reason" className="mb-1 block text-xs font-medium text-text-secondary">
                Reason for rejection
              </label>
              <textarea
                id="reject-reason"
                value={rejectReason}
                onChange={(e) => setRejectReason(e.target.value)}
                rows={2}
                className="w-full rounded-md border border-border bg-bg-surface px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand"
              />
              <div className="mt-2 flex gap-2">
                <Button variant="danger" loading={deciding} onClick={() => decide("rejected")}>
                  Confirm rejection
                </Button>
                <Button variant="ghost" onClick={() => setShowReject(false)}>
                  Cancel
                </Button>
              </div>
            </div>
          ) : null}
        </section>
      ) : null}

      {findingRecords.length > 0 ? (
        <section className="mb-6 rounded-lg border border-border bg-bg-surface">
          <header className="flex items-center justify-between border-b border-border px-4 py-3">
            <h2 className="text-sm font-semibold">Review findings</h2>
            <div className="flex items-center gap-3">
              {openBlockers > 0 ? (
                <span className="rounded px-1.5 py-0.5 text-[11px] font-medium text-status-danger">
                  {openBlockers} open blocker{openBlockers === 1 ? "" : "s"}
                </span>
              ) : null}
              <span className="text-xs text-text-muted">{findingRecords.length}</span>
              {claims && openFindings.length > 0 && !taskClosed ? (
                <Button variant="secondary" loading={fixing} onClick={fixAllOpen}>
                  Fix all open ({openFindings.length})
                </Button>
              ) : null}
            </div>
          </header>
          <ul className="divide-y divide-border">
            {[...openFindings]
              .sort((a, b) => (b.severity === "blocker" ? 1 : 0) - (a.severity === "blocker" ? 1 : 0))
              .map((finding) => (
                <li key={finding.id} className="flex gap-3 px-4 py-2.5 text-sm">
                  <span
                    className={`h-fit rounded px-1.5 py-0.5 text-[11px] font-medium uppercase ${
                      finding.severity === "blocker"
                        ? "bg-status-danger/10 text-status-danger"
                        : "bg-bg-hover text-text-muted"
                    }`}
                  >
                    {finding.severity}
                  </span>
                  <span className="min-w-0 flex-1 text-text-secondary">
                    {finding.file ? (
                      <span className="mr-2 font-mono text-xs text-text-muted">
                        {finding.file}
                        {finding.line > 0 ? `:${finding.line}` : ""}
                      </span>
                    ) : null}
                    {finding.message}
                    <span className="mt-1 block font-mono text-[10px] text-text-muted">
                      from run {finding.run_id.slice(0, 8)}
                      {finding.created_at ? ` · ${new Date(finding.created_at).toLocaleString()}` : ""}
                    </span>
                    {finding.status !== "open" ? (
                      <span className="ml-2 text-[11px] uppercase tracking-wide text-text-muted">
                        {finding.status}
                        {finding.reason ? ` — ${finding.reason}` : ""}
                      </span>
                    ) : null}
                  </span>
                  {claims ? (
                    finding.status === "open" ? (
                      <span className="flex shrink-0 gap-1">
                        {taskClosed ? (
                          <Button
                            variant="secondary"
                            className="h-7 px-2 text-xs"
                            onClick={() => void createFollowUp(finding)}
                          >
                            Follow-up task
                          </Button>
                        ) : (
                        <Button
                          variant="secondary"
                          className="h-7 px-2 text-xs"
                          onClick={() => void fixFinding(finding)}
                        >
                          Fix
                        </Button>
                        )}
                        {taskClosed ? null : (
                        <Button
                          variant="ghost"
                          className="h-7 px-2 text-xs"
                          onClick={() => setDismissing(finding)}
                        >
                          Dismiss
                        </Button>
                        )}
                      </span>
                    ) : (
                      <Button
                        variant="ghost"
                        className="h-7 shrink-0 px-2 text-xs"
                        onClick={() => void resolveFinding(finding, "reopen")}
                      >
                        Reopen
                      </Button>
                    )
                  ) : null}
                </li>
              ))}
            {openFindings.length === 0 ? (
              <li className="px-4 py-2.5 text-sm text-text-secondary">
                Nothing open — every finding is resolved.
              </li>
            ) : null}
          </ul>
          {resolvedFindings.length > 0 ? (
            <details className="border-t border-border px-4 py-2">
              <summary className="cursor-pointer text-xs text-text-muted">
                Resolved ({resolvedFindings.length})
              </summary>
              <ul className="mt-2 divide-y divide-border">
                {resolvedFindings.map((finding) => (
                  <li key={finding.id} className="flex flex-wrap items-center gap-2 py-1.5 text-xs text-text-muted">
                    <span className="uppercase">{finding.status}</span>
                    <span className="min-w-0 flex-1 truncate">{finding.message}</span>
                    {claims && finding.status === "dismissed" ? (
                      <Button
                        variant="ghost"
                        className="h-6 px-2 text-[11px]"
                        onClick={() => void resolveFinding(finding, "reopen")}
                      >
                        Reopen
                      </Button>
                    ) : null}
                  </li>
                ))}
              </ul>
            </details>
          ) : null}
        </section>
      ) : run?.review?.findings?.length ? (
        <section className="mb-6 rounded-lg border border-border bg-bg-surface">
          <header className="flex items-center justify-between border-b border-border px-4 py-3">
            <h2 className="text-sm font-semibold">Review findings</h2>
            <span className="text-xs text-text-muted">{run.review.findings.length}</span>
          </header>
          <ul className="divide-y divide-border">
            {[...run.review.findings]
              .sort((a, b) => (b.severity === "blocker" ? 1 : 0) - (a.severity === "blocker" ? 1 : 0))
              .map((finding, index) => (
                <li key={`${finding.file ?? "general"}-${index}`} className="flex gap-3 px-4 py-2.5 text-sm">
                  <span
                    className={`h-fit rounded px-1.5 py-0.5 text-[11px] font-medium uppercase ${
                      finding.severity === "blocker"
                        ? "bg-status-danger/10 text-status-danger"
                        : "bg-bg-hover text-text-muted"
                    }`}
                  >
                    {finding.severity}
                  </span>
                  <span className="text-text-secondary">
                    {finding.file ? <span className="mr-2 font-mono text-xs text-text-muted">{finding.file}</span> : null}
                    {finding.body}
                  </span>
                </li>
              ))}
          </ul>
        </section>
      ) : null}

      <PromptDialog
        open={dismissing !== null}
        title="Dismiss the finding"
        message="A dismissal needs a reason; it stays in the record."
        label="Reason"
        placeholder="e.g. not applicable: the file is generated"
        required
        confirmLabel="Dismiss"
        onConfirm={(reason) => {
          if (dismissing) void resolveFinding(dismissing, "dismiss", reason);
          setDismissing(null);
        }}
        onCancel={() => setDismissing(null)}
      />
      {retries.length > 0 ? (
        <section className="mb-6 rounded-lg border border-border bg-bg-surface p-4">
          <h2 className="text-sm font-semibold">Retries</h2>
          <p className="mt-1 text-xs text-text-secondary">Structured retries emitted by the runner.</p>
          <ul className="mt-2 grid gap-1 text-xs">
            {retries.map((event) => {
              const payload = event.data.payload ?? {};
              return (
                <li key={event.id} className="flex flex-wrap items-center gap-2 rounded border border-border px-2 py-1">
                  <span className="rounded bg-bg-hover px-1.5 py-0.5 text-[11px] uppercase text-text-muted">{event.type}</span>
                  {typeof payload.reason === "string" ? <span className="font-mono text-[11px]">{payload.reason}</span> : null}
                  {typeof payload.attempt === "number" ? <span className="text-text-muted">attempt {payload.attempt}</span> : null}
                </li>
              );
            })}
          </ul>
        </section>
      ) : null}

      {run?.diff ? (
        <div className="mb-6">
          <DiffViewer diff={run.diff} />
        </div>
      ) : null}

      {(artifacts.some((artifact) => artifact.kind.startsWith("visual/"))) && run ? (
        <section className="mb-6 rounded-lg border border-border bg-bg-surface p-4">
          <h2 className="text-sm font-semibold">Visual comparison</h2>
          <p className="mt-1 text-xs text-text-secondary">Baseline vs the screenshot captured by this run.</p>
          <div className="mt-2 flex flex-wrap items-center gap-2">
            <select aria-label="Visual name" value={visualName} onChange={(e) => setVisualName(e.target.value)} className="h-9 rounded-md border border-border bg-bg-surface px-2 text-xs">
              <option value="">Select screenshot</option>
              {artifacts.filter((artifact) => artifact.kind.startsWith("visual/")).map((artifact) => {
                const name = artifact.kind.slice("visual/".length);
                return <option key={artifact.kind} value={name}>{name}</option>;
              })}
            </select>
            <Button
              variant="secondary"
              loading={visualLoading}
              disabled={!visualName}
              onClick={async () => {
                setVisualError(null);
                setVisualLoading(true);
                try {
                  const [baseline, current] = await Promise.all([
                    getVisualBaseline(claims!.org, run.project_id, visualName),
                    getRunVisual(claims!.org, run.id, visualName),
                  ]);
                  if (baselineURL) URL.revokeObjectURL(baselineURL);
                  if (currentURL) URL.revokeObjectURL(currentURL);
                  setBaselineURL(URL.createObjectURL(baseline));
                  setCurrentURL(URL.createObjectURL(current));
                } catch (err) {
                  setVisualError(err instanceof ApiError ? err.message : "Could not load the visual comparison");
                } finally {
                  setVisualLoading(false);
                }
              }}
            >
              Compare
            </Button>
            {visualError ? <span className="text-xs text-status-danger">{visualError}</span> : null}
          </div>
          {baselineURL && currentURL ? (
            <div className="mt-3 grid gap-3 sm:grid-cols-2">
              <figure>
                <figcaption className="mb-1 text-xs text-text-secondary">Baseline</figcaption>
                <img src={baselineURL} alt="baseline screenshot" className="w-full rounded border border-border" />
              </figure>
              <figure>
                <figcaption className="mb-1 text-xs text-text-secondary">This run</figcaption>
                <img src={currentURL} alt="current screenshot" className="w-full rounded border border-border" />
              </figure>
            </div>
          ) : null}
        </section>
      ) : null}

      <section className="mb-6 rounded-lg border border-border bg-bg-surface">
        <header className="border-b border-border p-4">
          <h2 className="text-sm font-semibold">Artifacts</h2>
          <p className="text-xs text-text-secondary">
            Stored in the object bucket (plan, diff, review, test report).
          </p>
        </header>
        {artifacts.length === 0 ? (
          <p className="p-4 text-xs text-text-secondary">No artifacts stored yet.</p>
        ) : (
          <ul className="divide-y divide-border">
            {artifacts.map((artifact) => (
              <li key={artifact.kind} className="flex flex-wrap items-center gap-3 px-4 py-2 text-xs">
                <span className="rounded bg-bg-hover px-1.5 py-0.5 text-[11px] uppercase text-text-muted">
                  {artifact.kind}
                </span>
                <span className="font-mono text-[11px] text-text-secondary">{artifact.key}</span>
                <span className="ml-auto text-text-muted">{Math.round(artifact.bytes / 1024)} KiB</span>
                <span className="font-mono text-[10px] text-text-muted" title={artifact.sha256}>
                  {artifact.sha256.slice(0, 12)}…
                </span>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="mb-6 rounded-lg border border-border bg-bg-surface">
        <header className="border-b border-border p-4">
          <h2 className="text-sm font-semibold">Usage</h2>
          <p className="text-xs text-text-secondary">Tokens and cost recorded by the run.</p>
        </header>
        <dl className="grid grid-cols-2 gap-3 p-4 text-sm md:grid-cols-4">
          <div>
            <dt className="text-xs text-text-muted">Input tokens</dt>
            <dd className="font-mono">{usage.data?.totals.input_tokens ?? 0}</dd>
          </div>
          <div>
            <dt className="text-xs text-text-muted">Output tokens</dt>
            <dd className="font-mono">{usage.data?.totals.output_tokens ?? 0}</dd>
          </div>
          <div>
            <dt className="text-xs text-text-muted">Cost</dt>
            <dd className="font-mono">${(usage.data?.totals.cost_usd ?? 0).toFixed(4)}</dd>
          </div>
          <div>
            <dt className="text-xs text-text-muted">Budget</dt>
            <dd className="font-mono text-xs">
              {run?.budget && (run.budget.tokens_in || run.budget.tokens_out || run.budget.cost_micros)
                ? `${run.budget.tokens_in || "∞"} in / ${run.budget.tokens_out || "∞"} out / $${((run.budget.cost_micros || 0) / 1e6).toFixed(2)}`
                : "no limit"}
            </dd>
          </div>
        </dl>
      </section>

      <section className="mb-6 rounded-lg border border-border bg-bg-surface">
        <header className="border-b border-border p-4">
          <h2 className="text-sm font-semibold">Context</h2>
          <p className="text-xs text-text-secondary">
            Versioned inputs and rule versions captured when this run started.
          </p>
        </header>
        {manifestSources.length > 0 ? (
          <ul className="divide-y divide-border border-b border-border">
            {manifestSources.map((source) => (
              <li key={`${source.kind}-${source.id}`} className="flex flex-wrap items-center gap-3 px-4 py-2 text-xs">
                <span className="rounded bg-bg-hover px-1.5 py-0.5 text-[11px] uppercase text-text-muted">{source.kind}</span>
                <span className="flex-1 truncate font-mono text-text-secondary">{source.id}</span>
                <span className="text-text-muted">{source.version}</span>
                {source.sha256 ? <span className="font-mono text-[10px] text-text-muted" title={source.sha256}>{source.sha256.slice(0, 10)}…</span> : null}
              </li>
            ))}
          </ul>
        ) : null}
        {manifestPrecedence.length > 0 ? (
          <p className="border-b border-border px-4 py-2 text-[11px] text-text-muted">
            Precedence: {manifestPrecedence.join(" → ")}
          </p>
        ) : null}
        {snapshotRules.length === 0 && manifestSources.length === 0 ? (
          <p className="p-4 text-xs text-text-secondary">No context snapshot recorded.</p>
        ) : (
          <ul className="divide-y divide-border">
            {snapshotRules.map((rule) => (
              <li key={`${rule.rule_id}-${rule.version}`} className="flex flex-wrap items-center gap-3 px-4 py-2 text-xs">
                <span className="rounded bg-bg-hover px-1.5 py-0.5 text-[11px] uppercase text-text-muted">
                  {rule.level ?? "org"}
                </span>
                <span className="flex-1">{rule.name}</span>
                <span className="font-mono text-text-muted">v{rule.version}</span>
              </li>
            ))}
          </ul>
        )}
      </section>

      <section className="rounded-lg border border-border bg-bg-surface">
        <header className="flex items-center justify-between border-b border-border px-4 py-3">
          <h2 className="text-sm font-semibold">Live log</h2>
          <span className="text-xs text-text-muted">{events.length} events</span>
        </header>
        <div
          ref={logRef}
          className="max-h-[420px] overflow-y-auto bg-bg-base p-4 font-mono text-xs leading-relaxed"
        >
          {events.length === 0 ? (
            <p className="text-text-muted">Waiting for the agent…</p>
          ) : (
            events.map((event) => (
              <div key={event.id} className="relative flex gap-3 border-l border-border pb-2 pl-4">
                <span aria-hidden className="absolute -left-[3px] top-1.5 h-1.5 w-1.5 rounded-full bg-brand" />
                <span className="w-10 shrink-0 text-text-muted">{event.id}</span>
                <span className="w-28 shrink-0 text-brand sm:w-36">{event.type}</span>
                <span className="min-w-0 text-text-primary">
                  {String(event.data.payload?.text ?? event.data.payload?.status ?? "")}
                </span>
              </div>
            ))
          )}
        </div>
      </section>
    </div>
  );
}
