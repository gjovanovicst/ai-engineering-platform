"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { ApiError, createAgentPlugin, listAgentPlugins, listProjects, selectAgentPlugin } from "@/lib/api";
import { useZodForm } from "@/lib/form";
import { agentPluginSchema, type AgentPluginValues } from "@/lib/schemas";

export function AgentPluginsPanel({ orgID }: { orgID: string }) {
  const queryClient = useQueryClient();
  const plugins = useQuery({ queryKey: ["agent-plugins", orgID], queryFn: () => listAgentPlugins(orgID) });
  const projects = useQuery({ queryKey: ["projects", orgID], queryFn: () => listProjects(orgID) });
  const {
    register,
    handleSubmit,
    watch,
    setValue,
    formState: { errors },
  } = useZodForm<AgentPluginValues>(agentPluginSchema, {
    defaultValues: { name: "", version: "1.0.0", image: "", command: "", types: ["development"] },
  });
  const types = watch("types");
  const [projectID, setProjectID] = useState("");
  const [pluginID, setPluginID] = useState("");
  const [note, setNote] = useState<string | null>(null);

  const registerPlugin = handleSubmit(async (values) => {
    try {
      await createAgentPlugin(orgID, {
        name: values.name,
        version: values.version,
        image: values.image,
        command: values.command,
        agent_types: values.types,
      });
      setNote("Plugin registered");
      queryClient.invalidateQueries({ queryKey: ["agent-plugins", orgID] });
    } catch (err) {
      setNote(err instanceof ApiError ? err.message : "Could not register the plugin");
    }
  });

  function toggleType(type: string, checked: boolean) {
    setValue("types", checked ? [...types, type] : types.filter((t) => t !== type));
  }

  async function select() {
    try {
      await selectAgentPlugin(orgID, projectID, pluginID);
      setNote("Plugin selected for the project");
    } catch (err) {
      setNote(err instanceof ApiError ? err.message : "Could not select the plugin");
    }
  }

  const items = plugins.data?.items ?? [];
  const projectItems = projects.data?.data ?? [];

  return (
    <section className="rounded-lg border border-border bg-bg-surface p-4">
      <h2 className="text-sm font-semibold">Custom agent plugins</h2>
      <p className="mt-1 text-xs text-text-secondary">Images must be digest-pinned. The worker runs the command only for the selected agent types.</p>
      <form className="mt-3 grid gap-2 sm:grid-cols-2" onSubmit={registerPlugin}>
        <Field label="Plugin name" htmlFor="plugin-name" error={errors.name?.message}>
          <Input id="plugin-name" aria-invalid={Boolean(errors.name)} {...register("name")} placeholder="acme-reviewer" className="h-9 text-xs" />
        </Field>
        <Field label="Plugin version" htmlFor="plugin-version" error={errors.version?.message}>
          <Input id="plugin-version" aria-invalid={Boolean(errors.version)} {...register("version")} placeholder="1.0.0" className="h-9 text-xs" />
        </Field>
        <Field label="Plugin image" htmlFor="plugin-image" error={errors.image?.message}>
          <Input id="plugin-image" aria-invalid={Boolean(errors.image)} {...register("image")} placeholder="ghcr.io/acme/agent@sha256:..." className="h-9 text-xs sm:col-span-2" />
        </Field>
        <Field label="Plugin command" htmlFor="plugin-command" error={errors.command?.message}>
          <Input id="plugin-command" aria-invalid={Boolean(errors.command)} {...register("command")} placeholder="/agent --prompt /workspace/.aip/prompt.txt" className="h-9 text-xs sm:col-span-2" />
        </Field>
        <div className="sm:col-span-2">
          <div className="flex flex-wrap gap-3 text-xs">
            {["planning", "development", "review"].map((type) => (
              <label key={type} className="flex items-center gap-1">
                <input type="checkbox" checked={types.includes(type)} onChange={(e) => toggleType(type, e.target.checked)} />
                {type}
              </label>
            ))}
          </div>
          {errors.types?.message ? <p role="alert" className="mt-1 text-xs text-status-danger">{errors.types.message}</p> : null}
        </div>
        <div className="flex items-center gap-2 sm:col-span-2">
          <Button type="submit" variant="secondary">Register</Button>
        {note ? <span className="text-xs text-text-secondary">{note}</span> : null}
      </div>
      </form>

      <ul className="mt-4 grid gap-1 text-xs">
        {items.map((plugin) => (
          <li key={plugin.id} className="rounded border border-border p-2">
            <span className="font-medium">{plugin.name}@{plugin.version}</span>{" "}
            <span className="text-text-secondary">{plugin.agent_types.join(", ")}</span>
          </li>
        ))}
        {!items.length ? <li className="text-text-muted">No custom plugins yet.</li> : null}
      </ul>

      <div className="mt-4 grid gap-2 sm:grid-cols-2">
        <select aria-label="Project" value={projectID} onChange={(e) => setProjectID(e.target.value)} className="h-9 rounded-md border border-border bg-bg-surface px-2 text-xs">
          <option value="">Select project</option>
          {projectItems.map((project) => <option key={project.id} value={project.id}>{project.name}</option>)}
        </select>
        <select aria-label="Plugin" value={pluginID} onChange={(e) => setPluginID(e.target.value)} className="h-9 rounded-md border border-border bg-bg-surface px-2 text-xs">
          <option value="">Select plugin</option>
          {items.map((plugin) => <option key={plugin.id} value={plugin.id}>{plugin.name}@{plugin.version}</option>)}
        </select>
      </div>
      <div className="mt-2 flex items-center gap-2">
        <Button variant="secondary" onClick={select} disabled={!projectID}>Use for project</Button>
        <Button variant="secondary" onClick={async () => { await selectAgentPlugin(orgID, projectID, ""); setNote("Plugin cleared"); }} disabled={!projectID}>Clear</Button>
      </div>
    </section>
  );
}
