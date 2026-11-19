"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { SidePanel } from "@/components/ui/side-panel";
import { RepoCredentialControl } from "@/components/repo-credential-panel";
import {
  ApiError,
  connectRepo,
  listRepos,
  repoURL,
  webhookURL,
} from "@/lib/api";
import { useZodForm } from "@/lib/form";
import { repoConnectSchema, type RepoConnectValues } from "@/lib/schemas";

export function RepoPanel({
  orgID,
  projectID,
  canManage = false,
}: {
  orgID: string;
  projectID: string;
  canManage?: boolean;
}) {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useZodForm<RepoConnectValues>(repoConnectSchema, {
    defaultValues: { provider: "github", name: "", default_branch: "main", webhook_secret: "" },
  });

  const repos = useQuery({
    queryKey: ["repos", orgID, projectID],
    queryFn: () => listRepos(orgID, projectID),
  });

  const connect = useMutation({
    mutationFn: (input: RepoConnectValues) => connectRepo(orgID, projectID, input),
    onSuccess: () => {
      setOpen(false);
      setError(null);
      reset({ provider: "github", name: "", default_branch: "main", webhook_secret: "" });
      queryClient.invalidateQueries({ queryKey: ["repos", orgID, projectID] });
    },
    onError: (err) =>
      setError(err instanceof ApiError ? err.message : "Could not connect the repository"),
  });

  return (
    <section className="mt-8">
      <header className="mb-3 flex items-center justify-between">
        <div>
          <h2 className="text-sm font-semibold">Repositories</h2>
          <p className="text-xs text-text-secondary">
            Push branches and open pull requests / merge requests.
          </p>
        </div>
        <Button variant="secondary" onClick={() => setOpen(true)}>
          + Connect repository
        </Button>
      </header>

      {open ? (
        <SidePanel
          title="Connect repository"
          description="The agent pushes branches and opens pull requests here."
          onClose={() => setOpen(false)}
          footer={
            <>
              <Button type="button" variant="secondary" onClick={() => setOpen(false)}>Cancel</Button>
              <Button type="submit" form="repo-form" loading={connect.isPending}>Connect</Button>
            </>
          }
        >
        <form
          id="repo-form"
          className="space-y-4 p-5"
          onSubmit={handleSubmit((values) => connect.mutate(values))}
        >
          <div className="grid gap-3">
            <Field label="Provider" htmlFor="provider">
              <select
                id="provider"
                {...register("provider")}
                className="h-9 w-full rounded-md border border-border bg-bg-surface px-3 text-sm focus:outline-none focus:ring-2 focus:ring-brand"
              >
                <option value="github">GitHub</option>
                <option value="gitlab">GitLab</option>
              </select>
            </Field>
            <Field label="Repository" htmlFor="repo-name" error={errors.name?.message}>
              <Input id="repo-name" aria-invalid={Boolean(errors.name)} {...register("name")} placeholder="owner/name or group/project" />
            </Field>
            <Field label="Default branch" htmlFor="default-branch" error={errors.default_branch?.message}>
              <Input id="default-branch" aria-invalid={Boolean(errors.default_branch)} {...register("default_branch")} placeholder="main" />
            </Field>
            <Field label="Webhook secret" htmlFor="webhook-secret" error={errors.webhook_secret?.message}>
              <Input id="webhook-secret" type="password" aria-invalid={Boolean(errors.webhook_secret)} {...register("webhook_secret")} placeholder="The webhook signing secret" />
            </Field>
          </div>
          {error ? (
            <p role="alert" className="text-xs text-status-danger">
              {error}
            </p>
          ) : null}
        </form>
        </SidePanel>
      ) : null}

      {repos.isPending ? <div className="h-16 animate-pulse rounded-lg bg-bg-hover" /> : null}

      {repos.isError ? (
        <div role="alert" className="rounded-lg border border-status-danger/30 bg-status-danger/10 p-3 text-sm text-status-danger">
          Could not load repositories.
        </div>
      ) : null}

      {repos.isSuccess && repos.data.data.length === 0 && !open ? (
        <p className="rounded-lg border border-dashed border-border bg-bg-surface p-6 text-center text-sm text-text-secondary">
          No repository connected yet.
        </p>
      ) : null}

      {repos.isSuccess && repos.data.data.length > 0 ? (
        <ul className="divide-y divide-border overflow-hidden rounded-lg border border-border bg-bg-surface">
          {repos.data.data.map((repo) => (
            <li key={repo.id} className="flex flex-wrap items-center gap-3 px-4 py-3 text-sm">
              <span className="rounded bg-bg-hover px-1.5 py-0.5 text-[11px] uppercase text-text-muted">
                {repo.provider}
              </span>
              <a
                href={repoURL(repo)}
                target="_blank"
                rel="noreferrer"
                className="font-mono text-xs text-brand hover:underline"
              >
                {repo.name}
              </a>
              <span className="text-xs text-text-muted">base: {repo.default_branch}</span>
              {canManage ? (
                <RepoCredentialControl orgID={orgID} projectID={projectID} repo={repo} />
              ) : null}
              <code className="ml-auto truncate rounded bg-bg-hover px-2 py-1 text-[11px] text-text-secondary" title={webhookURL(repo)}>
                {webhookURL(repo)}
              </code>
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  );
}
