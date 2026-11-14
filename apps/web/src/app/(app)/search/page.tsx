"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { ApiError, ensureSession, searchTasks } from "@/lib/api";
import type { Claims } from "@/lib/auth";

export default function SearchPage() {
  const router = useRouter();
  const [claims, setClaims] = useState<Claims | null | undefined>(undefined);
  const [term, setTerm] = useState("");
  const [query, setQuery] = useState("");

  useEffect(() => {
    ensureSession().then((result) => {
      if (!result) {
        router.replace("/login");
        return;
      }
      setClaims(result);
    });
  }, [router]);

  const results = useQuery({
    queryKey: ["search", claims?.org, query],
    enabled: Boolean(claims?.org) && query.trim().length >= 2,
    queryFn: () => searchTasks(claims!.org, query.trim()),
  });

  return (
    <div className="page-stack">
      <header className="page-hero mb-6">
        <h1 className="text-[22px] font-semibold">Search</h1>
        <p className="mt-1 text-[13px] text-text-secondary">
          Find tasks by reference, title or description.
        </p>
      </header>

      <form
        className="search-box mb-6 flex gap-2"
        onSubmit={(event) => {
          event.preventDefault();
          setQuery(term);
        }}
      >
        <input
          value={term}
          onChange={(e) => setTerm(e.target.value)}
          placeholder="T-142 or SSO"
          aria-label="Search tasks"
          className="h-9 flex-1 rounded-md border border-border bg-bg-surface px-3 text-sm focus:outline-none focus:ring-2 focus:ring-brand"
        />
        <Button type="submit" disabled={term.trim().length < 2}>
          Search
        </Button>
      </form>

      {results.isPending && query ? (
        <p className="text-sm text-text-muted">Searching…</p>
      ) : null}

      {results.isError ? (
        <p role="alert" className="rounded-lg border border-status-danger/30 bg-status-danger/10 p-3 text-sm text-status-danger">
          {results.error instanceof ApiError ? results.error.message : "Search failed"}
        </p>
      ) : null}

      {results.isSuccess && results.data.data.length === 0 ? (
        <p className="rounded-lg border border-dashed border-border bg-bg-surface p-6 text-center text-sm text-text-secondary">
          Nothing matched “{query}”.
        </p>
      ) : null}

      {results.isSuccess && results.data.data.length > 0 ? (
        <p className="mb-3 text-xs text-text-muted">{results.data.data.length} result{results.data.data.length === 1 ? "" : "s"} for “{query}”</p>
      ) : null}

      {results.isSuccess && results.data.data.length > 0 ? (
        <ul className="divide-y divide-border overflow-hidden rounded-lg border border-border bg-bg-surface">
          {results.data.data.map((hit) => (
            <li key={hit.id}>
              <Link
                href={hit.url}
                className="flex items-center gap-3 px-4 py-3 text-sm hover:bg-bg-hover"
              >
                <span className="rounded bg-bg-hover px-1.5 py-0.5 text-[11px] uppercase text-text-muted">
                  {hit.type}
                </span>
                <span className="flex-1">
                  {hit.title}
                  {hit.subtitle ? (
                    <span className="ml-2 font-mono text-xs text-text-muted">{hit.subtitle}</span>
                  ) : null}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
