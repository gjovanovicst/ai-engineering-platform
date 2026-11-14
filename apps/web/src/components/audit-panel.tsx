"use client";

import { useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { ApiError, exportAudit, getOrg, listAudit, setLegalHold, verifyAuditWitness, writeAuditWitness } from "@/lib/api";

export function AuditPanel({ orgID }: { orgID: string }) {
  const audit = useQuery({ queryKey: ["audit", orgID], queryFn: () => listAudit(orgID) });
  const org = useQuery({ queryKey: ["org", orgID], queryFn: () => getOrg(orgID) });
  const [exporting, setExporting] = useState(false);
  const [exportNote, setExportNote] = useState<string | null>(null);
  const [exportError, setExportError] = useState<string | null>(null);
  const [witnessNote, setWitnessNote] = useState<string | null>(null);
  const [witnessBusy, setWitnessBusy] = useState(false);
  const [holdNote, setHoldNote] = useState<string | null>(null);

  async function toggleHold() {
    try {
      const updated = await setLegalHold(orgID, !(org.data?.legal_hold ?? false));
      setHoldNote(updated.legal_hold ? "legal hold set: retention will not archive this org's run evidence" : "legal hold cleared");
      org.refetch();
    } catch (err) {
      setHoldNote(err instanceof ApiError ? err.message : "Could not update the legal hold");
    }
  }

  async function writeWitness() {
    setWitnessBusy(true);
    try {
      const witness = await writeAuditWitness(orgID);
      setWitnessNote(`witness written at row ${witness.row_count}, hash ${witness.last_hash.slice(0, 12)}…`);
    } catch (err) {
      setWitnessNote(err instanceof ApiError ? err.message : "Could not write the witness");
    } finally {
      setWitnessBusy(false);
    }
  }

  async function verifyWitness() {
    setWitnessBusy(true);
    try {
      const result = await verifyAuditWitness(orgID);
      setWitnessNote(result.valid ? "witness matches the audit chain" : "WITNESS MISMATCH: the audit chain changed");
    } catch (err) {
      setWitnessNote(err instanceof ApiError ? err.message : "Could not verify the witness");
    } finally {
      setWitnessBusy(false);
    }
  }

  async function exportLog() {
    setExporting(true);
    setExportError(null);
    try {
      const result = await exportAudit(orgID);
      const url = URL.createObjectURL(result.blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = "audit.ndjson";
      link.click();
      setTimeout(() => URL.revokeObjectURL(url), 60000);
      setExportNote(result.signature ? `export downloaded, ${result.signature}` : "export downloaded (unsigned)");
    } catch (err) {
      setExportError(err instanceof ApiError ? err.message : "Could not export the audit log");
    } finally {
      setExporting(false);
    }
  }

  return (
    <section className="rounded-lg border border-border bg-bg-surface">
      <header className="border-b border-border p-4">
        <h2 className="text-sm font-semibold">Audit log</h2>
        <p className="mt-1 text-xs text-text-secondary">Append-only record of security-relevant actions.</p>
        <div className="mt-3 flex flex-wrap items-center gap-2">
          <Button variant="secondary" loading={exporting} onClick={exportLog}>Export NDJSON</Button>
          <Button variant="secondary" onClick={toggleHold}>{org.data?.legal_hold ? "Clear legal hold" : "Set legal hold"}</Button>
          <Button variant="secondary" loading={witnessBusy} onClick={writeWitness}>Write witness</Button>
          <Button variant="secondary" loading={witnessBusy} onClick={verifyWitness}>Verify witness</Button>
          {exportNote ? <span className="text-xs text-text-secondary">{exportNote}</span> : null}
          {exportError ? <span role="alert" className="text-xs text-status-danger">{exportError}</span> : null}
          {witnessNote ? <span className="text-xs text-text-secondary">{witnessNote}</span> : null}
          {holdNote ? <span className="text-xs text-text-secondary">{holdNote}</span> : null}
        </div>
      </header>
      {audit.isPending ? <p className="p-4 text-sm text-text-muted">Loading…</p> : null}
      {audit.isError ? (
        <p role="alert" className="p-4 text-sm text-status-danger">Could not load the audit log.</p>
      ) : null}
      {audit.isSuccess && audit.data.data.length === 0 ? (
        <p className="p-4 text-sm text-text-secondary">No entries yet.</p>
      ) : null}
      {audit.isSuccess && audit.data.data.length > 0 ? (
        <ul className="divide-y divide-border">
          {audit.data.data.map((entry) => (
            <li key={entry.id} className="flex flex-wrap items-center gap-3 px-4 py-2 text-xs">
              <span className="font-mono text-text-muted">{entry.created_at.replace("T", " ").slice(0, 19)}</span>
              <span className="font-medium">{entry.action}</span>
              <span className="text-text-secondary">{entry.target}</span>
              {entry.hash ? <span className="font-mono text-text-muted" title={entry.hash}>{entry.hash.slice(0, 12)}…</span> : null}
              <span className="ml-auto rounded bg-bg-hover px-1.5 py-0.5 uppercase text-text-muted">
                {entry.actor_kind}
              </span>
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  );
}
