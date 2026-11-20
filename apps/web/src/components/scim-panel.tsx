"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { ApiError, createScimToken, listScimTokens, revokeScimToken } from "@/lib/api";
import { useZodForm } from "@/lib/form";
import { scimSchema, type ScimValues } from "@/lib/schemas";

export function ScimPanel({ orgID }: { orgID: string }) {
  const queryClient = useQueryClient();
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useZodForm<ScimValues>(scimSchema, { defaultValues: { name: "" } });
  const [token, setToken] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const tokens = useQuery({ queryKey: ["scim-tokens", orgID], queryFn: () => listScimTokens(orgID) });
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["scim-tokens", orgID] });

  const create = useMutation({
    mutationFn: (values: ScimValues) => createScimToken(orgID, values.name.trim()),
    onSuccess: (created) => { setToken(created.token); setError(null); invalidate(); },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not create the token"),
  });
  const revoke = useMutation({
    mutationFn: (id: string) => revokeScimToken(orgID, id),
    onSuccess: () => { setError(null); invalidate(); },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not revoke the token"),
  });

  return (
    <section className="rounded-lg border border-border bg-bg-surface p-4">
      <h2 className="text-sm font-semibold">SCIM provisioning</h2>
      <p className="mt-1 text-xs text-text-secondary">
        Bearer token for <code>/scim/v2/Users</code> (read-only). The token is shown once; revoke it to disable access.
      </p>
      <form className="mt-3 flex flex-wrap items-end gap-2" onSubmit={handleSubmit((values) => create.mutate(values))}>
        <Field label="SCIM token name" htmlFor="scim-name" error={errors.name?.message}>
          <Input id="scim-name" aria-invalid={Boolean(errors.name)} {...register("name")} placeholder="okta" className="h-9 min-w-[200px] text-xs" />
        </Field>
        <Button type="submit" variant="secondary" loading={create.isPending} className="mt-5">Create token</Button>
      </form>
      {token ? (
        <div className="mt-3 rounded-md border border-status-warning/40 bg-status-warning/10 p-3 text-xs">
          <p className="font-medium">Copy this token now — it will not be shown again.</p>
          <code className="mt-1 block break-all font-mono">{token}</code>
        </div>
      ) : null}
      {error ? <p role="alert" className="mt-2 text-xs text-status-danger">{error}</p> : null}
      {tokens.isSuccess && tokens.data.data.length > 0 ? (
        <ul className="mt-3 divide-y divide-border rounded-md border border-border">
          {tokens.data.data.map((item) => (
            <li key={item.id} className="flex flex-wrap items-center gap-2 px-3 py-2 text-xs">
              <span className="font-medium">{item.name}</span>
              <span className="flex-1 text-text-muted">{item.revoked_at ? "revoked" : `created ${item.created_at.slice(0, 10)}`}</span>
              {!item.revoked_at ? <Button variant="secondary" onClick={() => revoke.mutate(item.id)}>Revoke</Button> : null}
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  );
}
