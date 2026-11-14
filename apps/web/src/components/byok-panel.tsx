"use client";

import { useEffect, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { ApiError, disableOrgCrypto, getOrgCrypto, setOrgCrypto } from "@/lib/api";

export function BYOKPanel({ orgID }: { orgID: string }) {
  const queryClient = useQueryClient();
  const config = useQuery({ queryKey: ["org-crypto", orgID], queryFn: () => getOrgCrypto(orgID), retry: false });
  const [kmsURL, setKmsURL] = useState("");
  const [keyID, setKeyID] = useState("");
  const [token, setToken] = useState("");
  const [note, setNote] = useState<string | null>(null);

  useEffect(() => {
    if (config.data) {
      setKmsURL(config.data.kms_url);
      setKeyID(config.data.key_id);
    }
  }, [config.data]);

  async function save() {
    try {
      await setOrgCrypto(orgID, kmsURL, keyID, token);
      setNote("BYOK enabled");
      queryClient.invalidateQueries({ queryKey: ["org-crypto", orgID] });
    } catch (err) {
      setNote(err instanceof ApiError ? err.message : "Could not save BYOK");
    }
  }

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
      <div className="mt-3 grid gap-2">
        <input aria-label="KMS URL" value={kmsURL} onChange={(e) => setKmsURL(e.target.value)} placeholder="https://kms.example.com" className="h-9 rounded-md border border-border bg-bg-surface px-3 text-xs" />
        <input aria-label="KMS key id" value={keyID} onChange={(e) => setKeyID(e.target.value)} placeholder="org-key" className="h-9 rounded-md border border-border bg-bg-surface px-3 text-xs" />
        <input aria-label="KMS token" type="password" value={token} onChange={(e) => setToken(e.target.value)} placeholder="optional bearer token" className="h-9 rounded-md border border-border bg-bg-surface px-3 text-xs" />
      </div>
      <div className="mt-3 flex flex-wrap items-center gap-2">
        <Button variant="secondary" onClick={save}>Enable BYOK</Button>
        {config.isSuccess ? <Button variant="secondary" onClick={disable}>Disable</Button> : null}
        {note ? <span className="text-xs text-text-secondary">{note}</span> : null}
      </div>
    </section>
  );
}
