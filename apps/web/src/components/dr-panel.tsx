"use client";

import Link from "next/link";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { ApiError, createRestoreRequest, getDRStatus, listRestoreRequests } from "@/lib/api";
import { useZodForm } from "@/lib/form";
import { restoreSchema, type RestoreValues } from "@/lib/schemas";

function hours(seconds: number) {
  return `${(seconds / 3600).toFixed(1)} h`;
}

export function DRPanel({ orgID }: { orgID: string }) {
  const queryClient = useQueryClient();
  const dr = useQuery({ queryKey: ["dr", orgID], queryFn: () => getDRStatus(orgID), refetchInterval: 30000 });
  const restores = useQuery({ queryKey: ["restores", orgID], queryFn: () => listRestoreRequests(orgID), refetchInterval: 15000 });
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useZodForm<RestoreValues>(restoreSchema, { defaultValues: { target: "" } });
  const [note, setNote] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const requestRestore = handleSubmit(async (values) => {
    setNote(null);
    setBusy(true);
    try {
      await createRestoreRequest(orgID, new Date(values.target).toISOString());
      setNote("Restore request queued");
      queryClient.invalidateQueries({ queryKey: ["restores", orgID] });
    } catch (err) {
      setNote(err instanceof ApiError ? err.message : "Could not queue the restore");
    } finally {
      setBusy(false);
    }
  });

  return (
    <section className="rounded-lg border border-border bg-bg-surface p-4">
      <h2 className="text-sm font-semibold">Backups & disaster recovery</h2>
      <p className="mt-1 text-xs text-text-secondary">
        WAL/base health from Patroni and MinIO. A restore request is executed by the operator runner on the host.
      </p>
      {dr.data ? (
        <div className="mt-3 grid gap-2 text-xs sm:grid-cols-2">
          <div className={`rounded border p-2 ${dr.data.wal_ok ? "border-status-success/40" : "border-status-danger/40"}`}>
            <p className="font-medium">WAL archive</p>
            <p className="text-text-secondary">archived {dr.data.wal_archived}, failed {dr.data.wal_failed}</p>
            <p className="text-text-secondary">last success {hours(dr.data.wal_age_seconds)} ago</p>
          </div>
          <div className={`rounded border p-2 ${dr.data.base_ok ? "border-status-success/40" : "border-status-danger/40"}`}>
            <p className="font-medium">Base backup</p>
            <p className="text-text-secondary">{dr.data.base_backup || "no heartbeat"}</p>
            <p className="text-text-secondary">{dr.data.base_age_seconds ? `${hours(dr.data.base_age_seconds)} ago` : "unknown"}</p>
          </div>
        </div>
      ) : null}
      {dr.data ? (
        <div className="mt-2 grid gap-2 text-xs sm:grid-cols-3">
          <div className="rounded border border-border p-2">
            <p className="font-medium">Restores</p>
            <p className="text-text-secondary">succeeded {dr.data.restore_succeeded}, failed {dr.data.restore_failed}</p>
          </div>
          <div className="rounded border border-border p-2">
            <p className="font-medium">Last restore</p>
            <p className="text-text-secondary">{dr.data.last_restore_age_seconds ? `${hours(dr.data.last_restore_age_seconds)} ago` : "never"}</p>
          </div>
          <div className="rounded border border-border p-2">
            <p className="font-medium">Offsite mirror</p>
            <p className="text-text-secondary">{dr.data.offsite_age_seconds ? `${hours(dr.data.offsite_age_seconds)} ago` : "never"}</p>
          </div>
          <div className="rounded border border-border p-2">
            <p className="font-medium">Restore duration</p>
            <p className="text-text-secondary">{dr.data.avg_restore_seconds ? `${(dr.data.avg_restore_seconds / 60).toFixed(1)} min avg` : "na"}</p>
          </div>
        </div>
      ) : null}
      <form className="mt-4 flex flex-wrap items-start gap-2" onSubmit={requestRestore}>
        <Field label="Target time" htmlFor="restore-target" error={errors.target?.message}>
          <Input id="restore-target" type="datetime-local" aria-invalid={Boolean(errors.target)} {...register("target")} className="h-9" />
        </Field>
        <Button type="submit" variant="secondary" loading={busy}>Request restore</Button>
        <Link href="/docs" className="text-xs text-brand underline">Runbook</Link>
        {note ? <span className="text-xs text-text-secondary">{note}</span> : null}
      </form>
      <ul className="mt-4 grid gap-1 text-xs">
        {(restores.data?.data ?? []).map((item) => (
          <li key={item.id} className="flex flex-wrap items-center gap-2 rounded border border-border px-2 py-1">
            <span className="font-mono text-[11px]">{item.target_time}</span>
            <span className={item.status === "succeeded" ? "text-status-success" : item.status === "failed" ? "text-status-danger" : "text-text-secondary"}>{item.status}</span>
            {item.detail ? <span className="text-text-muted">{item.detail}</span> : null}
          </li>
        ))}
        {restores.data?.data.length === 0 ? <li className="text-text-muted">No restore requests.</li> : null}
      </ul>
    </section>
  );
}
