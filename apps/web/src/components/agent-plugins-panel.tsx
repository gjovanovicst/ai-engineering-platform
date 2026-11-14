"use client";

import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { ApiError, createAgentPlugin, listAgentPlugins, listProjects, selectAgentPlugin } from "@/lib/api";

export function AgentPluginsPanel({ orgID }: { orgID: string }) {
  const queryClient = useQueryClient();
  const plugins = useQuery({ queryKey: ["agent-plugins", orgID], queryFn: () => listAgentPlugins(orgID) });
  const projects = useQuery({ queryKey: ["projects", orgID], queryFn: () => listProjects(orgID) });
  const [name, setName] = useState("");
  const [version, setVersion] = useState("1.0.0");
  const [image, setImage] = useState("");
  const [command, setCommand] = useState("");
  const [types, setTypes] = useState<string[]>(["development"]);
  const [projectID, setProjectID] = useState("");
  const [pluginID, setPluginID] = useState("");
  const [note, setNote] = useState<string | null>(null);

  async function register() {
    try {
      await createAgentPlugin(orgID, { name, version, image, command, agent_types: types });
      setNote("Plugin registered");
      queryClient.invalidateQueries({ queryKey: ["agent-plugins", orgID] });
    } catch (err) {
      setNote(err instanceof ApiError ? err.message : "Could not register the plugin");
    }
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
      <div className="mt-3 grid gap-2 sm:grid-cols-2">
        <input aria-label="Plugin name" value={name} onChange={(e) => setName(e.target.value)} placeholder="acme-reviewer" className="h-9 rounded-md border border-border bg-bg-surface px-3 text-xs" />
        <input aria-label="Plugin version" value={version} onChange={(e) => setVersion(e.target.value)} placeholder="1.0.0" className="h-9 rounded-md border border-border bg-bg-surface px-3 text-xs" />
        <input aria-label="Plugin image" value={image} onChange={(e) => setImage(e.target.value)} placeholder="ghcr.io/acme/agent@sha256:..." className="h-9 rounded-md border border-border bg-bg-surface px-3 text-xs sm:col-span-2" />
        <input aria-label="Plugin command" value={command} onChange={(e) => setCommand(e.target.value)} placeholder="/agent --prompt /workspace/.aip/prompt.txt" className="h-9 rounded-md border border-border bg-bg-surface px-3 text-xs sm:col-span-2" />
      </div>
      <div className="mt-2 flex flex-wrap gap-3 text-xs">
        {["planning", "development", "review"].map((type) => (
          <label key={type} className="flex items-center gap-1">
            <input type="checkbox" checked={types.includes(type)} onChange={(e) => setTypes(e.target.checked ? [...types, type] : types.filter((t) => t !== type))} />
            {type}
          </label>
        ))}
      </div>
      <div className="mt-3 flex items-center gap-2">
        <Button variant="secondary" onClick={register}>Register</Button>
        {note ? <span className="text-xs text-text-secondary">{note}</span> : null}
      </div>

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
