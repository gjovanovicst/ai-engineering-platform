"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { ensureSession, listOrgRepos, repoURL } from "@/lib/api";
import type { Claims } from "@/lib/auth";

export default function RepositoriesPage() {
  const router = useRouter();
  const [claims, setClaims] = useState<Claims | null | undefined>(undefined);

  useEffect(() => {
    ensureSession().then((result) => {
      if (!result) {
        router.replace("/login");
        return;
      }
      setClaims(result);
    });
  }, [router]);

  const repos = useQuery({
    queryKey: ["org-repos", claims?.org],
    enabled: Boolean(claims?.org),
    queryFn: () => listOrgRepos(claims!.org),
  });

  return (
    <div className="page-stack">
      <header className="page-hero mb-6">
        <h1 className="text-[22px] font-semibold">Repositories</h1>
        <p className="mt-1 text-[13px] text-text-secondary">
          Source repositories connected to projects, for GitHub and GitLab.
        </p>
      </header>

      {repos.isPending ? (
        <div className="space-y-2">{[0, 1, 2].map((i) => <div key={i} className="h-12 animate-pulse rounded-lg bg-bg-hover" />)}</div>
      ) : null}

      {repos.isError ? (
        <p role="alert" className="rounded-lg border border-status-danger/30 bg-status-danger/10 p-3 text-sm text-status-danger">
          Could not load repositories.
        </p>
      ) : null}

      {repos.isSuccess && repos.data.data.length === 0 ? (
        <p className="rounded-lg border border-dashed border-border bg-bg-surface p-8 text-center text-sm text-text-secondary">
          No repositories yet. Connect one from a project page.
        </p>
      ) : null}

      {repos.isSuccess && repos.data.data.length > 0 ? (
        <ul className="divide-y divide-border overflow-hidden rounded-lg border border-border bg-bg-surface">
          {repos.data.data.map((repo) => (
            <li key={repo.id} className="repo-list-row flex flex-wrap items-center gap-3 px-4 py-3 text-sm">
              <span className="rounded bg-bg-hover px-1.5 py-0.5 text-[11px] uppercase text-text-muted">
                {repo.provider}
              </span>
              <a href={repoURL(repo)} target="_blank" rel="noreferrer" className="font-mono text-xs text-brand hover:underline">
                {repo.name}
              </a>
              <span className="text-xs text-text-muted">base: {repo.default_branch}</span>
              <Link href={`/projects/${repo.project_id}`} className="ml-auto text-xs text-text-secondary hover:text-brand">
                {repo.project_name} →
              </Link>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
