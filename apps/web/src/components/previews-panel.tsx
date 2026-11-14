"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { ApiError, createPreview, destroyPreview, listPreviews } from "@/lib/api";

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
  const [form, setForm] = useState({ name: "preview", image: "", port: "3000", ttl: "120" });
  const [error, setError] = useState<string | null>(null);
  const previews = useQuery({ queryKey: ["previews", orgID, projectID], queryFn: () => listPreviews(orgID, projectID) });

  const create = useMutation({
    mutationFn: () => createPreview(orgID, projectID, {
      name: form.name.trim(), image: form.image.trim(), port: Number(form.port), ttl_minutes: Number(form.ttl),
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
      <div className="mt-3 flex flex-wrap items-end gap-2">
        <input aria-label="Image" value={form.image} onChange={(e) => setForm({ ...form, image: e.target.value })} placeholder="ghcr.io/acme/app:sha" className="h-9 min-w-[220px] flex-1 rounded-md border border-border bg-bg-surface px-3 font-mono text-xs" />
        <input aria-label="Port" value={form.port} onChange={(e) => setForm({ ...form, port: e.target.value })} placeholder="3000" className="h-9 w-20 rounded-md border border-border bg-bg-surface px-3 text-xs" />
        <input aria-label="TTL minutes" value={form.ttl} onChange={(e) => setForm({ ...form, ttl: e.target.value })} placeholder="120" className="h-9 w-20 rounded-md border border-border bg-bg-surface px-3 text-xs" />
        <Button variant="secondary" loading={create.isPending} onClick={() => create.mutate()}>Create preview</Button>
      </div>
      {error ? <p role="alert" className="mt-2 text-xs text-status-danger">{error}</p> : null}
      {previews.isSuccess && previews.data.items.length === 0 ? <p className="mt-3 text-xs text-text-secondary">No previews yet.</p> : null}
      {previews.isSuccess && previews.data.items.length > 0 ? (
        <ul className="mt-3 divide-y divide-border rounded-md border border-border">
          {previews.data.items.map((preview) => (
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
