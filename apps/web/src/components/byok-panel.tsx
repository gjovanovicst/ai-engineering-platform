"use client";

import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { ApiError, disableOrgCrypto, getOrgCrypto, setOrgCrypto } from "@/lib/api";
import { useZodForm } from "@/lib/form";
import { byokSchema, type ByokValues } from "@/lib/schemas";

export function BYOKPanel({ orgID }: { orgID: string }) {
  const queryClient = useQueryClient();
  const config = useQuery({ queryKey: ["org-crypto", orgID], queryFn: () => getOrgCrypto(orgID), retry: false });
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useZodForm<ByokValues>(byokSchema, { defaultValues: { kmsURL: "", keyID: "", token: "" } });
  const [note, setNote] = useState<string | null>(null);

  useEffect(() => {
    if (config.data) {
      reset({ kmsURL: config.data.kms_url, keyID: config.data.key_id, token: "" });
    }
  }, [config.data]);

  const save = handleSubmit(async (values) => {
    try {
      await setOrgCrypto(orgID, values.kmsURL, values.keyID, values.token);
      setNote("BYOK enabled");
      queryClient.invalidateQueries({ queryKey: ["org-crypto", orgID] });
    } catch (err) {
      setNote(err instanceof ApiError ? err.message : "Could not save BYOK");
    }
  });

  async function disable() {
    try {
      await disableOrgCrypto(orgID);
      setNote("BYOK disabled");
      queryClient.invalidateQueries({ queryKey: ["org-crypto", orgID] });
    } catch (err) {
      setNote(err instanceof ApiError ? err.message : "Could not disable BYOK");
    }
  }

  return (
    <section className="rounded-lg border border-border bg-bg-surface p-4">
      <h2 className="text-sm font-semibold">Bring your own key (BYOK)</h2>
      <p className="mt-1 text-xs text-text-secondary">
        Provider secrets are wrapped by this organisation&apos;s KMS key (v4 envelope). Disabling currently makes provider key operations fail closed.
      </p>
      {config.isSuccess ? (
        <p className="mt-2 text-xs text-text-secondary">
          status: <span className="font-mono">{config.data.status}</span>
        </p>
      ) : null}
      <form className="mt-3 grid gap-2" onSubmit={save}>
        <Field label="KMS URL" htmlFor="kms-url" error={errors.kmsURL?.message}>
          <Input id="kms-url" aria-invalid={Boolean(errors.kmsURL)} {...register("kmsURL")} placeholder="https://kms.example.com" className="h-9 text-xs" />
        </Field>
        <Field label="KMS key id" htmlFor="kms-key" error={errors.keyID?.message}>
          <Input id="kms-key" aria-invalid={Boolean(errors.keyID)} {...register("keyID")} placeholder="org-key" className="h-9 text-xs" />
        </Field>
        <Field label="KMS token" htmlFor="kms-token" error={errors.token?.message}>
          <Input id="kms-token" type="password" aria-invalid={Boolean(errors.token)} {...register("token")} placeholder="optional bearer token" className="h-9 text-xs" />
        </Field>
        <div className="flex flex-wrap items-center gap-2">
          <Button type="submit" variant="secondary">Enable BYOK</Button>
        {config.isSuccess ? <Button variant="secondary" onClick={disable}>Disable</Button> : null}
        {note ? <span className="text-xs text-text-secondary">{note}</span> : null}
        </div>
      </form>
    </section>
  );
}
