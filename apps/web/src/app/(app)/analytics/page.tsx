"use client";

import { useRouter } from "next/navigation";
import { Fragment, useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { IncidentsPanel } from "@/components/incidents-panel";
import { ApiError, ensureSession, getMetrics, type Metric } from "@/lib/api";
import type { Claims } from "@/lib/auth";

function MetricCard({ title, metric, suffix }: { title: string; metric: Metric; suffix?: string }) {
  return (
    <div className="metric-panel">
      <span>{title}</span>
      {metric.source === "na" ? (
        <strong className="!text-base !tracking-normal">Not collected yet</strong>
      ) : (
        <strong>{metric.value}<span className="ml-1 text-sm font-normal text-text-muted">{suffix ?? metric.unit}</span></strong>
      )}
      <small>Source: {metric.source}</small>
    </div>
  );
}

export default function AnalyticsPage() {
  const router = useRouter();
  const [claims, setClaims] = useState<Claims | null | undefined>(undefined);
  const [expanded, setExpanded] = useState<string | null>(null);

  useEffect(() => {
    ensureSession().then((result) => {
      if (!result) {
        router.replace("/login");
        return;
      }
      setClaims(result);
    });
  }, [router]);

  const query = useQuery({
    queryKey: ["metrics", claims?.org],
    enabled: Boolean(claims?.org),
    queryFn: () => getMetrics(claims!.org, 30),
  });

  const statuses = query.data?.runs_by_status ?? {};
  const total = Object.values(statuses).reduce((sum, value) => sum + value, 0);

  return (
    <div className="page-stack">
      <header className="page-hero mb-6">
        <h1 className="text-[22px] font-semibold">Analytics</h1>
        <p className="mt-1 text-[13px] text-text-secondary">
          Delivery metrics for the last {query.data?.days ?? 30} days.
        </p>
      </header>

      {query.isPending ? (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4" aria-label="Loading metrics">
          {[0, 1, 2, 3].map((i) => (
            <div key={i} className="h-28 animate-pulse rounded-lg bg-bg-hover" />
          ))}
        </div>
      ) : null}

      {query.isError ? (
        <div role="alert" className="rounded-lg border border-status-danger/30 bg-status-danger/10 p-4 text-sm text-status-danger">
          {query.error instanceof ApiError ? query.error.message : "Could not load metrics"}
        </div>
      ) : null}

      {query.isSuccess ? (
        <>
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
            <MetricCard title="Lead time" metric={query.data.lead_time_minutes} />
            <MetricCard title="Approval time" metric={query.data.approval_minutes} />
            <MetricCard title="Onboarding" metric={query.data.onboarding_hours} />
            <MetricCard title="Run success" metric={query.data.success_rate_percent} />
            <MetricCard title="Cost / accepted change" metric={query.data.cost_per_accepted_change_usd} />
            <MetricCard title="Deploys / day" metric={query.data.deployments_per_day} />
            <MetricCard title="Change failure rate" metric={query.data.change_failure_rate_percent} />
            <MetricCard title="MTTR" metric={query.data.mttr_minutes} />
            <MetricCard title="Runs per task" metric={query.data.runs_per_task} />
            <MetricCard title="First-pass merge" metric={query.data.first_pass_merge_percent} />
            <MetricCard title="Proof validity" metric={query.data.proof_validity_percent} />
            <MetricCard title="Dev runs / task" metric={query.data.dev_runs_per_task} />
            <MetricCard title="Escaped defects / 100 merges" metric={query.data.escaped_defects_per_100_merges} />
            <div className="rounded-lg border border-border bg-bg-surface p-4">
              <p className="text-xs uppercase text-text-muted">Merged PR/MR</p>
              <p className="mt-2 text-2xl font-semibold">{query.data.merged_pull_requests}</p>
              <p className="mt-1 text-[11px] text-text-muted">last {query.data.days} days</p>
            </div>
          </div>

          <section className="analytics-panel mt-6">
            <h2 className="mb-3 text-sm font-semibold">Runs by status</h2>
            {total === 0 ? (
              <div className="empty-state"><strong>No runs in this period</strong><p>Once agents execute tasks, their delivery outcomes will appear here.</p></div>
            ) : (
              <ul className="space-y-2">
                {Object.entries(statuses).map(([status, count]) => (
                  <li key={status} className="flex items-center gap-3 text-sm">
                    <span className="w-32 text-text-secondary">{status}</span>
                    <div className="h-2 flex-1 overflow-hidden rounded bg-bg-hover">
                      <div
                        className="analytics-bar h-full rounded bg-brand"
                        style={{ width: `${Math.round((count / total) * 100)}%` }}
                      />
                    </div>
                    <span className="w-8 text-right font-mono text-xs">{count}</span>
                  </li>
                ))}
              </ul>
            )}
          </section>

          {query.data.regressions.length > 0 ? (
            <section className="mt-6 rounded-lg border border-status-danger/40 bg-status-danger/5 p-4">
              <h2 className="text-sm font-semibold">Regressions</h2>
              <ul className="mt-2 space-y-1 text-xs">
                {query.data.regressions.map((r) => (
                  <li key={r.metric}>
                    <span className="font-mono">{r.metric}</span>: {r.current.toFixed(1)} vs baseline {r.baseline.toFixed(1)} (+{r.delta_percent.toFixed(0)}%)
                  </li>
                ))}
              </ul>
            </section>
          ) : null}

          {query.data.lead_time_trend.length > 0 ? (
            <section className="mt-6 rounded-lg border border-border bg-bg-surface p-4">
              <h2 className="text-sm font-semibold">Lead time per day</h2>
              <ul className="mt-3 space-y-1.5">
                {query.data.lead_time_trend.map((point) => (
                  <li key={point.date} className="flex items-center gap-3 text-xs">
                    <span className="w-24 font-mono text-text-secondary">{point.date}</span>
                    <div className="h-2 flex-1 overflow-hidden rounded bg-bg-hover">
                      <div className="h-full rounded bg-status-success" style={{ width: `${Math.min(100, Math.round((point.minutes / Math.max(...query.data.lead_time_trend.map((p) => p.minutes), 1)) * 100))}%` }} />
                    </div>
                    <span className="w-16 text-right font-mono">{(point.minutes / 60).toFixed(1)}h</span>
                  </li>
                ))}
              </ul>
            </section>
          ) : null}

          {query.data.merged_trend.length > 0 ? (
            <section className="mt-6 rounded-lg border border-border bg-bg-surface p-4">
              <h2 className="text-sm font-semibold">Merges per day</h2>
              <ul className="mt-3 space-y-1.5">
                {query.data.merged_trend.map((point) => {
                  const max = Math.max(...query.data.merged_trend.map((p) => p.total), 1);
                  return (
                    <li key={point.date} className="flex items-center gap-3 text-xs">
                      <span className="w-24 font-mono text-text-secondary">{point.date}</span>
                      <div className="h-2 flex-1 overflow-hidden rounded bg-bg-hover">
                        <div className="h-full rounded bg-brand" style={{ width: `${Math.round((point.total / max) * 100)}%` }} />
                      </div>
                      <span className="w-6 text-right font-mono">{point.total}</span>
                    </li>
                  );
                })}
              </ul>
            </section>
          ) : null}

          {query.data.projects.length > 0 ? (
            <section className="mt-6 overflow-hidden rounded-lg border border-border bg-bg-surface">
              <h2 className="border-b border-border p-4 text-sm font-semibold">Projects</h2>
              <table className="w-full text-left text-xs">
                <thead className="text-text-secondary">
                  <tr><th className="px-4 py-2">Project</th><th className="px-4 py-2">Merged</th><th className="px-4 py-2">Incidents</th><th className="px-4 py-2">CFR</th><th className="px-4 py-2">MTTR</th><th className="px-4 py-2">First merge</th><th className="px-4 py-2">Steps</th></tr>
                </thead>
                <tbody className="divide-y divide-border">
                  {query.data.projects.map((project) => (
                    <Fragment key={project.project_id}>
                      <tr>
                        <td className="px-4 py-2 font-medium">{project.name}</td>
                        <td className="px-4 py-2 font-mono">{project.merged}</td>
                        <td className="px-4 py-2 font-mono">{project.incidents}</td>
                        <td className="px-4 py-2 font-mono">{project.merged > 0 ? `${project.change_failure_rate_percent}%` : "na"}</td>
                        <td className="px-4 py-2 font-mono">{project.mttr_minutes ? `${project.mttr_minutes} min` : "na"}</td>
                        <td className="px-4 py-2 font-mono">{project.onboard_hours ? `${project.onboard_hours.toFixed(1)} h` : "na"}</td>
                        <td className="px-4 py-2">
                          <button type="button" aria-label={`Show onboarding steps for ${project.name}`} aria-expanded={expanded === project.project_id} onClick={() => setExpanded(expanded === project.project_id ? null : project.project_id)} className="text-xs text-brand underline">
                            {expanded === project.project_id ? "hide" : "steps"}
                          </button>
                        </td>
                      </tr>
                      {expanded === project.project_id ? (
                        <tr>
                          <td colSpan={7} className="bg-bg-hover px-4 py-2 text-xs text-text-secondary">
                            <div className="flex flex-wrap gap-4">
                              <span>first task: {project.task_hours !== undefined ? `${project.task_hours.toFixed(1)} h` : "na"}</span>
                              <span>first run: {project.run_hours !== undefined ? `${project.run_hours.toFixed(1)} h` : "na"}</span>
                              <span>first PR: {project.pr_hours !== undefined ? `${project.pr_hours.toFixed(1)} h` : "na"}</span>
                              <span>first merge: {project.onboard_hours !== undefined ? `${project.onboard_hours.toFixed(1)} h` : "na"}</span>
                            </div>
                          </td>
                        </tr>
                      ) : null}
                    </Fragment>
                  ))}
                </tbody>
              </table>
            </section>
          ) : null}

          <IncidentsPanel orgID={claims?.org ?? ""} />

          <p className="mt-4 text-[11px] text-text-muted">
            Cost is not collected yet; the metric shows <code>na</code> instead of a fake number.
          </p>
        </>
      ) : null}
    </div>
  );
}
