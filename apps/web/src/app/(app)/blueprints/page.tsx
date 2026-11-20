"use client";

import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { SidePanel } from "@/components/ui/side-panel";
import {
  ApiError,
  createBlueprint,
  ensureSession,
  instantiateBlueprint,
  listBlueprints,
  createBlueprintVersion,
  publishBlueprint,
} from "@/lib/api";
import type { Claims } from "@/lib/auth";

const steps = ["Basics", "Stack", "Repository", "Testing", "Agents"] as const;
const backendLanguages = ["go", "node", "python", "java", "none"];
const frontendFrameworks = ["next", "vite-react", "angular", "vue", "sveltekit", "none"];
const layouts = ["monorepo", "polyrepo"];
const commitConventions = ["conventional", "free"];
const agentRoles = ["planning", "development", "review"] as const;

type Form = {
  name: string;
  description: string;
  backend: string;
  frontend: string;
  layout: string;
  defaultBranch: string;
  commitConvention: string;
  unitTests: string;
  integrationTests: string;
  agents: Record<string, boolean>;
  starterTasks: string;
};

const templates: Array<{ id: string; name: string; description: string; patch: Partial<Form> }> = [
  {
    id: "saas",
    name: "SaaS monorepo",
    description: "Go API + Next.js in one repository, conventional commits, unit + integration tests.",
    patch: {
      backend: "go", frontend: "next", layout: "monorepo", defaultBranch: "main", commitConvention: "conventional",
      unitTests: "make test", integrationTests: "make test-integration",
      starterTasks: "Set up CI\nAdd the first end-to-end flow\nDocument local setup",
      agents: { planning: true, development: true, review: true },
    },
  },
  {
    id: "api",
    name: "Go API service",
    description: "Backend-only service with a polyrepo layout and integration tests.",
    patch: {
      backend: "go", frontend: "none", layout: "polyrepo", defaultBranch: "main", commitConvention: "conventional",
      unitTests: "go test ./...", integrationTests: "make test-integration",
      starterTasks: "Create the health endpoint\nAdd request logging",
      agents: { planning: true, development: true, review: true },
    },
  },
  {
    id: "web",
    name: "Next.js product UI",
    description: "Frontend-only product surface with a monorepo layout and visual checks ready.",
    patch: {
      backend: "none", frontend: "next", layout: "monorepo", defaultBranch: "main", commitConvention: "conventional",
      unitTests: "pnpm test", integrationTests: "pnpm test:e2e",
      starterTasks: "Build the app shell\nAdd the first screen\nAdd a visual check",
      agents: { planning: true, development: true, review: true },
    },
  },
];

const emptyForm: Form = {
  name: "",
  description: "",
  backend: "go",
  frontend: "next",
  layout: "monorepo",
  defaultBranch: "main",
  commitConvention: "conventional",
  unitTests: "make test",
  integrationTests: "make test-integration",
  agents: { planning: true, development: true, review: true },
  starterTasks: "",
};

export default function BlueprintsPage() {
  const router = useRouter();
  const queryClient = useQueryClient();
  const [claims, setClaims] = useState<Claims | null | undefined>(undefined);
  const [step, setStep] = useState(0);
  const [creating, setCreating] = useState(false);
  const [form, setForm] = useState<Form>(emptyForm);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    ensureSession().then((result) => {
      if (!result) {
        router.replace("/login");
        return;
      }
      setClaims(result);
    });
  }, [router]);

  const blueprints = useQuery({
    queryKey: ["blueprints", claims?.org],
    enabled: Boolean(claims?.org),
    queryFn: () => listBlueprints(claims!.org),
  });

  const spec = useMemo(
    () => ({
      schema_version: "2",
      name: form.name,
      description: form.description,
      stack: {
        frontend: { framework: form.frontend, language: "typescript" },
        backend: { language: form.backend },
      },
      repository: {
        layout: form.layout,
        default_branch: form.defaultBranch,
        commit_convention: form.commitConvention,
      },
      test_strategy: {
        unit: form.unitTests,
        integration: form.integrationTests,
      },
      agents: Object.fromEntries(agentRoles.filter((role) => form.agents[role]).map((role) => [role, {}])),
      starter_tasks: form.starterTasks
        .split("\n")
        .map((line) => line.trim())
        .filter(Boolean)
        .map((title) => ({ title })),
    }),
    [form],
  );

  const create = useMutation({
    mutationFn: () => createBlueprint(claims!.org, form.name, spec),
    onSuccess: () => {
      setError(null);
      setCreating(false);
      setStep(0);
      setForm(emptyForm);
      queryClient.invalidateQueries({ queryKey: ["blueprints", claims?.org] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not create the blueprint"),
  });

  const publish = useMutation({
    mutationFn: (id: string) => publishBlueprint(claims!.org, id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["blueprints", claims?.org] }),
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not publish the blueprint"),
  });

  const newVersion = useMutation({
    mutationFn: (id: string) => createBlueprintVersion(claims!.org, id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["blueprints", claims?.org] }),
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not create a version"),
  });

  const instantiate = useMutation({
    mutationFn: (id: string) => instantiateBlueprint(claims!.org, id),
    onSuccess: (project) => router.push(`/projects/${project.id}`),
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not create the project"),
  });

  const canContinue = step !== 0 || form.name.trim().length >= 3;
  const agentCount = Object.values(form.agents).filter(Boolean).length;

  function update<K extends keyof Form>(key: K, value: Form[K]) {
    setForm((current) => ({ ...current, [key]: value }));
  }

  // Closing the wizard drops the draft: the next open starts clean.
  function closeWizard() {
    setCreating(false);
    setStep(0);
    setForm(emptyForm);
    setError(null);
  }

  return (
    <div>
      <header className="mb-6 flex items-end justify-between">
        <div>
          <h1 className="text-[22px] font-semibold">Blueprints</h1>
          <p className="mt-1 text-[13px] text-text-secondary">
            A blueprint is the only entry point for creating a project.
          </p>
        </div>
        <Button onClick={() => setCreating(true)}>+ New blueprint</Button>
      </header>

      {!creating ? (
        <section className="mb-6">
          <h2 className="mb-2 text-sm font-semibold">Templates</h2>
          <div className="grid gap-3 md:grid-cols-3">
            {templates.map((template) => (
              <article key={template.id} className="rounded-lg border border-border bg-bg-surface p-3">
                <h3 className="text-sm font-medium">{template.name}</h3>
                <p className="mt-1 min-h-[48px] text-xs text-text-secondary">{template.description}</p>
                <Button
                  variant="secondary"
                  className="mt-3"
                  onClick={() => {
                    setForm((current) => ({ ...current, ...template.patch, name: current.name }));
                    setStep(0);
                    setCreating(true);
                  }}
                >
                  Use template
                </Button>
              </article>
            ))}
          </div>
        </section>
      ) : null}

      {error ? (
        <p role="alert" className="mb-4 rounded-md border border-status-danger/30 bg-status-danger/10 p-3 text-sm text-status-danger">
          {error}
        </p>
      ) : null}

      {creating ? (
        <SidePanel
          title="New blueprint"
          description="Define the stack, repository rules and starter tasks for a new project."
          dirty={JSON.stringify(form) !== JSON.stringify(emptyForm)}
          onClose={closeWizard}
          footer={
            <>
              <Button variant="secondary" onClick={closeWizard}>Cancel</Button>
              <Button variant="secondary" disabled={step === 0} onClick={() => setStep((s) => s - 1)}>Back</Button>
              {step < steps.length - 1 ? (
                <Button disabled={!canContinue} onClick={() => setStep((s) => s + 1)}>Continue</Button>
              ) : (
                <Button loading={create.isPending} onClick={() => create.mutate()}>Create blueprint</Button>
              )}
            </>
          }
        >
        <div className="blueprint-wizard">
          <header className="border-b border-border p-4">
            <ol className="flex flex-wrap gap-2 text-xs">
              {steps.map((label, index) => (
                <li
                  key={label}
                  className={`rounded-full px-2.5 py-1 ${
                    index === step
                      ? "bg-brand text-white"
                      : index < step
                        ? "bg-status-success/10 text-status-success"
                        : "bg-bg-hover text-text-muted"
                  }`}
                >
                  {index + 1}. {label}
                </li>
              ))}
            </ol>
          </header>

          <div className="p-4">
            {step === 0 ? (
              <div className="grid gap-3 md:grid-cols-2">
                <Field label="Name" htmlFor="w-name">
                  <Input
                    id="w-name"
                    required
                    minLength={3}
                    value={form.name}
                    onChange={(e) => update("name", e.target.value)}
                    placeholder="Novasoft standard"
                  />
                </Field>
                <Field label="Description" htmlFor="w-description">
                  <Input
                    id="w-description"
                    value={form.description}
                    onChange={(e) => update("description", e.target.value)}
                    placeholder="Default stack for client projects"
                  />
                </Field>
              </div>
            ) : null}

            {step === 1 ? (
              <div className="grid gap-3 md:grid-cols-3">
                <Field label="Backend" htmlFor="w-backend">
                  <Select id="w-backend" value={form.backend} onChange={(v) => update("backend", v)} options={backendLanguages} />
                </Field>
                <Field label="Frontend" htmlFor="w-frontend">
                  <Select id="w-frontend" value={form.frontend} onChange={(v) => update("frontend", v)} options={frontendFrameworks} />
                </Field>
                <Field label="Layout" htmlFor="w-layout">
                  <Select id="w-layout" value={form.layout} onChange={(v) => update("layout", v)} options={layouts} />
                </Field>
              </div>
            ) : null}

            {step === 2 ? (
              <div className="grid gap-3 md:grid-cols-2">
                <Field label="Default branch" htmlFor="w-branch">
                  <Input id="w-branch" value={form.defaultBranch} onChange={(e) => update("defaultBranch", e.target.value)} />
                </Field>
                <Field label="Commit convention" htmlFor="w-commit">
                  <Select
                    id="w-commit"
                    value={form.commitConvention}
                    onChange={(v) => update("commitConvention", v)}
                    options={commitConventions}
                  />
                </Field>
              </div>
            ) : null}

            {step === 3 ? (
              <div className="grid gap-3 md:grid-cols-2">
                <Field label="Unit test command" htmlFor="w-unit">
                  <Input id="w-unit" value={form.unitTests} onChange={(e) => update("unitTests", e.target.value)} />
                </Field>
                <Field label="Integration test command" htmlFor="w-integration">
                  <Input
                    id="w-integration"
                    value={form.integrationTests}
                    onChange={(e) => update("integrationTests", e.target.value)}
                  />
                </Field>
                <div className="md:col-span-2">
                  <Field label="Starter tasks (one title per line)" htmlFor="w-tasks">
                    <textarea
                      id="w-tasks"
                      rows={4}
                      value={form.starterTasks}
                      onChange={(e) => update("starterTasks", e.target.value)}
                      placeholder={"Set up CI\nAdd linting\nWrite the first migration"}
                      className="w-full rounded-md border border-border bg-bg-surface px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand"
                    />
                  </Field>
                </div>
              </div>
            ) : null}

            {step === 4 ? (
              <div className="space-y-4">
                <div>
                  <p className="mb-2 text-xs font-semibold uppercase text-text-muted">Agents enabled</p>
                  <div className="flex flex-wrap gap-3">
                    {agentRoles.map((role) => (
                      <label key={role} className="flex items-center gap-2 text-sm">
                        <input
                          type="checkbox"
                          checked={form.agents[role]}
                          onChange={(e) =>
                            update("agents", { ...form.agents, [role]: e.target.checked })
                          }
                        />
                        {role}
                      </label>
                    ))}
                  </div>
                </div>

                <div className="rounded-md border border-border bg-bg-base p-3 text-sm">
                  <p className="font-medium">Review</p>
                  <ul className="mt-2 space-y-1 text-text-secondary">
                    <li>
                      <span className="text-text-muted">Name:</span> {form.name || "—"}
                    </li>
                    <li>
                      <span className="text-text-muted">Stack:</span> {form.backend} backend · {form.frontend} frontend ·{" "}
                      {form.layout}
                    </li>
                    <li>
                      <span className="text-text-muted">Repository:</span> {form.defaultBranch} · {form.commitConvention}
                    </li>
                    <li>
                      <span className="text-text-muted">Tests:</span> {form.unitTests || "—"} /{" "}
                      {form.integrationTests || "—"}
                    </li>
                    <li>
                      <span className="text-text-muted">Agents:</span> {agentCount} ·{" "}
                      <span className="text-text-muted">Starter tasks:</span>{" "}
                      {spec.starter_tasks.length}
                    </li>
                  </ul>
                </div>
              </div>
            ) : null}
          </div>

        </div>
        </SidePanel>
      ) : null}

      {blueprints.isPending ? <div className="h-16 animate-pulse rounded-lg bg-bg-hover" /> : null}

      {blueprints.isSuccess && blueprints.data.data.length === 0 ? (
        <p className="rounded-lg border border-dashed border-border bg-bg-surface p-6 text-center text-sm text-text-secondary">
          No blueprints yet.
        </p>
      ) : null}

      {blueprints.isSuccess && blueprints.data.data.length > 0 ? (
        <ul className="blueprint-grid">
          {blueprints.data.data.map((blueprint) => (
            <li key={blueprint.id} className="blueprint-card">
              <div className="min-w-0 flex-1">
                <p className="text-sm font-medium">{blueprint.name}</p>
                <p className="text-xs text-text-muted">
                  v{blueprint.version} · {blueprint.spec.stack?.backend?.language ?? "?"} backend ·{" "}
                  {blueprint.spec.stack?.frontend?.framework ?? "?"} frontend ·{" "}
                  {blueprint.spec.repository?.layout ?? "?"} ·{" "}
                  {blueprint.spec.starter_tasks?.length ?? 0} starter tasks
                </p>
              </div>
              <span
                className={`rounded px-1.5 py-0.5 text-[11px] uppercase ${
                  blueprint.status === "published"
                    ? "bg-status-success/10 text-status-success"
                    : "bg-bg-hover text-text-muted"
                }`}
              >
                {blueprint.status}
              </span>
              <Button
                variant="secondary"
                loading={newVersion.isPending && newVersion.variables === blueprint.id}
                onClick={() => newVersion.mutate(blueprint.id)}
              >
                New version
              </Button>
              {blueprint.status === "draft" ? (
                <Button
                  variant="secondary"
                  loading={publish.isPending && publish.variables === blueprint.id}
                  onClick={() => publish.mutate(blueprint.id)}
                >
                  Publish
                </Button>
              ) : (
                <Button
                  variant="secondary"
                  loading={instantiate.isPending && instantiate.variables === blueprint.id}
                  onClick={() => instantiate.mutate(blueprint.id)}
                >
                  Create project
                </Button>
              )}
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}

function Select({
  id,
  value,
  onChange,
  options,
}: {
  id: string;
  value: string;
  onChange: (value: string) => void;
  options: string[];
}) {
  return (
    <select
      id={id}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className="h-9 w-full rounded-md border border-border bg-bg-surface px-3 text-sm"
    >
      {options.map((option) => (
        <option key={option} value={option}>
          {option}
        </option>
      ))}
    </select>
  );
}
