"use client";

import { useEffect, useMemo, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";

import { listOrgRuns, listProjects, searchTasks } from "@/lib/api";

type Item = { id: string; label: string; hint: string; group: string; run: () => void };

const staticActions: { id: string; label: string; hint: string; path: string }[] = [
  { id: "projects", label: "Go to projects", hint: "Every project in this workspace", path: "/projects" },
  { id: "agents", label: "Go to agents", hint: "Runs and their status", path: "/agents" },
  { id: "search", label: "Search tasks", hint: "The full search page", path: "/search" },
  { id: "settings", label: "Go to settings", hint: "Workspace, providers, access", path: "/settings" },
  { id: "help", label: "Open the help", hint: "Docs, shortcuts and glossary", path: "/overview?help=1" },
];

/** The command palette: actions, projects, tasks and runs, keyboard first. */
export function CommandPalette({ orgID, onClose }: { orgID: string; onClose: () => void }) {
  const router = useRouter();
  const [query, setQuery] = useState("");
  const [active, setActive] = useState(0);
  const inputRef = useRef<HTMLInputElement | null>(null);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  const projects = useQuery({ queryKey: ["projects", orgID], queryFn: () => listProjects(orgID) });
  const tasks = useQuery({
    queryKey: ["palette-tasks", orgID, query],
    enabled: query.trim().length >= 2,
    queryFn: () => searchTasks(orgID, query.trim()),
  });
  const runs = useQuery({ queryKey: ["palette-runs", orgID], queryFn: () => listOrgRuns(orgID) });

  const items = useMemo<Item[]>(() => {
    const go = (path: string) => () => {
      onClose();
      router.push(path);
    };
    const actions = staticActions.map((action) => ({
      id: `action-${action.id}`,
      label: action.label,
      hint: action.hint,
      group: "Actions",
      run: go(action.path),
    }));
    const projectItems = (projects.data?.data ?? []).map((project) => ({
      id: `project-${project.id}`,
      label: project.name,
      hint: "Project",
      group: "Projects",
      run: go(`/projects/${project.id}`),
    }));
    const hitItems = (tasks.data?.data ?? []).map((hit) => ({
      id: `hit-${hit.type}-${hit.id}`,
      label: hit.title,
      hint: hit.subtitle ?? hit.type,
      group: hit.type === "task" ? "Tasks" : hit.type === "run" ? "Runs" : hit.type === "project" ? "Projects" : "Results",
      run: go(hit.url),
    }));
    const runItems = (runs.data?.data ?? []).slice(0, 8).map((run) => ({
      id: `run-${run.id}`,
      label: `Run ${run.id.slice(0, 8)} · ${run.agent_type}`,
      hint: run.status.replaceAll("_", " "),
      group: "Runs",
      run: go(`/projects/${run.project_id}/runs/${run.id}`),
    }));
    return [...actions, ...projectItems, ...hitItems, ...runItems];
  }, [projects.data, tasks.data, runs.data, onClose, router]);

  const filtered = useMemo(() => {
    const term = query.trim().toLowerCase();
    if (!term) return items.slice(0, 12);
    return items.filter((item) => `${item.label} ${item.hint}`.toLowerCase().includes(term)).slice(0, 12);
  }, [items, query]);

  useEffect(() => {
    setActive(0);
  }, [query]);

  const current = filtered[active];

  function onKeyDown(event: React.KeyboardEvent) {
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActive((index) => (filtered.length === 0 ? 0 : (index + 1) % filtered.length));
    } else if (event.key === "ArrowUp") {
      event.preventDefault();
      setActive((index) => (filtered.length === 0 ? 0 : (index - 1 + filtered.length) % filtered.length));
    } else if (event.key === "Enter") {
      event.preventDefault();
      current?.run();
    } else if (event.key === "Escape") {
      event.preventDefault();
      onClose();
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-start justify-center bg-black/40 p-4 pt-[12vh]" role="presentation" onMouseDown={onClose}>
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Command palette"
        className="w-full max-w-xl overflow-hidden rounded-xl border border-border bg-bg-surface shadow-2xl"
        onMouseDown={(event) => event.stopPropagation()}
      >
        <input
          ref={inputRef}
          role="combobox"
          aria-expanded="true"
          aria-controls="command-palette-list"
          aria-activedescendant={current ? `command-palette-${current.id}` : undefined}
          aria-label="Search actions, projects, tasks and runs"
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          onKeyDown={onKeyDown}
          placeholder="Type an action, a project, a task ref or a run…"
          className="w-full border-b border-border bg-transparent px-4 py-3 text-sm focus:outline-none"
        />
        <ul id="command-palette-list" role="listbox" aria-label="Results" className="max-h-80 overflow-y-auto py-1">
          {filtered.length === 0 ? (
            <li className="px-4 py-6 text-center text-xs text-text-muted">Nothing matches. Try a task ref or a page name.</li>
          ) : (
            filtered.map((item, index) => (
              <li
                key={item.id}
                id={`command-palette-${item.id}`}
                role="option"
                aria-selected={index === active}
                onMouseEnter={() => setActive(index)}
                onMouseDown={(event) => {
                  event.preventDefault();
                  item.run();
                }}
                className={`flex cursor-pointer items-center justify-between gap-3 px-4 py-2 text-sm ${index === active ? "bg-bg-hover" : ""}`}
              >
                <span className="min-w-0">
                  <span className="block truncate">{item.label}</span>
                  <span className="block text-[11px] text-text-muted">{item.group}</span>
                </span>
                <span className="shrink-0 text-[11px] text-text-muted">{item.hint}</span>
              </li>
            ))
          )}
        </ul>
        <div className="flex items-center gap-3 border-t border-border px-4 py-2 text-[11px] text-text-muted">
          <span><kbd className="rounded border border-border px-1">↑</kbd> <kbd className="rounded border border-border px-1">↓</kbd> navigate</span>
          <span><kbd className="rounded border border-border px-1">Enter</kbd> run</span>
          <span><kbd className="rounded border border-border px-1">Esc</kbd> close</span>
        </div>
      </div>
    </div>
  );
}
