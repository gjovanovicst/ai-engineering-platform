"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { SidePanel } from "@/components/ui/side-panel";
import { useZodForm } from "@/lib/form";
import { designTokenSchema, tokenCategories, type DesignTokenValues } from "@/lib/schemas";
import {
  ApiError,
  exportDesignTokens,
  createDesignToken,
  deleteDesignToken,
  listDesignTokens,
  seedDesignRegistry,
  updateDesignToken,
} from "@/lib/api";



export function TokenRegistry({ orgID }: { orgID: string }) {
  const queryClient = useQueryClient();
  const [showForm, setShowForm] = useState(false);
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useZodForm<DesignTokenValues>(designTokenSchema, {
    defaultValues: { name: "", value: "", category: "color" },
  });
  const [edits, setEdits] = useState<Record<string, string>>({});
  const [error, setError] = useState<string | null>(null);

  const tokens = useQuery({ queryKey: ["design-tokens", orgID], queryFn: () => listDesignTokens(orgID) });

  const seed = useMutation({
    mutationFn: () => seedDesignRegistry(orgID),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["design-tokens", orgID] });
      queryClient.invalidateQueries({ queryKey: ["component-entries", orgID] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not seed the registry"),
  });

  const create = useMutation({
    mutationFn: (input: DesignTokenValues) => createDesignToken(orgID, input),
    onSuccess: () => {
      reset({ name: "", value: "", category: "color" });
      setShowForm(false);
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["design-tokens", orgID] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not create the token"),
  });

  const update = useMutation({
    mutationFn: (input: { id: string; value: string; category: string }) =>
      updateDesignToken(orgID, input.id, { value: input.value, category: input.category }),
    onSuccess: () => {
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["design-tokens", orgID] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not save the token"),
  });

  const remove = useMutation({
    mutationFn: (id: string) => deleteDesignToken(orgID, id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["design-tokens", orgID] }),
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not delete the token"),
  });

  return (
    <section className="rounded-lg border border-border bg-bg-surface">
      <header className="flex items-center justify-between border-b border-border p-4">
        <div>
          <h2 className="text-sm font-semibold">Token registry</h2>
          <p className="mt-1 text-xs text-text-secondary">
            Tokens your organisation standardises on. Changes are recorded with the workspace.
          </p>
        </div>
        <div className="flex gap-2">
          <Button variant="secondary" loading={seed.isPending} onClick={() => seed.mutate()}>Load built-in defaults</Button>
          <Button onClick={() => setShowForm(true)}>+ Add token</Button>
        </div>
      </header>

      {error ? (
        <p role="alert" className="border-b border-border p-3 text-xs text-status-danger">
          {error}
        </p>
      ) : null}

      {tokens.isSuccess && tokens.data.data.length === 0 ? (
        <p className="p-4 text-sm text-text-secondary">
          No tokens yet. Load the built-in defaults or add the first one.
        </p>
      ) : null}

      <ul className="divide-y divide-border">
        {tokens.isSuccess
          ? tokens.data.data.map((token) => (
              <li key={token.id} className="flex flex-wrap items-center gap-2 px-4 py-2 text-sm">
                <span className="w-40 font-mono text-xs">{token.name}</span>
                <span className="rounded bg-bg-hover px-1.5 py-0.5 text-[11px] uppercase text-text-muted">
                  {token.category}
                </span>
                <Input
                  aria-label={`${token.name} value`}
                  placeholder="value"
                  value={edits[token.id] ?? token.value}
                  onChange={(e) => setEdits({ ...edits, [token.id]: e.target.value })}
                  className="max-w-[200px] font-mono text-xs"
                />
                <Button
                  variant="secondary"
                  className="h-7 px-2 text-xs"
                  loading={update.isPending && update.variables?.id === token.id}
                  onClick={() =>
                    update.mutate({ id: token.id, value: edits[token.id] ?? token.value, category: token.category })
                  }
                >
                  Save
                </Button>
                <Button
                  variant="danger"
                  className="h-7 px-2 text-xs"
                  loading={remove.isPending && remove.variables === token.id}
                  onClick={() => remove.mutate(token.id)}
                >
                  Delete
                </Button>
              </li>
            ))
          : null}
      </ul>

      {showForm ? (
        <SidePanel
          dirty
          title="Add token"
          description="Tokens are captured into the next design standard version."
          onClose={() => setShowForm(false)}
          footer={
            <>
              <Button variant="secondary" onClick={() => setShowForm(false)}>Cancel</Button>
              <Button type="submit" form="token-form" loading={create.isPending}>Add token</Button>
            </>
          }
        >
      <form
        id="token-form"
        className="space-y-4 p-5"
        onSubmit={handleSubmit((values) => create.mutate(values))}
      >
        <Field label="Name" htmlFor="token-name" error={errors.name?.message}>
          <Input id="token-name" aria-invalid={Boolean(errors.name)} {...register("name")} placeholder="e.g. color-brand" />
        </Field>
        <Field label="Value" htmlFor="token-value" error={errors.value?.message}>
          <Input id="token-value" aria-invalid={Boolean(errors.value)} {...register("value")} placeholder="e.g. #2563eb or 37 99 235" />
        </Field>
        <Field label="Category" htmlFor="token-category">
          <select
            id="token-category"
            {...register("category")}
            className="h-9 w-full rounded-md border border-border bg-bg-surface px-3 text-sm"
          >
            {tokenCategories.map((category) => (
              <option key={category} value={category}>
                {category}
              </option>
            ))}
          </select>
        </Field>
      </form>
        </SidePanel>
      ) : null}
    </section>
  );
}

async function downloadTokens(orgID: string, format: "css" | "scss" | "ts", setError: (v: string | null) => void) {
  try {
    const body = await exportDesignTokens(orgID, format);
    const url = URL.createObjectURL(new Blob([body], { type: "text/plain" }));
    const link = document.createElement("a");
    link.href = url;
    link.download = `design-tokens.${format === "ts" ? "ts" : format}`;
    link.click();
    setTimeout(() => URL.revokeObjectURL(url), 60000);
    setError(null);
  } catch (err) {
    setError(err instanceof ApiError ? err.message : "Could not export tokens");
  }
}
