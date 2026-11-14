"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { setSession } from "@/lib/session";
import type { TokenPair } from "@/lib/api";

export default function OIDCCallbackPage() {
  const router = useRouter();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const params = new URLSearchParams(window.location.hash.replace(/^#/, ""));
    const access = params.get("access");
    const refresh = params.get("refresh");
    if (!access || !refresh) {
      setError("The sign-in response is incomplete.");
      return;
    }
    setSession({ access_token: access, refresh_token: refresh } as TokenPair);
    router.replace("/projects");
  }, [router]);

  return (
    <main className="mx-auto flex min-h-screen max-w-md flex-col justify-center p-6">
      <h1 className="text-xl font-semibold">Signing you in…</h1>
      {error ? (
        <p role="alert" className="mt-3 text-sm text-status-danger">
          {error}
        </p>
      ) : (
        <p className="mt-3 text-sm text-text-secondary">One moment.</p>
      )}
    </main>
  );
}
