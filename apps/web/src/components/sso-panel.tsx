"use client";

import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { ApiError, fetchSSO, getSSO, saveSSO } from "@/lib/api";

export function SSOPanel({ orgID }: { orgID: string }) {
  const queryClient = useQueryClient();
  const config = useQuery({ queryKey: ["sso", orgID], queryFn: () => getSSO(orgID), retry: false });
  const [form, setForm] = useState({ entity_id: "", metadata_url: "", sso_url: "", certificate_pem: "", enabled: false });
  const [note, setNote] = useState<string | null>(null);

  useEffect(() => {
    if (config.data) setForm(config.data);
  }, [config.data]);

  async function fetchMetadata() {
    try {
      const updated = await fetchSSO(orgID, form.metadata_url);
      setForm(updated);
      setNote("IdP metadata fetched");
      queryClient.invalidateQueries({ queryKey: ["sso", orgID] });
    } catch (err) {
      setNote(err instanceof ApiError ? err.message : "Could not fetch metadata");
    }
  }

  async function save() {
    try {
      await saveSSO(orgID, form);
      setNote("SSO configuration saved");
      queryClient.invalidateQueries({ queryKey: ["sso", orgID] });
    } catch (err) {
      setNote(err instanceof ApiError ? err.message : "Could not save SSO");
    }
  }

  return (
    <section className="rounded-lg border border-border bg-bg-surface p-4">
      <h2 className="text-sm font-semibold">SAML SSO</h2>
      <p className="mt-1 text-xs text-text-secondary">
        IdP metadata for this workspace. <strong>SAML login is not implemented in this build</strong>; enabling only records the configuration.
      </p>
      <div className="mt-3 grid gap-2">
        <input aria-label="Entity ID" value={form.entity_id} onChange={(e) => setForm({ ...form, entity_id: e.target.value })} placeholder="https://idp.example.com/metadata" className="h-9 rounded-md border border-border bg-bg-surface px-3 text-xs" />
        <input aria-label="Metadata URL" value={form.metadata_url} onChange={(e) => setForm({ ...form, metadata_url: e.target.value })} placeholder="https://idp.example.com/saml/metadata" className="h-9 rounded-md border border-border bg-bg-surface px-3 text-xs" />
        <input aria-label="SSO URL" value={form.sso_url} onChange={(e) => setForm({ ...form, sso_url: e.target.value })} placeholder="https://idp.example.com/sso" className="h-9 rounded-md border border-border bg-bg-surface px-3 text-xs" />
        <textarea aria-label="Certificate PEM" value={form.certificate_pem} onChange={(e) => setForm({ ...form, certificate_pem: e.target.value })} rows={3} placeholder="-----BEGIN CERTIFICATE-----" className="rounded-md border border-border bg-bg-surface px-3 py-2 font-mono text-xs" />
        <label className="flex items-center gap-2 text-xs text-text-secondary">
          <input type="checkbox" checked={form.enabled} onChange={(e) => setForm({ ...form, enabled: e.target.checked })} />
          mark SSO as enabled (metadata only)
        </label>
      </div>
      <div className="mt-3 flex items-center gap-2">
        <Button variant="secondary" onClick={fetchMetadata}>Fetch metadata</Button>
        <Button variant="secondary" onClick={save}>Save SSO</Button>
        {note ? <span className="text-xs text-text-secondary">{note}</span> : null}
      </div>
    </section>
  );
}
