"use client";

import Link from "next/link";

import { Button } from "@/components/ui/button";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { ensureSession, listOrgRuns } from "@/lib/api";
import type { Claims } from "@/lib/auth";

const filters = [
  { label: "All", value: "" },
  { label: "Queued", value: "queued" },
  { label: "Running", value: "running" },
  { label: "Awaiting approval", value: "awaiting_approval" },
  { label: "Succeeded", value: "succeeded" },
  { label: "Failed", value: "failed" },
];

const runStyles: Record<string, string> = {
  queued: "text-text-muted",
  running: "text-brand",
  awaiting_approval: "text-status-warning",
  succeeded: "text-status-success",
  failed: "text-status-danger",
  canceled: "text-text-muted",
};

export default function AgentsPage() {
  const router = useRouter();
  const [claims, setClaims] = useState<Claims | null | undefined>(undefined);
  const [status, setStatus] = useState("");

  useEffect(() => {
    ensureSession().then((result) => {
      if (!result) {
        router.replace("/login");
        return;
      }
      setClaims(result);
    });
  }, [router]);

  const runs = useQuery({
    queryKey: ["org-runs", claims?.org, status],
    enabled: Boolean(claims?.org),
    queryFn: () => listOrgRuns(claims!.org, status || undefined),
  });

  const allRuns = runs.data?.data ?? [];
  const runningCount = allRuns.filter((run) => run.status === "running").length;
  const attentionCount = allRuns.filter((run) => ["failed", "awaiting_approval"].includes(run.status)).length;

  return (
    <div className="page-stack">
      <header className="page-hero mb-6">
        <h1 className="text-[22px] font-semibold">Agents</h1>
        <p className="mt-1 text-[13px] text-text-secondary">
          Every run across the workspace, newest first.
        </p>
      </header>

      <section className="mb-5 grid gap-3 sm:grid-cols-3">
        {[{label:"Runs in view",value:allRuns.length,note:"Matching current filter"},{label:"Running now",value:runningCount,note:"Active agent executions"},{label:"Needs attention",value:attentionCount,note:"Failed or waiting approval"}].map((item) => <div key={item.label} className="metric-panel"><span>{item.label}</span><strong>{item.value}</strong><small>{item.note}</small></div>)}
      </section>

      <div className="mb-4 flex flex-wrap gap-2 rounded-xl border border-border bg-bg-surface p-2">
        {filters.map((filter) => (
          <button
            key={filter.label}
            type="button"
            onClick={() => setStatus(filter.value)}
            className={`rounded-md px-2.5 py-1 text-xs font-medium ${
              status === filter.value ? "bg-brand text-white" : "bg-bg-hover text-text-secondary"
            }`}
          >
            {filter.label}
          </button>
        ))}
      </div>

      {runs.isPending ? (
        <div className="space-y-2">{[0, 1, 2, 3].map((i) => <div key={i} className="h-12 animate-pulse rounded-lg bg-bg-hover" />)}</div>
      ) : null}

      {runs.isError ? (
        <div role="alert" className="rounded-xl border border-status-danger/30 bg-status-danger/10 p-4 text-sm text-status-danger">Could not load runs. <button className="ml-2 underline" onClick={() => runs.refetch()}>Try again</button></div>
      ) : null}

      {runs.isSuccess && runs.data.data.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border bg-bg-surface p-8 text-center">
          <p className="text-sm font-medium">No runs for this filter</p>
          <p className="mt-1 text-xs text-text-secondary">
            Every run starts from a task: pick one and start planning, or clear the filter to see the whole history.
          </p>
          <div className="mt-3 flex flex-wrap items-center justify-center gap-2">
            <Link href="/projects" className="inline-flex h-8 items-center rounded-md bg-brand px-3 text-xs font-medium text-white hover:opacity-90">
              Open the projects
            </Link>
            <Button variant="secondary" className="h-8 text-xs" onClick={() => setStatus("all")}>Clear the filter</Button>
          </div>
        </div>
      ) : null}

      {runs.isSuccess && runs.data.data.length > 0 ? (
        <ul className="divide-y divide-border overflow-hidden rounded-lg border border-border bg-bg-surface">
          {runs.data.data.map((run) => (
            <li key={run.id}>
              <Link
                href={`/projects/${run.project_id}/runs/${run.id}`}
                className="run-list-row flex flex-wrap items-center gap-3 px-4 py-3 text-sm hover:bg-bg-hover"
              >
                <span className="w-20 font-mono text-xs text-text-muted">{run.agent_type}</span>
                <span className="font-mono text-xs text-text-muted">{run.task_ref || "—"}</span>
                <span className="flex-1">{run.task_title || "—"}</span>
                <span className={`text-xs ${runStyles[run.status] ?? ""}`}>{run.status}</span>
              </Link>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
