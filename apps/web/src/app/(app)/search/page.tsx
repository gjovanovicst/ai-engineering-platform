"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { ApiError, ensureSession, searchTasks } from "@/lib/api";
import { useZodForm } from "@/lib/form";
import { searchSchema, type SearchValues } from "@/lib/schemas";
import type { Claims } from "@/lib/auth";

export default function SearchPage() {
  const router = useRouter();
  const [claims, setClaims] = useState<Claims | null | undefined>(undefined);
  const [query, setQuery] = useState("");
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useZodForm<SearchValues>(searchSchema, { defaultValues: { q: "" } });

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

      <form className="search-box mb-6 flex flex-wrap items-start gap-2" onSubmit={handleSubmit((values) => setQuery(values.q))}>
        <Field label="Search tasks" htmlFor="search-term" error={errors.q?.message}>
          <Input
            id="search-term"
            aria-invalid={Boolean(errors.q)}
            {...register("q")}
            placeholder="T-142 or SSO"
            className="h-9 w-[340px] max-w-full"
          />
        </Field>
        <Button type="submit" className="mt-0.5">
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
