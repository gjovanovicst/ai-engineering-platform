"use client";

import Link from "next/link";
import { useQuery } from "@tanstack/react-query";

import { ensureSession, getPullRequest, listPullRequestChecks, type CheckResult } from "@/lib/api";

const stateStyles: Record<string, string> = {
  open: "bg-status-warning/10 text-status-warning",
  merged: "bg-status-success/10 text-status-success",
  closed: "bg-bg-hover text-text-muted",
};

export function PullRequestDetailView({ projectId, prId }: { projectId: string; prId: string }) {
  const query = useQuery({
    queryKey: ["pull-request", prId],
    queryFn: async () => {
      const session = await ensureSession();
      if (!session) throw new Error("not signed in");
      const pr = await getPullRequest(session.org, prId);
      const checks = await listPullRequestChecks(session.org, prId);
      return { pr, checks: checks.data };
    },
  });

  if (query.isPending) return <p className="text-sm text-text-muted">Loading pull request…</p>;
  if (query.isError) return <p role="alert" className="text-sm text-status-danger">Could not load the pull request.</p>;

  const { pr, checks } = query.data;
  return (
    <div className="max-w-3xl">
      <nav className="mb-4 text-xs text-text-muted">
        <Link href={`/projects/${projectId}`} className="hover:underline">Project</Link>
        <span> / </span>
        <span>Pull request #{pr.number}</span>
      </nav>

      <header className="mb-6 flex flex-wrap items-center gap-3">
        <h1 className="text-xl font-semibold">#{pr.number}</h1>
        <span className={`rounded px-2 py-0.5 text-xs ${stateStyles[pr.state] ?? ""}`}>{pr.state}</span>
        <span className="rounded bg-bg-hover px-2 py-0.5 text-xs uppercase text-text-muted">{pr.provider}</span>
        <span className="ml-auto text-xs text-text-secondary">{pr.repo_name}</span>
        <a href={pr.url} target="_blank" rel="noreferrer" className="text-xs text-brand hover:underline">
          Open on host ↗
        </a>
      </header>

      {pr.linked?.task_ref ? (
        <section className="mb-4 rounded-lg border border-border bg-bg-surface p-4">
          <h2 className="text-xs font-semibold uppercase text-text-muted">Task</h2>
          <p className="mt-1 text-sm">
            <span className="font-mono text-text-secondary">{pr.linked.task_ref}</span>{" "}
            {pr.linked.task_title}
          </p>
        </section>
      ) : null}

      <section className="rounded-lg border border-border bg-bg-surface">
        <header className="border-b border-border p-4">
          <h2 className="text-sm font-semibold">Checks</h2>
          <p className="text-xs text-text-secondary">CI results reported by the git host.</p>
        </header>
        {checks.length === 0 ? (
          <p className="p-4 text-xs text-text-secondary">No checks reported yet.</p>
        ) : (
          <ul className="divide-y divide-border">
            {checks.map((check: CheckResult) => (
              <li key={check.name} className="flex items-center gap-3 px-4 py-2.5 text-sm">
                <span className="font-mono text-xs">{check.name}</span>
                <span className={`ml-auto rounded px-2 py-0.5 text-xs ${
                  check.status === "passed"
                    ? "bg-status-success/10 text-status-success"
                    : check.status === "failed"
                      ? "bg-status-danger/10 text-status-danger"
                      : "bg-bg-hover text-text-muted"
                }`}>
                  {check.status}
                </span>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  );
}
