"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { SidePanel } from "@/components/ui/side-panel";
import {
  ApiError,
  approveDeployment,
  createEnvironment,
  deployRelease,
  listDeployments,
  listEnvironments,
} from "@/lib/api";
import { useZodForm } from "@/lib/form";
import { environmentSchema, type EnvironmentValues } from "@/lib/schemas";

const names = ["dev", "staging", "prod"];

const statusStyles: Record<string, string> = {
  queued: "bg-status-warning/10 text-status-warning",
  running: "bg-brand-subtle text-brand",
  succeeded: "bg-status-success/10 text-status-success",
  failed: "bg-status-danger/10 text-status-danger",
  rolled_back: "bg-bg-hover text-text-muted",
};

export function EnvironmentsPanel({ orgID, projectID }: { orgID: string; projectID: string }) {
  const queryClient = useQueryClient();
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useZodForm<EnvironmentValues>(environmentSchema, { defaultValues: { name: "dev", url: "" } });
  const [selected, setSelected] = useState<string | null>(null);
  const [release, setRelease] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [showForm, setShowForm] = useState(false);

  const envs = useQuery({
    queryKey: ["environments", orgID, projectID],
    queryFn: () => listEnvironments(orgID, projectID),
  });
  const activeEnv = selected ?? envs.data?.data[0]?.id ?? null;

  const deployments = useQuery({
    queryKey: ["deployments", orgID, activeEnv],
    enabled: Boolean(activeEnv),
    queryFn: () => listDeployments(orgID, activeEnv!),
  });

  const addEnv = useMutation({
    mutationFn: (input: EnvironmentValues) => createEnvironment(orgID, projectID, input.name, input.url),
    onSuccess: () => {
      reset({ name: "dev", url: "" });
      setShowForm(false);
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["environments", orgID, projectID] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not add the environment"),
  });

  const deploy = useMutation({
    mutationFn: () => deployRelease(orgID, activeEnv!, release),
    onSuccess: () => {
      setRelease("");
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["deployments", orgID, activeEnv] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not deploy"),
  });

  const approve = useMutation({
    mutationFn: (id: string) => approveDeployment(orgID, id),
    onSuccess: () => {
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["deployments", orgID, activeEnv] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not approve"),
  });

  return (
    <section className="mt-8">
      <header className="mb-3 flex items-center justify-between">
        <div>
          <h2 className="text-sm font-semibold">Environments</h2>
          <p className="text-xs text-text-secondary">
            Deployments per environment. Prod requires a second admin (four eyes).
          </p>
        </div>
        <Button variant="secondary" onClick={() => setShowForm(true)}>+ Add environment</Button>
      </header>

      {error ? (
        <p role="alert" className="mb-3 rounded-md border border-status-danger/30 bg-status-danger/10 p-2 text-xs text-status-danger">
          {error}
        </p>
      ) : null}

      {showForm ? (
        <SidePanel
          dirty
          title="Add environment"
          description="Environments hold deployments; prod needs a second admin approval."
          onClose={() => setShowForm(false)}
          footer={
            <>
              <Button type="button" variant="secondary" onClick={() => setShowForm(false)}>Cancel</Button>
              <Button type="submit" form="environment-form" loading={addEnv.isPending}>Add environment</Button>
            </>
          }
        >
          <form
            id="environment-form"
            className="space-y-4 p-5"
            onSubmit={handleSubmit((values) => addEnv.mutate(values))}
          >
            <Field label="Name" htmlFor="env-name" error={errors.name?.message}>
              <select
                id="env-name"
                {...register("name")}
                className="h-9 w-full rounded-md border border-border bg-bg-surface px-3 text-sm"
              >
                {names.map((name) => (
                  <option key={name} value={name}>{name}</option>
                ))}
              </select>
            </Field>
            <Field label="URL" htmlFor="env-url" error={errors.url?.message}>
              <Input id="env-url" aria-invalid={Boolean(errors.url)} {...register("url")} placeholder="https://dev.example.com" />
            </Field>
          </form>
        </SidePanel>
      ) : null}

      <div className="grid gap-4 lg:grid-cols-2">
        <div className="rounded-lg border border-border bg-bg-surface">
          <ul className="divide-y divide-border">
            {envs.isSuccess && envs.data.data.length === 0 ? (
              <li className="p-4 text-sm text-text-secondary">No environments yet.</li>
            ) : null}
            {envs.isSuccess
              ? envs.data.data.map((env) => (
                  <li key={env.id}>
                    <button
                      type="button"
                      onClick={() => setSelected(env.id)}
                      className={`flex w-full items-center gap-3 px-4 py-2.5 text-left text-sm hover:bg-bg-hover ${
                        activeEnv === env.id ? "bg-bg-hover" : ""
                      }`}
                    >
                      <span className="rounded bg-bg-hover px-1.5 py-0.5 text-[11px] uppercase text-text-muted">
                        {env.name}
                      </span>
                      <span className="flex-1 truncate text-xs text-text-secondary">{env.url || "no URL"}</span>
                    </button>
                  </li>
                ))
              : null}
          </ul>

        </div>

        <div className="rounded-lg border border-border bg-bg-surface">
          <div className="flex items-start gap-2 border-b border-border p-4">
            <Field label="Release ref" htmlFor="release-ref">
              <Input
                id="release-ref"
                value={release}
                onChange={(e) => setRelease(e.target.value)}
                placeholder="v1.2.3 or sha"
              />
            </Field>
            <Button
              loading={deploy.isPending}
              disabled={!activeEnv || release.trim() === ""}
              onClick={() => deploy.mutate()}
            >
              Deploy
            </Button>
          </div>
          <ul className="divide-y divide-border">
            {deployments.isSuccess && deployments.data.data.length === 0 ? (
              <li className="p-4 text-sm text-text-secondary">No deployments yet.</li>
            ) : null}
            {deployments.isSuccess
              ? deployments.data.data.map((item) => (
                  <li key={item.id} className="flex items-center gap-2 px-4 py-2 text-sm">
                    <span className="font-mono text-xs">{item.release_ref}</span>
                    <span className={`rounded px-1.5 py-0.5 text-[11px] ${statusStyles[item.status] ?? ""}`}>
                      {item.status}
                    </span>
                    {item.status === "queued" ? (
                      <Button
                        variant="secondary"
                        className="ml-auto h-7 px-2 text-xs"
                        loading={approve.isPending && approve.variables === item.id}
                        onClick={() => approve.mutate(item.id)}
                      >
                        Approve
                      </Button>
                    ) : null}
                  </li>
                ))
              : null}
          </ul>
        </div>
      </div>
    </section>
  );
}
