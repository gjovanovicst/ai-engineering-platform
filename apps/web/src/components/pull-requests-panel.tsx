"use client";

import Link from "next/link";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import {
  ApiError,
  decideApproval,
  listApprovals,
  listMergeQueue,
  listProjectPullRequests,
  listPullRequestChecks,
  type PullRequest,
} from "@/lib/api";

const stateStyles: Record<string, string> = {
  open: "bg-brand-subtle text-brand",
  merged: "bg-status-success/10 text-status-success",
  closed: "bg-bg-hover text-text-muted",
};

export function PullRequestsPanel({ orgID, projectID }: { orgID: string; projectID: string }) {
  const queryClient = useQueryClient();
  const [error, setError] = useState<string | null>(null);
  const query = useQuery({
    queryKey: ["pull-requests", orgID, projectID],
    queryFn: () => listProjectPullRequests(orgID, projectID),
  });
  const approvals = useQuery({ queryKey: ["approvals", orgID], queryFn: () => listApprovals(orgID) });
  const queue = useQuery({ queryKey: ["merge-queue", orgID, projectID], queryFn: () => listMergeQueue(orgID, projectID) });

  const decide = useMutation({
    mutationFn: (input: { id: string; decision: "approved" | "rejected" }) =>
      decideApproval(orgID, input.id, input.decision, "pull request decision"),
    onSuccess: () => {
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["pull-requests", orgID, projectID] });
      queryClient.invalidateQueries({ queryKey: ["approvals", orgID] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not decide the pull request"),
  });

  const approvalFor = (pr: PullRequest) =>
    approvals.data?.data.find((a) => a.type === "pr" && a.number === pr.number && a.status === "pending");

  return (
    <section className="mt-8">
      <header className="mb-3 flex items-center justify-between">
        <div>
          <h2 className="text-sm font-semibold">Pull requests</h2>
          <p className="text-xs text-text-secondary">Opened by the agent for this project.</p>
        </div>
        <span className="text-xs text-text-muted">{query.data?.data.length ?? 0}</span>
      </header>

      {error ? (
        <p role="alert" className="mb-3 rounded-lg border border-status-danger/30 bg-status-danger/10 p-3 text-sm text-status-danger">
          {error}
        </p>
      ) : null}

      {query.isPending ? (
        <div className="space-y-2" aria-label="Loading pull requests">
          {[0, 1].map((i) => (
            <div key={i} className="h-10 animate-pulse rounded-lg bg-bg-hover" />
          ))}
        </div>
      ) : null}

      {query.isError ? (
        <p role="alert" className="rounded-lg border border-status-danger/30 bg-status-danger/10 p-3 text-sm text-status-danger">
          Could not load pull requests.
        </p>
      ) : null}

      {query.isSuccess && query.data.data.length === 0 ? (
        <p className="rounded-lg border border-dashed border-border bg-bg-surface p-4 text-sm text-text-secondary">
          No pull requests yet.
        </p>
      ) : null}

      {query.isSuccess && query.data.data.length > 0 ? (
        <ul className="divide-y divide-border overflow-hidden rounded-lg border border-border bg-bg-surface">
          {query.data.data.map((pr: PullRequest) => (
            <li key={pr.id} className="flex items-center gap-3 px-4 py-2.5 text-sm">
              <span className="rounded bg-bg-hover px-1.5 py-0.5 text-[11px] uppercase text-text-muted">
                {pr.provider}
              </span>
              <Link
                href={`/projects/${projectID}/pulls/${pr.id}`}
                className="flex-1 font-mono text-xs text-brand hover:underline"
              >
                #{pr.number}
              </Link>
              <a href={pr.url} target="_blank" rel="noreferrer" className="text-xs text-text-muted hover:underline">
                host ↗
              </a>
              <span className={`rounded px-1.5 py-0.5 text-[11px] ${stateStyles[pr.state] ?? ""}`}>
                {pr.state}
              </span>
              <ChecksForPR orgID={orgID} prID={pr.id} />
              {(() => {
                const approval = approvalFor(pr);
                if (!approval) return null;
                return (
                  <>
                    <Button
                      variant="secondary"
                      className="h-7 px-2 text-xs"
                      loading={decide.isPending && decide.variables?.id === approval.id}
                      onClick={() => decide.mutate({ id: approval.id, decision: "approved" })}
                    >
                      Approve
                    </Button>
                    <Button
                      variant="danger"
                      className="h-7 px-2 text-xs"
                      onClick={() => decide.mutate({ id: approval.id, decision: "rejected" })}
                    >
                      Reject
                    </Button>
                  </>
                );
              })()}
            </li>
          ))}
        </ul>
      ) : null}
    
      <div className="mt-8">
        <h3 className="text-sm font-semibold">Merge queue</h3>
        {queue.isPending ? <p className="mt-2 text-xs text-text-muted">Loading…</p> : null}
        {queue.isSuccess && (queue.data.items ?? []).length === 0 ? (
          <p className="mt-2 text-xs text-text-secondary">Queue is empty.</p>
        ) : null}
        {queue.isSuccess && (queue.data.items ?? []).length > 0 ? (
          <ul className="mt-2 divide-y divide-border rounded-lg border border-border bg-bg-surface">
            {(queue.data.items ?? []).map((item) => (
              <li key={item.id} className="flex flex-wrap items-center gap-2 px-3 py-2 text-xs">
                <span className="font-medium">#{item.number}</span>
                <span className="rounded bg-bg-hover px-1.5 py-0.5">{item.status}</span>
                {item.error ? <span className="text-status-danger">{item.error}</span> : null}
              </li>
            ))}
          </ul>
        ) : null}
      </div>
</section>
  );
}

const checkStyles: Record<string, string> = {
  passed: "bg-status-success/10 text-status-success",
  failed: "bg-status-danger/10 text-status-danger",
  running: "bg-brand-subtle text-brand",
  pending: "bg-bg-hover text-text-muted",
};

function ChecksForPR({ orgID, prID }: { orgID: string; prID: string }) {
  const checks = useQuery({
    queryKey: ["pr-checks", orgID, prID],
    queryFn: () => listPullRequestChecks(orgID, prID),
  });
  const items = checks.data?.data ?? [];
  if (items.length === 0) return null;
  return (
    <span className="flex flex-wrap gap-1">
      {items.map((check) => {
        const badge = (
          <span
            title={`${check.name}: ${check.status} (${check.duration_ms} ms)`}
            className={`rounded px-1.5 py-0.5 text-[11px] ${checkStyles[check.status] ?? ""}`}
          >
            {check.status === "passed" ? "✓" : check.status === "failed" ? "✕" : "…"} {check.name}
          </span>
        );
        return check.url ? (
          <a key={check.name} href={check.url} target="_blank" rel="noreferrer" title="Open the CI job">
            {badge}
          </a>
        ) : (
          <span key={check.name}>{badge}</span>
        );
      })}
    </span>
  );
}
