"use client";

import { useEffect } from "react";

import { Button } from "@/components/ui/button";

export default function AppError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    // The API/trace has the details; the UI only needs the digest.
    console.error("app error", error.digest ?? error.message);
  }, [error]);

  return (
    <div className="rounded-lg border border-status-danger/30 bg-status-danger/10 p-6">
      <h2 className="text-sm font-semibold text-status-danger">Something broke on this page</h2>
      <p className="mt-1 text-sm text-text-secondary">
        {error.message || "Unexpected error."}
        {error.digest ? <span className="ml-1 font-mono text-xs">({error.digest})</span> : null}
      </p>
      <Button className="mt-4" onClick={reset}>
        Try again
      </Button>
    </div>
  );
}
