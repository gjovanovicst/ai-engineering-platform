"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { ensureSession, getMetrics, listBlueprints, listProjects, listProviders } from "@/lib/api";
import type { Claims } from "@/lib/auth";

function MetricCard({ label, value, detail, tone = "blue", icon }: { label: string; value: string | number; detail: string; tone?: string; icon: string }) {
  return <article className={`overview-metric metric-${tone}`}><div className="metric-top"><span>{label}</span><span className="metric-icon" aria-hidden>{icon}</span></div><strong>{value}</strong><p>{detail}</p></article>;
}

export default function OverviewPage() {
  const router = useRouter();
  const [claims, setClaims] = useState<Claims | null>(null);
  useEffect(() => { ensureSession().then((result) => { if (!result) router.replace("/login"); else setClaims(result); }); }, [router]);
  const orgID = claims?.org ?? "";
  const projects = useQuery({ queryKey: ["overview-projects", orgID], enabled: Boolean(orgID), queryFn: () => listProjects(orgID) });
  const blueprints = useQuery({ queryKey: ["overview-blueprints", orgID], enabled: Boolean(orgID), queryFn: () => listBlueprints(orgID) });
  const providers = useQuery({ queryKey: ["overview-providers", orgID], enabled: Boolean(orgID), queryFn: () => listProviders(orgID) });
  const metrics = useQuery({ queryKey: ["overview-metrics", orgID], enabled: Boolean(orgID), queryFn: () => getMetrics(orgID) });
  const projectList = projects.data?.data ?? [];
  const blueprintCount = blueprints.data?.data.length ?? 0;
  const providerCount = providers.data?.data.length ?? 0;
  const successful = metrics.data?.succeeded_runs ?? 0;
  const totalRuns = Object.values(metrics.data?.runs_by_status ?? {}).reduce((sum, n) => sum + n, 0);
  const successRate = totalRuns ? Math.round(successful / totalRuns * 100) : 0;
  const setup = useMemo(() => [
    { title: "Create your first project", detail: "Define the product and connect the work to a project.", done: projectList.length > 0, href: "/projects", tag: "Projects" },
    { title: "Connect an AI provider", detail: "Choose the model your agents are allowed to use.", done: providerCount > 0, href: "/settings", tag: "AI provider" },
    { title: "Publish a blueprint", detail: "Standardize how new applications and tasks start.", done: blueprintCount > 0, href: "/blueprints", tag: "Blueprints" },
    { title: "Complete a verified run", detail: "Run an agent, inspect the diff and review test results.", done: successful > 0, href: "/agents", tag: "AI workflow" },
  ], [projectList.length, providerCount, blueprintCount, successful]);

  return <main className="overview-page">
    <section className="overview-hero">
      <div className="hero-orb hero-orb-one"/><div className="hero-orb hero-orb-two"/>
      <div className="hero-copy"><span className="eyebrow"><span className="eyebrow-dot"/> ENGINEERING WORKSPACE</span><h1>Build better software.<br/><span>With AI in your process.</span></h1><p>Plan, build, test and review in one place — with your team’s standards and human approval built in.</p>
        <div className="hero-actions"><Link className="primary-action" href="/projects">Explore projects <span>↗</span></Link><Link className="secondary-action" href="/agents">View AI activity</Link></div>
      </div>
      <div className="hero-flow" aria-label="AI development workflow"><div className="flow-node flow-done"><span>✓</span><div><b>Plan</b><small>Requirements ready</small></div></div><div className="flow-connector"/><div className="flow-node flow-active"><span>✳</span><div><b>Build with AI</b><small>Rules-aware execution</small></div><i>LIVE</i></div><div className="flow-connector"/><div className="flow-node"><span>◇</span><div><b>Verify</b><small>Tests and human review</small></div></div></div>
    </section>

    <section className="overview-section"><div className="section-heading"><div><span className="eyebrow muted-eyebrow">AT A GLANCE</span><h2>Workspace health</h2><p>Real activity recorded by your workspace.</p></div><span className="range-pill">Last 30 days <span>⌄</span></span></div>
      <div className="overview-metrics-grid"><MetricCard label="Active projects" value={projects.isPending ? "—" : projectList.length} detail="Projects in your workspace" icon="▦"/><MetricCard label="Agent runs" value={metrics.isPending ? "—" : totalRuns} detail="Recorded executions" tone="purple" icon="✳"/><MetricCard label="Successful runs" value={metrics.isPending ? "—" : `${successRate}%`} detail={`${successful} completed successfully`} tone="green" icon="✓"/><MetricCard label="AI spend" value={metrics.data?.cost_usd?.source === "measured" ? `$${metrics.data.cost_usd.value.toFixed(2)}` : "Not available"} detail={metrics.data?.cost_usd?.source === "measured" ? "Measured provider usage" : "No measured cost data yet"} tone="amber" icon="$"/></div>
    </section>

    <div className="overview-content-grid"><section className="surface-panel projects-panel"><div className="panel-heading"><div><h2>Projects</h2><p>Your products and engineering workspaces</p></div><Link href="/projects" className="text-link">All projects <span>→</span></Link></div>
      {projects.isPending ? <div className="panel-loading">Loading projects…</div> : projectList.length ? <div className="project-list">{projectList.slice(0, 5).map((project, i) => <Link href={`/projects/${project.id}`} key={project.id} className="project-row"><span className={`project-symbol project-symbol-${i % 4}`}>{project.name.slice(0, 1).toUpperCase()}</span><span className="project-main"><b>{project.name}</b><small>{project.description || project.slug}</small></span><span className={`status-chip status-${project.status.toLowerCase().replace(/[^a-z]/g, "-")}`}>{project.status.replaceAll("_", " ")}</span><span className="row-arrow">↗</span></Link>)}</div> : <div className="empty-projects"><span>▦</span><b>No projects yet</b><p>Create a project to start organizing tasks, repositories and AI runs.</p><Link href="/projects" className="primary-action">Create a project <span>+</span></Link></div>}
      <div className="panel-footer"><span><i className="status-live-dot"/> Workspace connected</span><Link href="/projects">Manage projects →</Link></div>
    </section>

    <section className="surface-panel setup-panel"><div className="panel-heading"><div><h2>Getting started</h2><p>Set up the foundations for a reliable AI workflow.</p></div><span className="setup-count">{setup.filter(x => x.done).length}/{setup.length}</span></div><div className="setup-progress"><span style={{ width: `${setup.filter(x => x.done).length / setup.length * 100}%` }}/></div><div className="setup-list">{setup.map((item) => <Link href={item.href} className={`setup-item ${item.done ? "setup-item-done" : ""}`} key={item.title}><span className="setup-check">{item.done ? "✓" : "→"}</span><span className="setup-copy"><b>{item.title}</b><small>{item.detail}</small></span><span className="setup-tag">{item.done ? "Done" : item.tag}</span></Link>)}</div><Link href="/setup" className="setup-help">Open the setup wizard <span>↗</span></Link></section></div>

    <section className="workflow-strip"><div className="workflow-intro"><span className="eyebrow muted-eyebrow">BUILT FOR TRUST</span><h2>AI moves work forward.<br/>Your team stays in control.</h2><p>Every run is traceable, changes are reviewable, and quality gates stay visible.</p></div><div className="workflow-benefit"><span className="benefit-icon benefit-blue">⌘</span><b>Context-aware agents</b><p>Use project rules, approved blueprints and relevant task context.</p></div><div className="workflow-benefit"><span className="benefit-icon benefit-green">✓</span><b>Quality gates</b><p>Builds and tests run before changes are ready for review.</p></div><div className="workflow-benefit"><span className="benefit-icon benefit-purple">⛨</span><b>Human approval</b><p>Review plans and code changes before they move forward.</p></div></section>
  </main>;
}
