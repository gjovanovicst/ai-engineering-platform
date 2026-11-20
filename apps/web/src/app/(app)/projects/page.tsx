"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { SidePanel } from "@/components/ui/side-panel";
import { ApiError, createProject, ensureSession, listProjects } from "@/lib/api";
import { useAssistantForm } from "@/lib/assistant-form";
import { useZodForm } from "@/lib/form";
import { projectSchema, type ProjectValues } from "@/lib/schemas";
import type { Claims } from "@/lib/auth";

export default function ProjectsPage() {
  const router = useRouter();
  const [claims, setClaims] = useState<Claims | null | undefined>(undefined);
  const queryClient = useQueryClient();
  const [showForm, setShowForm] = useState(false);
  const [search, setSearch] = useState("");
  const [statusFilter, setStatusFilter] = useState("all");
  const [formError, setFormError] = useState<string | null>(null);
  const {
    register,
    handleSubmit,
    reset,
    setValue,
    formState: { errors },
  } = useZodForm<ProjectValues>(projectSchema, { defaultValues: { name: "", description: "" } });

  useEffect(() => {
    ensureSession().then((result) => {
      if (!result) {
        router.replace("/login");
        return;
      }
      setClaims(result);
    });
  }, [router]);

  const create = useMutation({
    mutationFn: (input: ProjectValues) => createProject(claims!.org, input),
    onSuccess: (project) => {
      setShowForm(false);
      reset({ name: "", description: "" });
      setFormError(null);
      queryClient.invalidateQueries({ queryKey: ["projects", claims?.org] });
      router.push(`/projects/${project.id}`);
    },
    onError: (err) => setFormError(err instanceof ApiError ? err.message : "Could not create the project"),
  });

  const query = useQuery({
    queryKey: ["projects", claims?.org],
    enabled: Boolean(claims?.org),
    queryFn: () => listProjects(claims!.org),
  });

  const allProjects = query.data?.data ?? [];
  const visibleProjects = allProjects.filter((project) => {
    const matchesSearch = `${project.name} ${project.slug} ${project.description ?? ""}`.toLowerCase().includes(search.toLowerCase());
    const matchesStatus = statusFilter === "all" || project.status.toLowerCase() === statusFilter;
    return matchesSearch && matchesStatus;
  });
  const activeCount = allProjects.filter((project) => ["active", "ready", "in_progress"].includes(project.status.toLowerCase())).length;

  useAssistantForm({
    id: "project-form",
    title: "New project",
    fields: [
      { name: "name", label: "Name", type: "text" },
      { name: "description", label: "Description", type: "textarea" },
    ],
    apply: (values) => {
      if (typeof values.name === "string") setValue("name", values.name as never, { shouldDirty: true });
      if (typeof values.description === "string") setValue("description", values.description as never, { shouldDirty: true });
    },
  });

  return (
    <div className="page-stack">
      <header className="page-hero mb-6 flex flex-wrap items-end justify-between gap-4">
        <div>
          <p className="text-xs text-text-muted">{claims?.org ? "Workspace" : ""}</p>
          <h1 className="text-[22px] font-semibold">Projects</h1>
          <p className="mt-1 text-[13px] text-text-secondary">A shared view of delivery, code and AI execution across your workspace.</p>
        </div>
        <Button onClick={() => setShowForm(true)}>+ New project</Button>
      </header>

      <section className="grid gap-3 sm:grid-cols-3 mb-5" aria-label="Project summary">
        {[{label:"All projects",value:allProjects.length,note:"Across this workspace"},{label:"Active projects",value:activeCount,note:"Ready or in delivery"},{label:"Currently visible",value:visibleProjects.length,note:"Matching your filters"}].map((item) => <div key={item.label} className="metric-panel"><span>{item.label}</span><strong>{item.value}</strong><small>{item.note}</small></div>)}
      </section>

      {showForm ? (
        <SidePanel
          title="New project"
          description="A project groups tasks, repositories and environments for one product."
          onClose={() => setShowForm(false)}
          footer={
            <>
              <Button type="button" variant="secondary" onClick={() => setShowForm(false)}>Cancel</Button>
              <Button type="submit" form="project-form" loading={create.isPending}>
                Create project
              </Button>
            </>
          }
        >
        <form
          id="project-form"
          className="space-y-4 p-5"
          onSubmit={handleSubmit((values) => create.mutate(values))}
        >
          <div className="grid gap-3">
            <Field label="Name" htmlFor="project-name" error={errors.name?.message}>
              <Input
                id="project-name"
                autoFocus
                aria-invalid={Boolean(errors.name)}
                {...register("name")}
                placeholder="e.g. Customer portal"
              />
            </Field>
            <Field label="Description" htmlFor="project-description" error={errors.description?.message}>
              <Input
                id="project-description"
                aria-invalid={Boolean(errors.description)}
                {...register("description")}
                placeholder="What the project delivers and for whom"
              />
            </Field>
          </div>
          {formError ? (
            <p role="alert" className="text-xs text-status-danger">
              {formError}
            </p>
          ) : null}
        </form>
        </SidePanel>
      ) : null}

      {query.isSuccess && allProjects.length > 0 ? (
        <div className="mb-4 flex flex-wrap gap-2">
          <input aria-label="Search projects" value={search} onChange={(e) => setSearch(e.target.value)} placeholder="Search projects…" className="h-10 min-w-[220px] flex-1 rounded-xl border border-border bg-bg-surface px-3 text-sm" />
          <select aria-label="Filter by status" value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)} className="h-10 rounded-xl border border-border bg-bg-surface px-3 text-sm"><option value="all">All statuses</option><option value="active">Active</option><option value="ready">Ready</option><option value="in_progress">In progress</option><option value="archived">Archived</option></select>
        </div>
      ) : null}

      {query.isPending ? (
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3" aria-label="Loading projects">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-32 animate-pulse rounded-lg bg-bg-hover" />
          ))}
        </div>
      ) : null}

      {query.isError ? (
        <div
          role="alert"
          className="rounded-lg border border-status-danger/30 bg-status-danger/10 p-4 text-sm"
        >
          <p className="font-medium text-status-danger">
            {query.error instanceof ApiError ? query.error.message : "Failed to load projects"}
          </p>
          <Button variant="secondary" className="mt-3" onClick={() => query.refetch()}>
            Retry
          </Button>
        </div>
      ) : null}

      {query.isSuccess && allProjects.length === 0 ? (
        <div className="rounded-lg border border-dashed border-border bg-bg-surface p-12 text-center">
          <p className="font-medium">No projects yet</p>
          <p className="mt-1 text-sm text-text-secondary">
            Create your first project from a blueprint to get started.
          </p>
          <Button className="mt-4" onClick={() => setShowForm(true)}>
            + New project
          </Button>
        </div>
      ) : null}

      {query.isSuccess && allProjects.length > 0 && visibleProjects.length === 0 ? (
        <div className="empty-state"><strong>No matching projects</strong><p>Try another search term or change the status filter.</p><Button variant="secondary" onClick={() => { setSearch(""); setStatusFilter("all"); }}>Clear filters</Button></div>
      ) : null}

      {query.isSuccess && visibleProjects.length > 0 ? (
        <ul className="project-card-grid">
          {visibleProjects.map((project) => (
            <li key={project.id}>
              <Link
                href={`/projects/${project.id}`}
                className="project-card block transition"
              >
                <div className="flex items-start justify-between gap-3">
                  <span className="project-monogram" aria-hidden="true">{project.name.trim().slice(0, 1).toUpperCase() || "P"}</span>
                  <span className="project-card-status">{project.status.replaceAll("_", " ")}</span>
                </div>
                <h2 className="mt-4 font-semibold">{project.name}</h2>
                <div className="flex items-center justify-between gap-2">
                  <span className="text-[11px] text-text-muted">Open project <span aria-hidden="true">↗</span></span>
                </div>
                <p className="mt-1 text-xs text-text-muted">{project.slug}</p>
                <p className="mt-3 line-clamp-2 text-sm text-text-secondary">
                  {project.description || "No description"}
                </p>
              </Link>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
