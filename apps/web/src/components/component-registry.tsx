"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { SidePanel } from "@/components/ui/side-panel";
import {
  ApiError,
  importComponents,
  importFigmaComponents,
  createComponentEntry,
  deleteComponentEntry,
  listComponentEntries,
  updateComponentEntry,
} from "@/lib/api";

const statuses = ["approved", "draft", "published", "deprecated", "forbidden"];
const categories = ["layout", "navigation", "data", "form", "feedback", "overlay", "typography", "other"];
const emptyForm = {
  name: "", purpose: "", rules: "", status: "approved", category: "other",
  framework: "", package: "", import_path: "", version: "", docs: "", example: "",
};

export function ComponentRegistry({ orgID }: { orgID: string }) {
  const queryClient = useQueryClient();
  const [form, setForm] = useState(emptyForm);
  const [drafts, setDrafts] = useState<Record<string, typeof emptyForm>>({});
  const [error, setError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);

  const entries = useQuery({ queryKey: ["component-entries", orgID], queryFn: () => listComponentEntries(orgID) });

  const create = useMutation({
    mutationFn: () => createComponentEntry(orgID, form),
    onSuccess: () => {
      setForm(emptyForm);
      setShowForm(false);
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["component-entries", orgID] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not create the component"),
  });

  const update = useMutation({
    mutationFn: (input: { id: string } & typeof emptyForm) =>
      updateComponentEntry(orgID, input.id, input),
    onSuccess: () => {
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["component-entries", orgID] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not save the component"),
  });

  const remove = useMutation({
    mutationFn: (id: string) => deleteComponentEntry(orgID, id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["component-entries", orgID] }),
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not delete the component"),
  });

  return (
    <section className="rounded-lg border border-border bg-bg-surface">
      <header className="flex flex-wrap items-start justify-between gap-3 border-b border-border p-4">
        <div>
          <h2 className="text-sm font-semibold">Component registry</h2>
          <p className="mt-1 text-xs text-text-secondary">
            The components your teams may use, with the contract each one keeps.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Button variant="secondary" onClick={() => importFigma(orgID, setError)}>Import Figma</Button>
          <label className="cursor-pointer rounded-md border border-border px-3 py-2 text-xs text-text-secondary hover:bg-bg-hover">
            Import Storybook JSON
            <input type="file" accept="application/json,.json" className="hidden" onChange={(e) => importFile(e.target.files?.[0] ?? null, orgID, setError)} />
          </label>
          <Button variant="secondary" onClick={() => setShowForm(true)}>+ Add component</Button>
        </div>
      </header>

      {error ? (
        <p role="alert" className="border-b border-border p-3 text-xs text-status-danger">
          {error}
        </p>
      ) : null}

      {entries.isSuccess && entries.data.data.length === 0 ? (
        <p className="p-4 text-sm text-text-secondary">
          No components yet. Load the defaults from the Design System page or add one below.
        </p>
      ) : null}

      <ul className="divide-y divide-border">
        {entries.isSuccess
          ? entries.data.data.map((entry) => {
              const draft = drafts[entry.id] ?? {
                name: entry.name, purpose: entry.purpose, rules: entry.rules, status: entry.status,
                category: entry.category, framework: entry.framework, package: entry.package,
                import_path: entry.import_path, version: entry.version, docs: entry.docs, example: entry.example,
              };
              return (
                <li key={entry.id} className="space-y-2 px-4 py-3 text-sm">
                  <div className="flex flex-wrap items-center gap-2">
                    <span className="w-36 font-medium">{entry.name}</span>
                    <span className="rounded bg-bg-hover px-1.5 py-0.5 text-[10px] uppercase text-text-muted">{entry.category}</span>
                    {entry.framework ? <span className="text-[11px] text-text-muted">{entry.framework}</span> : null}
                    {entry.version ? <span className="font-mono text-[10px] text-text-muted">v{entry.version}</span> : null}
                    <select
                      aria-label={`${entry.name} status`}
                      value={draft.status}
                      onChange={(e) => setDrafts({ ...drafts, [entry.id]: { ...draft, status: e.target.value } })}
                      className="h-8 rounded-md border border-border bg-bg-surface px-2 text-xs"
                    >
                      {statuses.map((status) => (
                        <option key={status} value={status}>
                          {status}
                        </option>
                      ))}
                    </select>
                    <Button
                      variant="secondary"
                      className="h-7 px-2 text-xs"
                      loading={update.isPending && update.variables?.id === entry.id}
                      onClick={() => update.mutate({ id: entry.id, ...draft })}
                    >
                      Save
                    </Button>
                    <Button
                      variant="danger"
                      className="h-7 px-2 text-xs"
                      loading={remove.isPending && remove.variables === entry.id}
                      onClick={() => remove.mutate(entry.id)}
                    >
                      Delete
                    </Button>
                  </div>
                  {entry.package || entry.import_path ? (
                    <p className="font-mono text-[11px] text-text-secondary">
                      {entry.package}
                      {entry.package && entry.import_path ? " · " : ""}
                      {entry.import_path}
                    </p>
                  ) : null}
                  <Input
                    aria-label={`${entry.name} purpose`}
                    value={draft.purpose}
                    placeholder="Purpose"
                    onChange={(e) => setDrafts({ ...drafts, [entry.id]: { ...draft, purpose: e.target.value } })}
                  />
                  <Input
                    aria-label={`${entry.name} rules`}
                    value={draft.rules}
                    placeholder="Rules"
                    onChange={(e) => setDrafts({ ...drafts, [entry.id]: { ...draft, rules: e.target.value } })}
                  />
                  {entry.docs || entry.example ? (
                    <details className="rounded-md border border-border bg-bg-canvas p-2 text-xs">
                      <summary className="cursor-pointer text-text-secondary">Preview & usage</summary>
                      {entry.docs ? (
                        <a href={entry.docs} target="_blank" rel="noreferrer" className="mt-2 block text-brand hover:underline">
                          {entry.docs}
                        </a>
                      ) : null}
                      {entry.example ? (
                        <pre className="mt-2 overflow-x-auto rounded bg-bg-surface p-2 font-mono text-[11px] text-text-primary">{entry.example}</pre>
                      ) : null}
                      {entry.example ? (
                        <button
                          type="button"
                          className="mt-2 text-[11px] text-brand hover:underline"
                          onClick={() => navigator.clipboard?.writeText(entry.example)}
                        >
                          Copy example
                        </button>
                      ) : null}
                    </details>
                  ) : null}
                </li>
              );
            })
          : null}
      </ul>

      {showForm ? (
      <SidePanel
        title="Add component"
        description="Register a component with its package, import path and the contract it keeps."
        onClose={() => setShowForm(false)}
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowForm(false)}>Cancel</Button>
            <Button type="submit" form="component-form" loading={create.isPending}>Add component</Button>
          </>
        }
      >
      <form
        id="component-form"
        className="grid gap-3 p-5"
        onSubmit={(event) => {
          event.preventDefault();
          create.mutate();
        }}
      >
        <Field label="Name" htmlFor="component-name">
          <Input id="component-name" required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
        </Field>
        <Field label="Status" htmlFor="component-status">
          <select
            id="component-status"
            value={form.status}
            onChange={(e) => setForm({ ...form, status: e.target.value })}
            className="h-9 w-full rounded-md border border-border bg-bg-surface px-3 text-sm"
          >
            {statuses.map((status) => (
              <option key={status} value={status}>
                {status}
              </option>
            ))}
          </select>
        </Field>
        <Field label="Purpose" htmlFor="component-purpose">
          <Input id="component-purpose" value={form.purpose} onChange={(e) => setForm({ ...form, purpose: e.target.value })} />
        </Field>
        <Field label="Rules" htmlFor="component-rules">
          <Input id="component-rules" value={form.rules} onChange={(e) => setForm({ ...form, rules: e.target.value })} />
        </Field>
        <div className="grid gap-2 md:col-span-2 md:grid-cols-3">
          <select aria-label="Category" value={form.category} onChange={(e) => setForm({ ...form, category: e.target.value })} className="h-9 rounded-md border border-border bg-bg-surface px-2 text-sm">
            {categories.map((category) => <option key={category} value={category}>{category}</option>)}
          </select>
          <Input aria-label="Framework" placeholder="framework (react, vue…)" value={form.framework} onChange={(e) => setForm({ ...form, framework: e.target.value })} />
          <Input aria-label="Version" placeholder="version" value={form.version} onChange={(e) => setForm({ ...form, version: e.target.value })} />
          <Input aria-label="Package" placeholder="package" value={form.package} onChange={(e) => setForm({ ...form, package: e.target.value })} />
          <Input aria-label="Import path" placeholder="import path" value={form.import_path} onChange={(e) => setForm({ ...form, import_path: e.target.value })} />
          <Input aria-label="Docs" placeholder="docs URL" value={form.docs} onChange={(e) => setForm({ ...form, docs: e.target.value })} />
        </div>
        <div className="md:col-span-2">
          <Input aria-label="Example" placeholder="example usage" value={form.example} onChange={(e) => setForm({ ...form, example: e.target.value })} />
        </div>
      </form>
      </SidePanel>
      ) : null}
    </section>
  );
}

async function importFile(file: File | null, orgID: string, setError: (v: string | null) => void) {
  if (!file) return;
  try {
    const parsed: unknown = JSON.parse(await file.text());
    const list = Array.isArray(parsed) ? parsed : (parsed as { components?: unknown }).components;
    if (!Array.isArray(list)) throw new Error("expected an array or {components: []}");
    const result = await importComponents(orgID, list as Parameters<typeof importComponents>[1]);
    setError(`imported ${result.created}, skipped ${result.skipped}`);
    window.location.reload();
  } catch (err) {
    setError(err instanceof Error ? err.message : "Could not import components");
  }
}

async function importFigma(orgID: string, setError: (v: string | null) => void) {
  const key = window.prompt("Figma file key");
  if (!key) return;
  try {
    const result = await importFigmaComponents(orgID, key.trim());
    setError(`imported ${result.created}, skipped ${result.skipped}`);
    window.location.reload();
  } catch (err) {
    setError(err instanceof Error ? err.message : "Could not import from Figma");
  }
}
