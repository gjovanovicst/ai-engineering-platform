"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import {
  ApiError,
  connectRepo,
  createProvider,
  ensureSession,
  listProjects,
  listProviders,
  listRepos,
  updateProject,
} from "@/lib/api";
import type { Claims } from "@/lib/auth";
import { steps } from "./steps";

const STORAGE_KEY = "aip.setup.step";

export default function SetupPage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [claims, setClaims] = useState<Claims | null>(null);
  const [step, setStep] = useState(0);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    ensureSession().then((result) => {
      if (!result) router.replace("/login");
      else setClaims(result);
    });
    if (typeof window !== "undefined") {
      setStep(Number(window.localStorage.getItem(STORAGE_KEY) ?? 0) || 0);
    }
  }, [router]);

  const orgID = claims?.org ?? "";
  const projects = useQuery({ queryKey: ["setup-projects", orgID], enabled: Boolean(orgID), queryFn: () => listProjects(orgID) });
  const providers = useQuery({ queryKey: ["setup-providers", orgID], enabled: Boolean(orgID), queryFn: () => listProviders(orgID) });
  const project = projects.data?.data[0];
  const repos = useQuery({
    queryKey: ["setup-repos", orgID, project?.id],
    enabled: Boolean(orgID && project),
    queryFn: () => listRepos(orgID, project!.id),
  });

  const done = useMemo(
    () => ({
      provider: (providers.data?.data.length ?? 0) > 0,
      repository: (repos.data?.data.length ?? 0) > 0,
      test: Boolean(project?.test_command),
      sandbox: true,
    }),
    [providers.data, repos.data, project?.test_command],
  );

  const current = steps[step];
  const progress = steps.filter((entry) => done[entry.key]).length;

  const [provider, setProvider] = useState({ name: "openai", base_url: "https://api.openai.com/v1", model: "gpt-4o-mini", api_key: "" });
  const [repo, setRepo] = useState({ provider: "github" as "github" | "gitlab", name: "", default_branch: "main", webhook_secret: "" });
  const [testCommand, setTestCommand] = useState("");

  function persist(next: number) {
    setStep(next);
    setError(null);
    if (typeof window !== "undefined") window.localStorage.setItem(STORAGE_KEY, String(next));
  }

  async function submitProvider() {
    setBusy(true);
    setError(null);
    try {
      await createProvider(orgID, provider);
      await queryClient.invalidateQueries();
      persist(1);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not save the provider");
    } finally {
      setBusy(false);
    }
  }

  async function submitRepository() {
    if (!project) {
      setError("Create a project first");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await connectRepo(orgID, project.id, repo);
      await queryClient.invalidateQueries();
      persist(2);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not connect the repository");
    } finally {
      setBusy(false);
    }
  }

  async function submitTestCommand() {
    if (!project) {
      setError("Create a project first");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await updateProject(orgID, project.id, { test_command: testCommand });
      await queryClient.invalidateQueries();
      persist(3);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not save the test command");
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="mx-auto max-w-2xl px-4 py-8">
      <nav aria-label="Breadcrumb" className="mb-1 text-xs text-text-muted">
        <Link href="/overview" className="hover:text-text-secondary">Overview</Link>
        <span> / </span>
        <span className="text-text-secondary">Setup</span>
      </nav>
      <h1 className="text-xl font-semibold">Set up your workspace</h1>
      <p className="mt-1 text-sm text-text-secondary">
        Four steps to a reliable AI workflow. You can leave and come back — the progress is saved.
      </p>

      <div className="mt-4 flex items-center gap-2" role="progressbar" aria-valuenow={progress} aria-valuemin={0} aria-valuemax={steps.length} aria-label="Setup progress">
        {steps.map((entry, index) => (
          <span key={entry.key} className={`h-1.5 flex-1 rounded-full ${done[entry.key] ? "bg-status-success" : index === step ? "bg-brand" : "bg-bg-hover"}`} />
        ))}
      </div>
      <p className="mt-1 text-xs text-text-muted">{progress}/{steps.length} done</p>

      <ol className="mt-6 space-y-2">
        {steps.map((entry, index) => (
          <li key={entry.key}>
            <button
              type="button"
              onClick={() => persist(index)}
              className={`flex w-full items-center gap-3 rounded-lg border px-3 py-2 text-left ${index === step ? "border-brand bg-brand-subtle/30" : "border-border bg-bg-surface hover:bg-bg-hover"}`}
            >
              <span className={`grid h-6 w-6 shrink-0 place-items-center rounded-full text-xs ${done[entry.key] ? "bg-status-success text-white" : "bg-bg-hover text-text-muted"}`}>
                {done[entry.key] ? "✓" : index + 1}
              </span>
              <span className="min-w-0">
                <span className="block text-sm font-medium">{entry.title}</span>
                <span className="block text-xs text-text-muted">{entry.detail}</span>
              </span>
            </button>
          </li>
        ))}
      </ol>

      <section className="mt-6 rounded-lg border border-border bg-bg-surface p-4">
        <h2 className="text-sm font-semibold">{current.title}</h2>
        {error ? <p role="alert" className="mt-2 rounded border border-status-danger/30 bg-status-danger/10 p-2 text-xs text-status-danger">{error}</p> : null}

        {current.key === "provider" ? (
          <div className="mt-3 grid gap-2 sm:grid-cols-2">
            <Field label="Name" value={provider.name} onChange={(value) => setProvider({ ...provider, name: value })} />
            <Field label="Base URL" value={provider.base_url} onChange={(value) => setProvider({ ...provider, base_url: value })} />
            <Field label="Model" value={provider.model} onChange={(value) => setProvider({ ...provider, model: value })} />
            <Field label="API key" value={provider.api_key} onChange={(value) => setProvider({ ...provider, api_key: value })} type="password" />
            <div className="sm:col-span-2"><Button loading={busy} onClick={() => void submitProvider()}>Save and continue</Button></div>
          </div>
        ) : null}

        {current.key === "repository" ? (
          <div className="mt-3 grid gap-2 sm:grid-cols-2">
            <label className="text-xs text-text-secondary">
              Provider
              <select aria-label="Repository provider" value={repo.provider} onChange={(event) => setRepo({ ...repo, provider: event.target.value as "github" | "gitlab" })} className="mt-1 h-9 w-full rounded-md border border-border bg-bg-surface px-2 text-sm">
                <option value="github">GitHub</option>
                <option value="gitlab">GitLab</option>
              </select>
            </label>
            <Field label="Repository" value={repo.name} onChange={(value) => setRepo({ ...repo, name: value })} />
            <Field label="Default branch" value={repo.default_branch} onChange={(value) => setRepo({ ...repo, default_branch: value })} />
            <Field label="Webhook secret" value={repo.webhook_secret} onChange={(value) => setRepo({ ...repo, webhook_secret: value })} type="password" />
            <div className="sm:col-span-2"><Button loading={busy} onClick={() => void submitRepository()}>Save and continue</Button></div>
          </div>
        ) : null}

        {current.key === "test" ? (
          <div className="mt-3 space-y-2">
            <Field label="Test command" value={testCommand} onChange={setTestCommand} />
            <Button loading={busy} onClick={() => void submitTestCommand()}>Save and continue</Button>
          </div>
        ) : null}

        {current.key === "sandbox" ? (
          <div className="mt-3 space-y-3 text-sm text-text-secondary">
            <p>
              The sandbox image is set by the operator (<code className="font-mono text-xs">AIP_SANDBOX_IMAGE</code>). The worker runs every agent in it for isolation.
            </p>
            <Button onClick={() => persist(0)}>Back to the start</Button>
          </div>
        ) : null}
      </section>

      <div className="mt-4 flex items-center justify-between text-xs">
        <Link href="/overview" className="text-text-muted hover:text-text-secondary">Skip for now</Link>
        {step > 0 ? <button type="button" onClick={() => persist(step - 1)} className="text-text-muted hover:text-text-secondary">← Back</button> : null}
      </div>
    </main>
  );
}

function Field({ label, value, onChange, type = "text" }: { label: string; value: string; onChange: (value: string) => void; type?: string }) {
  return (
    <label className="text-xs text-text-secondary">
      {label}
      <input
        aria-label={label}
        type={type}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="mt-1 h-9 w-full rounded-md border border-border bg-bg-surface px-2 text-sm"
      />
    </label>
  );
}
