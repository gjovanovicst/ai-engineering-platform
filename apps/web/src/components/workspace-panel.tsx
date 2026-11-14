"use client";

import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useRouter } from "next/navigation";

import { Button } from "@/components/ui/button";
import { ApiError, deleteOrg, getOrg, logout, renameOrg, setOrgRegion } from "@/lib/api";

export function WorkspacePanel({ orgID }: { orgID: string }) {
  const router = useRouter();
  const queryClient = useQueryClient();
  const org = useQuery({ queryKey: ["org", orgID], queryFn: () => getOrg(orgID) });
  const [name, setName] = useState("");
  const [confirmSlug, setConfirmSlug] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [region, setRegion] = useState("eu-central");
  const [regionNote, setRegionNote] = useState<string | null>(null);

  useEffect(() => {
    if (org.data) {
      setName(org.data.name);
      setRegion(org.data.region ?? "eu-central");
    }
  }, [org.data]);

  const save = async () => {
    setBusy(true);
    setError(null);
    try {
      await renameOrg(orgID, name);
      await queryClient.invalidateQueries({ queryKey: ["org", orgID] });
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not rename the workspace");
    } finally {
      setBusy(false);
    }
  };

  const saveRegion = async () => {
    setRegionNote(null);
    try {
      await setOrgRegion(orgID, region);
      await queryClient.invalidateQueries({ queryKey: ["org", orgID] });
      setRegionNote("residency label saved");
    } catch (err) {
      setRegionNote(err instanceof ApiError ? err.message : "Could not save the region");
    }
  };

  const remove = async () => {
    setBusy(true);
    setError(null);
    try {
      await deleteOrg(orgID, confirmSlug);
      await logout();
      router.replace("/login");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not delete the workspace");
      setBusy(false);
    }
  };

  if (org.isPending) return <p className="text-sm text-text-muted">Loading workspace…</p>;
  if (org.isError || !org.data) return <p role="alert" className="text-sm text-status-danger">Could not load the workspace.</p>;

  return (
    <div className="space-y-6">
      <section className="rounded-lg border border-border bg-bg-surface p-4">
        <h2 className="text-sm font-semibold">Workspace</h2>
        <p className="mt-1 text-xs text-text-secondary">
          Slug <span className="font-mono">{org.data.slug}</span> is permanent.
        </p>
        <div className="mt-3 flex flex-wrap items-end gap-2">
          <label className="text-xs text-text-muted" htmlFor="org-name">Name</label>
          <input
            id="org-name"
            value={name}
            onChange={(e) => setName(e.target.value)}
            className="h-9 min-w-[240px] flex-1 rounded-md border border-border bg-bg-surface px-3 text-sm"
          />
          <Button loading={busy} onClick={save}>Save</Button>
        </div>
      </section>

      <section className="rounded-lg border border-border bg-bg-surface p-4">
        <h2 className="text-sm font-semibold">Residency</h2>
        <p className="mt-1 text-xs text-text-secondary">
          Region label for this workspace. Storage routing is not enforced by this deployment yet.
        </p>
        <div className="mt-3 flex flex-wrap items-center gap-2">
          <select aria-label="Region" value={region} onChange={(e) => setRegion(e.target.value)} className="h-9 rounded-md border border-border bg-bg-surface px-2 text-xs">
            <option value="eu-central">eu-central</option>
            <option value="eu-west">eu-west</option>
            <option value="us-east">us-east</option>
            <option value="us-west">us-west</option>
            <option value="ap-southeast">ap-southeast</option>
          </select>
          <Button variant="secondary" onClick={saveRegion}>Save region</Button>
          {regionNote ? <span className="text-xs text-text-secondary">{regionNote}</span> : null}
        </div>
      </section>

      <section className="rounded-lg border border-status-danger/40 bg-status-danger/5 p-4">
        <h2 className="text-sm font-semibold text-status-danger">Delete workspace</h2>
        <p className="mt-1 text-xs text-text-secondary">
          Removes every project, task and run. Type the slug <span className="font-mono">{org.data.slug}</span> to confirm.
        </p>
        <div className="mt-3 flex flex-wrap items-center gap-2">
          <input
            aria-label="Confirm slug"
            value={confirmSlug}
            onChange={(e) => setConfirmSlug(e.target.value)}
            className="h-9 min-w-[240px] flex-1 rounded-md border border-border bg-bg-surface px-3 text-sm"
          />
          <Button variant="danger" loading={busy} onClick={remove}>Delete</Button>
        </div>
      </section>

      {error ? <p role="alert" className="text-sm text-status-danger">{error}</p> : null}
    </div>
  );
}
