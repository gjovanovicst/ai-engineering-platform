"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { ApiError, createPreview, destroyPreview, listPreviews } from "@/lib/api";
import { useZodForm } from "@/lib/form";
import { previewSchema, type PreviewValues } from "@/lib/schemas";

const styles: Record<string, string> = {
  requested: "bg-status-warning/10 text-status-warning",
  starting: "bg-status-warning/10 text-status-warning",
  running: "bg-status-success/10 text-status-success",
  destroying: "bg-status-warning/10 text-status-warning",
  destroyed: "bg-bg-hover text-text-muted",
  failed: "bg-status-danger/10 text-status-danger",
};

export function PreviewsPanel({ orgID, projectID }: { orgID: string; projectID: string }) {
  const queryClient = useQueryClient();
  const [error, setError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useZodForm<PreviewValues>(previewSchema, { defaultValues: { image: "", port: "3000", ttl: "120" } });
  const previews = useQuery({ queryKey: ["previews", orgID, projectID], queryFn: () => listPreviews(orgID, projectID) });

  const create = useMutation({
    mutationFn: (values: PreviewValues) => createPreview(orgID, projectID, {
      name: "preview", image: values.image.trim(), port: Number(values.port), ttl_minutes: Number(values.ttl),
    }),
    onSuccess: () => {
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["previews", orgID, projectID] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not create the preview"),
  });
  const destroy = useMutation({
    mutationFn: (id: string) => destroyPreview(orgID, projectID, id),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["previews", orgID, projectID] }),
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not destroy the preview"),
  });

  return (
    <section className="mt-8 rounded-lg border border-border bg-bg-surface p-4">
      <h2 className="text-sm font-semibold">Preview environments</h2>
      <p className="mt-1 text-xs text-text-secondary">Ephemeral container per PR; destroyed when the TTL expires.</p>
      <form className="mt-3 flex flex-wrap items-end gap-2" onSubmit={handleSubmit((values) => create.mutate(values))}>
        <Field label="Image" htmlFor="preview-image" error={errors.image?.message}>
          <Input id="preview-image" aria-invalid={Boolean(errors.image)} {...register("image")} placeholder="ghcr.io/acme/app:sha" className="h-9 min-w-[220px] font-mono text-xs" />
        </Field>
        <Field label="Port" htmlFor="preview-port" error={errors.port?.message}>
          <Input id="preview-port" aria-invalid={Boolean(errors.port)} {...register("port")} placeholder="3000" className="h-9 w-20 text-xs" />
        </Field>
        <Field label="TTL minutes" htmlFor="preview-ttl" error={errors.ttl?.message}>
          <Input id="preview-ttl" aria-invalid={Boolean(errors.ttl)} {...register("ttl")} placeholder="120" className="h-9 w-20 text-xs" />
        </Field>
        <Button type="submit" variant="secondary" loading={create.isPending}>Create preview</Button>
      </form>
      {error ? <p role="alert" className="mt-2 text-xs text-status-danger">{error}</p> : null}
      {previews.isSuccess && (previews.data.items ?? []).length === 0 ? <p className="mt-3 text-xs text-text-secondary">No previews yet.</p> : null}
      {previews.isSuccess && (previews.data.items ?? []).length > 0 ? (
        <ul className="mt-3 divide-y divide-border rounded-md border border-border">
          {(previews.data.items ?? []).map((preview) => (
            <li key={preview.id} className="flex flex-wrap items-center gap-2 px-3 py-2 text-xs">
              <span className={`rounded px-1.5 py-0.5 ${styles[preview.status] ?? "bg-bg-hover"}`}>{preview.status}</span>
              <span className="font-medium">{preview.name}</span>
              <span className="flex-1 font-mono text-text-muted">{preview.image}</span>
              {preview.url ? <a href={preview.url} target="_blank" rel="noreferrer" className="text-brand hover:underline">open</a> : null}
              {preview.error ? <span className="text-status-danger">{preview.error}</span> : null}
              <Button variant="secondary" onClick={() => destroy.mutate(preview.id)}>Destroy</Button>
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  );
}
