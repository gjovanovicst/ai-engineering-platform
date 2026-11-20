"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { ApiError, createAPIKey, listAPIKeys, revokeAPIKey } from "@/lib/api";
import { useZodForm } from "@/lib/form";
import { apiKeySchema, type ApiKeyValues } from "@/lib/schemas";

export function APIKeysPanel({ orgID }: { orgID: string }) {
  const queryClient = useQueryClient();
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useZodForm<ApiKeyValues>(apiKeySchema, { defaultValues: { name: "", role: "member", days: "90" } });
  const [token, setToken] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const keys = useQuery({ queryKey: ["api-keys", orgID], queryFn: () => listAPIKeys(orgID) });
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["api-keys", orgID] });

  const create = useMutation({
    mutationFn: (input: ApiKeyValues) => createAPIKey(orgID, input.name.trim(), input.role, Number(input.days) || 0),
    onSuccess: (key) => { setToken(key.token); reset({ name: "" }); setError(null); invalidate(); },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not create the key"),
  });
  const revoke = useMutation({
    mutationFn: (id: string) => revokeAPIKey(orgID, id),
    onSuccess: () => { setError(null); invalidate(); },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not revoke the key"),
  });

  return (
    <section className="rounded-lg border border-border bg-bg-surface p-4">
      <h2 className="text-sm font-semibold">API keys</h2>
      <p className="mt-1 text-xs text-text-secondary">Service accounts for CI and automation. The token is shown once.</p>
      <form className="mt-3 flex flex-wrap items-start gap-2" onSubmit={handleSubmit((values) => create.mutate(values))}>
        <Field label="Key name" htmlFor="key-name" error={errors.name?.message}>
          <Input id="key-name" aria-invalid={Boolean(errors.name)} {...register("name")} placeholder="ci-runner" className="h-9 min-w-[200px]" />
        </Field>
        <Field label="Role" htmlFor="key-role">
          <select id="key-role" {...register("role")} className="h-9 rounded-md border border-border bg-bg-surface px-2 text-xs">
            <option value="admin">admin</option><option value="member">member</option><option value="viewer">viewer</option>
          </select>
        </Field>
        <Field label="Expires in days" htmlFor="key-days" error={errors.days?.message} help="0 = never">
          <Input id="key-days" aria-invalid={Boolean(errors.days)} {...register("days")} placeholder="90" className="h-9 w-24" />
        </Field>
        <Button type="submit" variant="secondary" loading={create.isPending} className="mt-[22px]">Create key</Button>
      </form>
      {token ? (
        <div className="mt-3 rounded-md border border-status-warning/40 bg-status-warning/10 p-3 text-xs">
          <p className="font-medium">Copy this token now — it will not be shown again.</p>
          <code className="mt-1 block break-all font-mono">{token}</code>
        </div>
      ) : null}
      {error ? <p role="alert" className="mt-2 text-xs text-status-danger">{error}</p> : null}
      {keys.isSuccess && keys.data.items.length > 0 ? (
        <ul className="mt-3 divide-y divide-border rounded-md border border-border">
          {keys.data.items.map((key) => (
            <li key={key.id} className="flex flex-wrap items-center gap-2 px-3 py-2 text-xs">
              <span className="font-medium">{key.name}</span>
              <span className="rounded bg-bg-hover px-1.5 py-0.5">{key.role}</span>
              <span className="flex-1 text-text-muted">{key.revoked_at ? "revoked" : key.last_used_at ? `used ${key.last_used_at.slice(0, 19)}` : "never used"}</span>
              <span className="text-text-muted">{key.expires_at ? `expires ${key.expires_at.slice(0, 10)}` : "no expiry"}</span>
              {!key.revoked_at ? <Button variant="secondary" onClick={() => revoke.mutate(key.id)}>Revoke</Button> : null}
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  );
}
