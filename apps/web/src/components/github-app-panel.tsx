"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { ApiError, createGitHubAppManifest, getGitHubApp, setGitHubInstallation } from "@/lib/api";
import { useZodForm } from "@/lib/form";
import { githubAppSchema, type GitHubAppValues } from "@/lib/schemas";

export function GitHubAppPanel({ orgID }: { orgID: string }) {
  const queryClient = useQueryClient();
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useZodForm<GitHubAppValues>(githubAppSchema, { defaultValues: { installation: "" } });
  const [error, setError] = useState<string | null>(null);
  const status = useQuery({ queryKey: ["github-app", orgID], queryFn: () => getGitHubApp(orgID) });

  const manifest = useMutation({
    mutationFn: () => createGitHubAppManifest(orgID),
    onSuccess: (result) => {
      const form = document.createElement("form");
      form.method = "POST";
      form.action = `${result.action_url}?state=${encodeURIComponent(result.state)}`;
      const input = document.createElement("input");
      input.type = "hidden";
      input.name = "manifest";
      input.value = JSON.stringify(result.manifest);
      form.appendChild(input);
      document.body.appendChild(form);
      form.submit();
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not start the GitHub App setup"),
  });

  const install = useMutation({
    mutationFn: (values: GitHubAppValues) => setGitHubInstallation(orgID, Number(values.installation)),
    onSuccess: () => {
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["github-app", orgID] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not save the installation"),
  });

  const app = status.data?.github_app;

  return (
    <section className="rounded-lg border border-border bg-bg-surface p-4">
      <h2 className="text-sm font-semibold">GitHub App</h2>
      <p className="mt-1 text-xs text-text-secondary">
        Create the app from the platform; GitHub returns its id, key and webhook secret. Then install it and paste the installation id.
      </p>
      {status.isPending ? <p className="mt-3 text-xs text-text-muted">Loading…</p> : null}
      {status.isSuccess && !status.data.configured ? (
        <Button className="mt-3" loading={manifest.isPending} onClick={() => manifest.mutate()}>
          Create GitHub App
        </Button>
      ) : null}
      {app ? (
        <div className="mt-3 space-y-3 text-xs">
          <p>
            <span className="text-text-muted">App:</span> {app.slug} (id {app.app_id}) ·{" "}
            <span className={app.installed ? "text-status-success" : "text-status-warning"}>
              {app.installed ? `installed (${app.installation_id})` : "not installed"}
            </span>
          </p>
          {app.install_url ? (
            <a href={app.install_url} target="_blank" rel="noreferrer" className="text-brand hover:underline">
              Install on repositories →
            </a>
          ) : null}
          <form className="flex flex-wrap items-start gap-2" onSubmit={handleSubmit((values) => install.mutate(values))}>
            <Field label="Installation id" htmlFor="gh-installation" error={errors.installation?.message}>
              <Input
                id="gh-installation"
                aria-invalid={Boolean(errors.installation)}
                {...register("installation")}
                placeholder="installation id"
                className="max-w-[220px]"
              />
            </Field>
            <Button type="submit" variant="secondary" loading={install.isPending} className="mt-5">
              Save installation
            </Button>
          </form>
        </div>
      ) : null}
      {error ? <p role="alert" className="mt-2 text-xs text-status-danger">{error}</p> : null}
    </section>
  );
}
