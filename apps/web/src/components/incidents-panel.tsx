"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { ApiError, createIncident, listIncidents, resolveIncident } from "@/lib/api";
import { useZodForm } from "@/lib/form";
import { incidentSchema, type IncidentValues } from "@/lib/schemas";

export function IncidentsPanel({ orgID }: { orgID: string }) {
  const queryClient = useQueryClient();
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useZodForm<IncidentValues>(incidentSchema, { defaultValues: { title: "", severity: "sev2" } });
  const [error, setError] = useState<string | null>(null);
  const incidents = useQuery({ queryKey: ["incidents", orgID], queryFn: () => listIncidents(orgID) });
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["incidents", orgID] });

  const create = useMutation({
    mutationFn: (values: IncidentValues) => createIncident(orgID, { title: values.title.trim(), severity: values.severity }),
    onSuccess: () => { setError(null); invalidate(); },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not open the incident"),
  });
  const resolve = useMutation({
    mutationFn: (id: string) => resolveIncident(orgID, id),
    onSuccess: () => { setError(null); invalidate(); },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not resolve the incident"),
  });

  return (
    <section className="mt-6 rounded-lg border border-border bg-bg-surface p-4">
      <h2 className="text-sm font-semibold">Incidents</h2>
      <p className="mt-1 text-xs text-text-secondary">Production incidents drive the change-failure rate and MTTR.</p>
      <form className="mt-3 flex flex-wrap items-end gap-2" onSubmit={handleSubmit((values) => create.mutate(values))}>
        <Field label="Incident title" htmlFor="incident-title" error={errors.title?.message}>
          <Input id="incident-title" aria-invalid={Boolean(errors.title)} {...register("title")} placeholder="Checkout latency spike" className="h-9 min-w-[240px] text-xs" />
        </Field>
        <Field label="Severity" htmlFor="incident-severity">
          <select id="incident-severity" {...register("severity")} className="h-9 rounded-md border border-border bg-bg-surface px-2 text-xs">
            <option value="sev1">sev1</option><option value="sev2">sev2</option><option value="sev3">sev3</option><option value="sev4">sev4</option>
          </select>
        </Field>
        <Button type="submit" variant="secondary" loading={create.isPending} className="mt-5">Open incident</Button>
      </form>
      {error ? <p role="alert" className="mt-2 text-xs text-status-danger">{error}</p> : null}
      {incidents.isLoading ? <p className="mt-3 text-xs text-text-muted">Loading…</p> : null}
      {incidents.isError ? <p className="mt-3 text-xs text-status-danger">Could not load incidents.</p> : null}
      {incidents.isSuccess && incidents.data.data.length === 0 ? <p className="mt-3 text-xs text-text-muted">No incidents recorded.</p> : null}
      {incidents.isSuccess && incidents.data.data.length > 0 ? (
        <ul className="mt-3 divide-y divide-border rounded-md border border-border">
          {incidents.data.data.map((incident) => (
            <li key={incident.id} className="flex flex-wrap items-center gap-2 px-3 py-2 text-xs">
              <span className={`rounded px-1.5 py-0.5 ${incident.status === "open" ? "bg-status-danger/10 text-status-danger" : "bg-bg-hover text-text-muted"}`}>{incident.status}</span>
              <span className="rounded bg-bg-hover px-1.5 py-0.5">{incident.severity}</span>
              <span className="flex-1 font-medium">{incident.title}</span>
              <span className="text-text-muted">{incident.opened_at.slice(0, 16).replace("T", " ")}</span>
              {incident.status === "open" ? <Button variant="secondary" loading={resolve.isPending} onClick={() => resolve.mutate(incident.id)}>Resolve</Button> : null}
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  );
}
