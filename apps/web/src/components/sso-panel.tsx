"use client";

import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { ApiError, fetchSSO, getSSO, saveSSO } from "@/lib/api";
import { useZodForm } from "@/lib/form";
import { ssoSchema, type SSOValues } from "@/lib/schemas";

export function SSOPanel({ orgID }: { orgID: string }) {
  const queryClient = useQueryClient();
  const config = useQuery({ queryKey: ["sso", orgID], queryFn: () => getSSO(orgID), retry: false });
  const {
    register,
    handleSubmit,
    reset,
    getValues,
    formState: { errors, isDirty },
  } = useZodForm<SSOValues>(ssoSchema, {
    defaultValues: { entity_id: "", metadata_url: "", sso_url: "", certificate_pem: "", enabled: false },
  });
  const [note, setNote] = useState<string | null>(null);

  useEffect(() => {
    if (config.data && !isDirty) reset(config.data);
  }, [config.data, isDirty, reset]);

  async function fetchMetadata() {
    try {
      const updated = await fetchSSO(orgID, getValues("metadata_url"));
      reset(updated);
      setNote("IdP metadata fetched");
      queryClient.invalidateQueries({ queryKey: ["sso", orgID] });
    } catch (err) {
      setNote(err instanceof ApiError ? err.message : "Could not fetch metadata");
    }
  }

  const save = handleSubmit(async (values) => {
    try {
      await saveSSO(orgID, values);
      setNote("SSO configuration saved");
      queryClient.invalidateQueries({ queryKey: ["sso", orgID] });
    } catch (err) {
      setNote(err instanceof ApiError ? err.message : "Could not save SSO");
    }
  });

  return (
    <section className="rounded-lg border border-border bg-bg-surface p-4">
      <h2 className="text-sm font-semibold">SAML SSO</h2>
      <p className="mt-1 text-xs text-text-secondary">
        IdP metadata for this workspace. <strong>SAML login is not implemented in this build</strong>; enabling only records the configuration.
      </p>
      <form id="sso-form" className="mt-3 grid gap-2" onSubmit={save}>
        <Field label="Entity ID" htmlFor="sso-entity" error={errors.entity_id?.message}>
          <Input id="sso-entity" aria-invalid={Boolean(errors.entity_id)} {...register("entity_id")} placeholder="https://idp.example.com/metadata" className="h-9 text-xs" />
        </Field>
        <Field label="Metadata URL" htmlFor="sso-metadata" error={errors.metadata_url?.message}>
          <Input id="sso-metadata" aria-invalid={Boolean(errors.metadata_url)} {...register("metadata_url")} placeholder="https://idp.example.com/saml/metadata" className="h-9 text-xs" />
        </Field>
        <Field label="SSO URL" htmlFor="sso-url" error={errors.sso_url?.message}>
          <Input id="sso-url" aria-invalid={Boolean(errors.sso_url)} {...register("sso_url")} placeholder="https://idp.example.com/sso" className="h-9 text-xs" />
        </Field>
        <Field label="Certificate PEM" htmlFor="sso-cert" error={errors.certificate_pem?.message}>
          <textarea id="sso-cert" rows={3} aria-invalid={Boolean(errors.certificate_pem)} {...register("certificate_pem")} placeholder="-----BEGIN CERTIFICATE-----" className="rounded-md border border-border bg-bg-surface px-3 py-2 font-mono text-xs" />
        </Field>
        <label className="flex items-center gap-2 text-xs text-text-secondary">
          <input type="checkbox" {...register("enabled")} />
          mark SSO as enabled (metadata only)
        </label>
        <div className="flex items-center gap-2">
          <Button type="button" variant="secondary" onClick={fetchMetadata}>Fetch metadata</Button>
          <Button type="submit" variant="secondary">Save SSO</Button>
          {note ? <span className="text-xs text-text-secondary">{note}</span> : null}
        </div>
      </form>
    </section>
  );
}
