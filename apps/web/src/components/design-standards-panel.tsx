"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { SidePanel } from "@/components/ui/side-panel";
import { useZodForm } from "@/lib/form";
import { designStandardSchema, type DesignStandardValues } from "@/lib/schemas";
import {
  ApiError,
  createDesignStandard,
  createDesignStandardVersion,
  listDesignStandards,
  listDesignTokens,
  publishDesignStandard,
  type DesignStandard,
} from "@/lib/api";

type TokenDiff = { name: string; from?: string; to?: string };

type BuilderState = {
  id?: string;
  base: DesignStandard["spec"] | null;
  name: string;
  accessibility: string;
  responsive: string;
  patterns: string;
  advanced: string;
  error?: string;
};

function diffTokens(a: DesignStandard, b: DesignStandard): TokenDiff[] {
  const left = new Map(a.spec.tokens.map((token) => [token.name, token.value]));
  const right = new Map(b.spec.tokens.map((token) => [token.name, token.value]));
  const names = new Set([...left.keys(), ...right.keys()]);
  return [...names]
    .filter((name) => left.get(name) !== right.get(name))
    .sort()
    .map((name) => ({ name, from: left.get(name), to: right.get(name) }));
}

function lines(value: string): string[] {
  return value.split("\n").map((line) => line.trim()).filter(Boolean);
}

function builderFrom(item?: DesignStandard): BuilderState {
  const spec = item?.spec as Record<string, unknown> | undefined;
  const list = (value: unknown): string[] =>
    Array.isArray(value) ? value.filter((entry): entry is string => typeof entry === "string") : [];
  const advanced: Record<string, unknown> = {};
  if (spec) {
    for (const [key, value] of Object.entries(spec)) {
      if (!["schema_version", "tokens", "accessibility", "responsive", "patterns"].includes(key)) {
        advanced[key] = value;
      }
    }
  }
  return {
    id: item?.id,
    base: item?.spec ?? null,
    name: item?.name ?? "",
    accessibility: list(spec?.accessibility).join("\n"),
    responsive: list(spec?.responsive).join("\n"),
    patterns: list(spec?.patterns).join("\n"),
    advanced: Object.keys(advanced).length > 0 ? JSON.stringify(advanced, null, 2) : "",
  };
}

export function DesignStandardsPanel({ orgID }: { orgID: string }) {
  const queryClient = useQueryClient();
  const standards = useQuery({ queryKey: ["design-standards", orgID], queryFn: () => listDesignStandards(orgID) });
  const tokens = useQuery({ queryKey: ["design-tokens", orgID], queryFn: () => listDesignTokens(orgID) });
  const [builder, setBuilder] = useState<BuilderState | null>(null);
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useZodForm<DesignStandardValues>(designStandardSchema, {
    defaultValues: { name: "", accessibility: "", responsive: "", patterns: "", advanced: "" },
  });
  const [compare, setCompare] = useState<{ base: DesignStandard; next: DesignStandard } | null>(null);
  const [error, setError] = useState<string | null>(null);

  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["design-standards", orgID] });
  const onError = (err: unknown) =>
    setError(err instanceof ApiError ? err.message : "The design standard operation failed");

  const create = useMutation({
    mutationFn: (state: BuilderState) => {
      const spec = buildSpec(state, tokens.data?.data ?? [], null);
      return state.id
        ? createDesignStandardVersion(orgID, state.id, spec)
        : createDesignStandard(orgID, state.name.trim(), spec);
    },
    onSuccess: () => {
      setBuilder(null);
      setError(null);
      invalidate();
    },
    onError,
  });

  const publish = useMutation({
    mutationFn: (id: string) => publishDesignStandard(orgID, id),
    onSuccess: invalidate,
    onError,
  });

  const rows = standards.data?.data ?? [];
  const groups = [...new Set(rows.map((row) => row.name))];
  const tokenCount = tokens.data?.data.length ?? 0;

  const submit = handleSubmit((values) => {
    if (!builder) return;
    try {
      const spec = buildSpec({ ...builder, ...values }, tokens.data?.data ?? [], builder.base);
      if (spec.tokens.length === 0) throw new Error("Add design tokens first");
      create.mutate({ ...builder, ...values, base: spec });
    } catch (err) {
      setBuilder({ ...builder, ...values, error: err instanceof Error ? err.message : "Invalid spec" });
    }
  });

  function openBuilder(state: BuilderState | null) {
    setBuilder(state);
    if (state) {
      reset({
        name: state.name,
        accessibility: state.accessibility,
        responsive: state.responsive,
        patterns: state.patterns,
        advanced: state.advanced,
      });
    }
  }

  return (
    <section className="rounded-lg border border-border bg-bg-surface">
      <header className="flex flex-wrap items-start justify-between gap-3 border-b border-border p-4">
        <div>
          <h2 className="text-sm font-semibold">Design standards</h2>
          <p className="mt-1 text-xs text-text-secondary">
            Versioned standards projects and agents inherit. Draft edits stay mutable; published versions are frozen.
          </p>
        </div>
        <Button variant="secondary" onClick={() => openBuilder(builderFrom())}>
          New standard
        </Button>
      </header>

      {builder ? (
        <SidePanel
          title={builder.id ? "Edit design standard version" : "New design standard"}
          description="Tokens come from the registry; other sections shape what agents inherit. Publishing freezes the version."
          onClose={() => setBuilder(null)}
          footer={
            <>
              <Button variant="secondary" onClick={() => setBuilder(null)}>Cancel</Button>
              <Button loading={create.isPending} disabled={tokenCount === 0} onClick={submit}>
                {builder.id ? "Create version" : "Create standard"}
              </Button>
            </>
          }
        >
        <div className="space-y-4 p-5">
          <div className="grid gap-3">
            <Field label="Name" htmlFor="standard-name" error={errors.name?.message}>
              <Input id="standard-name" aria-invalid={Boolean(errors.name)} {...register("name")} placeholder="Acme product UI" />
            </Field>
            <Field label={`Tokens (from the registry: ${tokenCount})`} htmlFor="standard-tokens">
              <Input id="standard-tokens" disabled value={tokenCount ? `${tokenCount} tokens will be captured` : "No tokens yet"} className="bg-bg-canvas text-text-muted" />
            </Field>
          </div>
          <div className="grid gap-3">
            <Field label="Accessibility (one per line)" htmlFor="standard-accessibility" error={errors.accessibility?.message}>
              <textarea
                id="standard-accessibility"
                rows={4}
                aria-invalid={Boolean(errors.accessibility)}
                {...register("accessibility")}
                className="w-full rounded-md border border-border bg-bg-surface px-3 py-2 text-xs"
              />
            </Field>
            <Field label="Responsive rules (one per line)" htmlFor="standard-responsive" error={errors.responsive?.message}>
              <textarea
                id="standard-responsive"
                rows={4}
                aria-invalid={Boolean(errors.responsive)}
                {...register("responsive")}
                className="w-full rounded-md border border-border bg-bg-surface px-3 py-2 text-xs"
              />
            </Field>
            <Field label="Patterns (one per line)" htmlFor="standard-patterns" error={errors.patterns?.message}>
              <textarea
                id="standard-patterns"
                rows={4}
                aria-invalid={Boolean(errors.patterns)}
                {...register("patterns")}
                className="w-full rounded-md border border-border bg-bg-surface px-3 py-2 text-xs"
              />
            </Field>
          </div>
          <Field label="Typography, spacing, radius, themes (JSON, optional)" htmlFor="standard-advanced" error={errors.advanced?.message}>
            <textarea
              id="standard-advanced"
              rows={5}
              aria-invalid={Boolean(errors.advanced)}
              {...register("advanced")}
              placeholder={'{\n  "typography": {"font_family": "Inter"},\n  "spacing": {"unit": "4px"},\n  "radius": {"card": "14px"}\n}'}
              className="w-full rounded-md border border-border bg-bg-surface px-3 py-2 font-mono text-xs"
            />
          </Field>
          {builder.error ? <p role="alert" className="text-xs text-status-danger">{builder.error}</p> : null}
          <p className="text-xs text-text-muted">
            {builder.id ? "Creates the next draft version." : "Creates v1 as a draft."} Publish when ready.
          </p>
        </div>
        </SidePanel>
      ) : null}

      {error ? <p role="alert" className="border-b border-border p-3 text-xs text-status-danger">{error}</p> : null}
      {standards.isPending ? <p className="p-4 text-sm text-text-muted">Loading standards…</p> : null}
      {standards.isError ? <p role="alert" className="p-4 text-sm text-status-danger">Could not load design standards.</p> : null}
      {standards.isSuccess && rows.length === 0 ? (
        <p className="p-4 text-sm text-text-secondary">No design standard yet. Create one from your token registry.</p>
      ) : null}

      {groups.map((group) => {
        const versions = rows.filter((row) => row.name === group);
        const latestPublished = versions.find((row) => row.status === "published");
        return (
          <div key={group} className="border-b border-border last:border-b-0">
            <p className="px-4 pt-3 text-xs font-semibold uppercase tracking-wide text-text-muted">{group}</p>
            <ul className="divide-y divide-border">
              {versions.map((item) => (
                <li key={item.id} className="flex flex-wrap items-center gap-3 px-4 py-2.5 text-sm">
                  <span className="font-mono text-xs">v{item.version}</span>
                  <span className={`rounded px-1.5 py-0.5 text-[11px] uppercase ${
                    item.status === "published" ? "bg-status-success/10 text-status-success" : "bg-bg-hover text-text-muted"
                  }`}>{item.status}</span>
                  <span className="font-mono text-[10px] text-text-muted" title={item.sha256}>{item.sha256.slice(0, 12)}…</span>
                  <span className="text-xs text-text-muted">{item.spec.tokens.length} tokens</span>
                  <span className="ml-auto flex gap-2">
                    {latestPublished && item.status === "draft" ? (
                      <Button variant="ghost" className="h-7 px-2 text-xs" onClick={() => setCompare({ base: latestPublished, next: item })}>
                        Compare
                      </Button>
                    ) : null}
                    <Button variant="secondary" className="h-7 px-2 text-xs" onClick={() => setBuilder(builderFrom(item))}>
                      Edit version
                    </Button>
                    {item.status === "draft" ? (
                      <Button
                        className="h-7 px-2 text-xs"
                        loading={publish.isPending && publish.variables === item.id}
                        onClick={() => publish.mutate(item.id)}
                      >
                        Publish
                      </Button>
                    ) : null}
                  </span>
                </li>
              ))}
            </ul>
          </div>
        );
      })}

      {compare ? (
        <div className="border-t border-border p-4">
          <div className="flex items-center justify-between">
            <p className="text-xs font-semibold">
              v{compare.base.version} → v{compare.next.version} token changes
            </p>
            <Button variant="ghost" className="h-7 px-2 text-xs" onClick={() => setCompare(null)}>Close</Button>
          </div>
          {diffTokens(compare.base, compare.next).length === 0 ? (
            <p className="mt-2 text-xs text-text-secondary">No token changes; other fields may still differ.</p>
          ) : (
            <ul className="mt-2 space-y-1 text-xs">
              {diffTokens(compare.base, compare.next).map((change) => (
                <li key={change.name} className="font-mono">
                  {change.name}: <span className="text-status-danger">{change.from ?? "—"}</span>{" → "}
                  <span className="text-status-success">{change.to ?? "—"}</span>
                </li>
              ))}
            </ul>
          )}
        </div>
      ) : null}
    </section>
  );
}

function buildSpec(state: BuilderState, registryTokens: { name: string; value: string; category: string }[], base: DesignStandard["spec"] | null) {
  let advanced: Record<string, unknown> = {};
  if (state.advanced.trim()) {
    advanced = JSON.parse(state.advanced);
  }
  const tokens = base?.tokens?.length ? base.tokens : registryTokens.map((token) => ({ name: token.name, value: token.value, category: token.category }));
  return {
    ...(base ?? {}),
    ...advanced,
    schema_version: "1",
    tokens,
    accessibility: lines(state.accessibility),
    responsive: lines(state.responsive),
    patterns: lines(state.patterns),
  } as DesignStandard["spec"];
}
