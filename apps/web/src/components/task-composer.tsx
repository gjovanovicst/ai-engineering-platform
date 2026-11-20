"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { useFieldArray } from "react-hook-form";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { SidePanel } from "@/components/ui/side-panel";
import { ApiError, createTask, startRun, transitionTask, updateTask, uploadTaskAttachment, type Task } from "@/lib/api";
import { useAssistantForm } from "@/lib/assistant-form";
import { useZodForm } from "@/lib/form";
import { taskComposerSchema, type TaskComposerValues } from "@/lib/schemas";
import { completeAcceptance, dependencyRefs, dorIssues, linesList, linesText, tagList, taskInput, type TaskComposerAction } from "@/components/task-composer-model";
import { HelpTip } from "@/components/ui/help-tip";

const blankAC = { given: "", when: "", then: "" };
const priorities: { value: "low" | "med" | "high"; label: string }[] = [
  { value: "low", label: "Low" },
  { value: "med", label: "Medium" },
  { value: "high", label: "High" },
];

type Props = {
  orgID: string;
  projectID: string;
  tasks: Task[];
  task?: Task;
  onClose: () => void;
  onSaved: (task: Task, action: TaskComposerAction) => void;
};

export function TaskComposer({ orgID, projectID, tasks, task, onClose, onSaved }: Props) {
  const router = useRouter();
  const editing = Boolean(task);
  const defaults: TaskComposerValues = {
    title: task?.title ?? "",
    description: task?.description ?? "",
    references: "",
    priority: task?.priority ?? "med",
    tags: (task?.tags ?? []).join(", "),
    acceptance: task?.acceptance?.length ? task.acceptance.map((a) => ({ ...a })) : [{ ...blankAC }],
    files: linesText(task?.files),
    outOfScope: linesText(task?.out_of_scope),
    testPlan: linesText(task?.test_plan),
    rollback: linesText(task?.rollback),
    dependsOn: dependencyRefs(task, tasks),
  };
  const {
    register,
    handleSubmit,
    watch,
    setValue,
    control,
    formState: { errors, isDirty },
  } = useZodForm<TaskComposerValues>(taskComposerSchema, { defaultValues: defaults });
  const { fields, append, remove } = useFieldArray({ control, name: "acceptance" });
  const [attachments, setAttachments] = useState<File[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<TaskComposerAction | null>(null);

  const title = watch("title");
  const tags = watch("tags");
  const priority = watch("priority");
  const acceptance = watch("acceptance");
  const dependsOn = watch("dependsOn");
  const values = { files: watch("files"), testPlan: watch("testPlan"), rollback: watch("rollback") };

  const issues = dorIssues(title, acceptance);
  const ready = issues.length === 0;
  const previewTags = tagList(tags);
  const dependencyOptions = tasks.filter((t) => t.id !== task?.id);
  const completeCount = completeAcceptance(acceptance).length;


  function toggleDependency(ref: string) {
    setValue("dependsOn", dependsOn.includes(ref) ? dependsOn.filter((r) => r !== ref) : [...dependsOn, ref], {
      shouldDirty: true,
    });
  }

  async function perform(formValues: TaskComposerValues, action: TaskComposerAction) {
    setBusy(action);
    setError(null);
    try {
      const input = taskInput({
        title: formValues.title,
        description: formValues.description,
        references: formValues.references,
        priority: formValues.priority,
        tags: formValues.tags,
        acceptance: formValues.acceptance,
        dependsOn: formValues.dependsOn,
        files: formValues.files,
        outOfScope: formValues.outOfScope,
        testPlan: formValues.testPlan,
        rollback: formValues.rollback,
      });
      let saved: Task;
      if (editing && task) {
        saved = await updateTask(orgID, projectID, task.id, input);
      } else {
        saved = await createTask(orgID, projectID, input);
      }
      const failedUploads: string[] = [];
      for (const file of attachments) {
        try {
          await uploadTaskAttachment(orgID, projectID, saved.id, file);
        } catch {
          failedUploads.push(file.name);
        }
      }
      if (action !== "draft" && saved.status === "draft") {
        saved = await transitionTask(orgID, projectID, saved.id, "ready");
      }
      if (action === "started") {
        const run = await startRun(orgID, projectID, saved.ref);
        onSaved(saved, "started");
        router.push(`/projects/${projectID}/runs/${run.id}`);
        return;
      }
      if (failedUploads.length > 0) {
        setError(`The task was saved, but these files failed to upload: ${failedUploads.join(", ")}.`);
        return;
      }
      onSaved(saved, action);
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Could not save the task");
    } finally {
      setBusy(null);
    }
  }

  function save(action: TaskComposerAction) {
    if (busy) return;
    if (action !== "draft" && !ready) {
      setError(issues.join(" "));
      return;
    }
    void handleSubmit((formValues) => perform(formValues, action))();
  }

  // The assistant can read the fields and fill them (the human still submits).
  useAssistantForm({
    id: "task-composer",
    title: editing ? "Edit task" : "New task",
    fields: [
      { name: "title", label: "Title", type: "text" },
      { name: "description", label: "Description", type: "textarea" },
      { name: "acceptance", label: "Acceptance criteria", type: "list" },
      { name: "priority", label: "Priority", type: "select" },
    ],
    apply: (values) => {
      const set = (name: string, kind: "string" | "list") => {
        const value = values[name];
        if (kind === "string" && typeof value === "string") setValue(name as never, value as never, { shouldDirty: true });
        if (kind === "list" && Array.isArray(value)) setValue(name as never, value as never, { shouldDirty: true });
      };
      set("title", "string");
      set("description", "string");
      set("acceptance", "list");
      set("priority", "string");
    },
  });

  return (
    <SidePanel
      dirty={isDirty}
      size="xl"
      title={editing ? `Edit ${task?.ref}` : "New task"}
      description={
        editing
          ? "Update the task; the agent sees the same acceptance criteria and scope."
          : "A well-defined task gives the agent the context and the proof it needs. Create it as a draft, ready, or start planning immediately."
      }
      onClose={onClose}
      footer={
        <>
          <Button type="button" variant="secondary" onClick={onClose}>
            Cancel
          </Button>
          {editing ? (
            <>
              <Button type="button" variant="secondary" loading={busy === "draft"} onClick={() => save("draft")}>
                Save
              </Button>
              {task?.status === "draft" ? (
                <Button type="button" variant="secondary" disabled={!ready} loading={busy === "ready"} onClick={() => save("ready")}>
                  Save &amp; mark ready
                </Button>
              ) : null}
              {task?.status === "ready" ? (
                <Button type="button" disabled={!ready} loading={busy === "started"} onClick={() => save("started")}>
                  Save &amp; start planning
                </Button>
              ) : null}
            </>
          ) : (
            <>
              <Button type="button" variant="secondary" loading={busy === "draft"} onClick={() => save("draft")}>
                Save draft
              </Button>
              <Button type="button" variant="secondary" disabled={!ready} loading={busy === "ready"} onClick={() => save("ready")}>
                Create &amp; ready
              </Button>
              <Button type="button" disabled={!ready} loading={busy === "started"} onClick={() => save("started")}>
                Create &amp; start planning
              </Button>
            </>
          )}
        </>
      }
    >
      <div className="grid gap-6 p-5 xl:grid-cols-[minmax(0,1fr)_300px]">
        <div className="space-y-6">
          <section className="space-y-3">
            <div className="flex items-center gap-2">
              <span className="task-section-number">1</span>
              <h3 className="text-sm font-semibold">What needs to be done?</h3>
            </div>
            <Field label="Task title" htmlFor="task-title" error={errors.title?.message}>
              <Input
                id="task-title"
                maxLength={200}
                aria-invalid={Boolean(errors.title)}
                {...register("title")}
                placeholder="e.g. Add SSO login with Microsoft Entra ID"
              />
            </Field>
            <Field label="Description and context" htmlFor="task-description" error={errors.description?.message}>
              <textarea
                id="task-description"
                rows={5}
                {...register("description")}
                placeholder="Describe the problem, the current and expected behaviour, constraints and relevant technical context…"
                className="w-full rounded-md border border-border bg-bg-surface px-3 py-2.5 text-sm placeholder:text-text-muted focus:border-brand focus:outline-none focus:ring-2 focus:ring-brand/20"
              />
            </Field>
            <Field label="Design references and technical context" htmlFor="task-references">
              <textarea
                id="task-references"
                rows={2}
                {...register("references")}
                placeholder="Figma or wireframe links, API docs, related tickets, repository paths (one per line)…"
                className="w-full rounded-md border border-border bg-bg-surface px-3 py-2 text-sm placeholder:text-text-muted focus:border-brand focus:outline-none focus:ring-2 focus:ring-brand/20"
              />
            </Field>
            <div className="grid gap-3 sm:grid-cols-2">
              <Field label="Priority" htmlFor="task-priority">
                <select
                  id="task-priority"
                  {...register("priority")}
                  className="h-10 w-full rounded-md border border-border bg-bg-surface px-3 text-sm"
                >
                  {priorities.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="Labels" htmlFor="task-tags">
                <Input id="task-tags" {...register("tags")} placeholder="frontend, auth, security" />
              </Field>
            </div>
          </section>

          <section className="space-y-3 border-t border-border pt-5">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <div className="flex items-center gap-2">
                <span className="task-section-number">2</span>
                <h3 className="text-sm font-semibold">Acceptance criteria <HelpTip term="acceptance" /></h3>
              </div>
              <span className="text-xs text-text-muted">{completeCount} complete</span>
            </div>
            <p className="text-xs text-text-secondary">
              Each criterion is a testable outcome. At least one complete criterion is required before a task is ready.
            </p>
            <div className="space-y-3">
              {fields.map((field, index) => {
                const criterion = acceptance[index] ?? blankAC;
                const complete = Boolean(criterion.given.trim() && criterion.when.trim() && criterion.then.trim());
                return (
                  <div key={field.id} className="rounded-lg border border-border bg-bg-subtle p-3">
                    <div className="mb-2 flex items-center justify-between">
                      <span className="text-xs font-medium text-text-secondary">
                        Criterion {index + 1} {complete ? <span className="text-status-success">✓</span> : null}
                      </span>
                      {fields.length > 1 ? (
                        <button
                          type="button"
                          aria-label={`Remove criterion ${index + 1}`}
                          onClick={() => remove(index)}
                          className="text-xs text-text-muted hover:text-status-danger"
                        >
                          Remove
                        </button>
                      ) : null}
                    </div>
                    <div className="grid gap-2 md:grid-cols-3">
                      <Field label="Given" htmlFor={`ac-given-${index}`}>
                        <textarea
                          id={`ac-given-${index}`}
                          rows={2}
                          {...register(`acceptance.${index}.given`)}
                          placeholder="Context"
                          className="w-full rounded-md border border-border bg-bg-surface px-2 py-1.5 text-sm"
                        />
                      </Field>
                      <Field label="When" htmlFor={`ac-when-${index}`}>
                        <textarea
                          id={`ac-when-${index}`}
                          rows={2}
                          {...register(`acceptance.${index}.when`)}
                          placeholder="Action"
                          className="w-full rounded-md border border-border bg-bg-surface px-2 py-1.5 text-sm"
                        />
                      </Field>
                      <Field label="Then" htmlFor={`ac-then-${index}`}>
                        <textarea
                          id={`ac-then-${index}`}
                          rows={2}
                          {...register(`acceptance.${index}.then`)}
                          placeholder="Outcome"
                          className="w-full rounded-md border border-border bg-bg-surface px-2 py-1.5 text-sm"
                        />
                      </Field>
                    </div>
                  </div>
                );
              })}
            </div>
            {errors.acceptance?.message ? (
              <p role="alert" className="text-xs text-status-danger">
                {errors.acceptance.message}
              </p>
            ) : null}
            <Button type="button" variant="secondary" onClick={() => append({ ...blankAC })}>
              + Add criterion
            </Button>
          </section>

          <section className="space-y-3 border-t border-border pt-5">
            <div className="flex items-center gap-2">
              <span className="task-section-number">3</span>
              <h3 className="text-sm font-semibold">Scope and proof</h3>
            </div>
            <p className="text-xs text-text-secondary">
              Where the change lands, what is out of scope, the test that proves it and the rollback.
              Recommended before a task is handed to an agent.
            </p>
            <div className="grid gap-3 md:grid-cols-2">
              <Field label="Files and packages" htmlFor="task-files">
                <textarea id="task-files" rows={3} {...register("files")} placeholder="apps/api/internal/httpapi/runs.go (one per line)" className="w-full rounded-md border border-border bg-bg-surface px-2 py-1.5 text-sm" />
              </Field>
              <Field label="Out of scope" htmlFor="task-out-of-scope">
                <textarea id="task-out-of-scope" rows={3} {...register("outOfScope")} placeholder="What this task explicitly does not do" className="w-full rounded-md border border-border bg-bg-surface px-2 py-1.5 text-sm" />
              </Field>
              <Field label="The test that proves it" htmlFor="task-test-plan">
                <textarea id="task-test-plan" rows={3} {...register("testPlan")} placeholder="TestFoo in runs_test.go" className="w-full rounded-md border border-border bg-bg-surface px-2 py-1.5 text-sm" />
              </Field>
              <Field label="Rollback" htmlFor="task-rollback">
                <textarea id="task-rollback" rows={3} {...register("rollback")} placeholder="Revert the commit; the migration is additive" className="w-full rounded-md border border-border bg-bg-surface px-2 py-1.5 text-sm" />
              </Field>
            </div>
          </section>

          <section className="space-y-3 border-t border-border pt-5">
            <div className="flex items-center gap-2">
              <span className="task-section-number">4</span>
              <h3 className="text-sm font-semibold">Dependencies</h3>
            </div>
            <p className="text-xs text-text-secondary">Tasks that must be done before this one can start.</p>
            {dependencyOptions.length === 0 ? (
              <p className="rounded-lg border border-dashed border-border bg-bg-subtle p-3 text-xs text-text-secondary">
                No other tasks in this project yet.
              </p>
            ) : (
              <div className="max-h-44 space-y-1 overflow-y-auto rounded-lg border border-border bg-bg-subtle p-2">
                {dependencyOptions.map((option) => (
                  <label key={option.id} className="flex items-center gap-2 rounded-md px-2 py-1.5 text-sm hover:bg-bg-hover">
                    <input type="checkbox" checked={dependsOn.includes(option.ref)} onChange={() => toggleDependency(option.ref)} />
                    <span className="font-mono text-xs text-text-muted">{option.ref}</span>
                    <span className="truncate">{option.title}</span>
                  </label>
                ))}
              </div>
            )}
          </section>

          <section className="space-y-3 border-t border-border pt-5">
            <div className="flex items-center gap-2">
              <span className="task-section-number">5</span>
              <h3 className="text-sm font-semibold">Attachments</h3>
            </div>
            <p className="text-xs text-text-secondary">Screenshots, wireframes or logs; uploaded with the task when you save.</p>
            <input
              type="file"
              multiple
              aria-label="Task attachments"
              onChange={(e) => setAttachments((current) => [...current, ...Array.from(e.target.files ?? [])])}
              className="block w-full text-xs text-text-secondary"
            />
            {attachments.length > 0 ? (
              <ul className="space-y-1">
                {attachments.map((file, index) => (
                  <li key={`${file.name}-${index}`} className="flex items-center justify-between rounded-md border border-border bg-bg-subtle px-2 py-1 text-xs">
                    <span className="truncate">{file.name}</span>
                    <button
                      type="button"
                      aria-label={`Remove ${file.name}`}
                      onClick={() => setAttachments((current) => current.filter((_, i) => i !== index))}
                      className="text-text-muted hover:text-status-danger"
                    >
                      Remove
                    </button>
                  </li>
                ))}
              </ul>
            ) : null}
          </section>
        </div>

        <aside className="space-y-4">
          <div className="rounded-lg border border-border bg-bg-subtle p-4">
            <div className="flex items-center justify-between">
              <span className="text-xs font-semibold uppercase tracking-wider text-brand">Preview</span>
              <span className="text-xs text-text-muted">{task?.ref ?? "New"}</span>
            </div>
            <div className="mt-3 rounded-md border border-border bg-bg-surface p-3">
              <div className="flex items-center gap-2">
                <span className={`h-2 w-2 rounded-full ${priority === "high" ? "bg-status-danger" : priority === "med" ? "bg-status-warning" : "bg-text-muted"}`} />
                <span className="text-xs uppercase text-text-secondary">{priority}</span>
                <span className="ml-auto rounded-full bg-bg-hover px-2 py-0.5 text-[11px] text-text-muted">
                  {editing ? task?.status?.replaceAll("_", " ") : "draft"}
                </span>
              </div>
              <p className="mt-2 text-sm font-medium">{title || "Task title"}</p>
              <p className="mt-1 line-clamp-2 text-xs text-text-secondary">{watch("description") || "The description appears here."}</p>
              {previewTags.length > 0 ? (
                <div className="mt-2 flex flex-wrap gap-1">
                  {previewTags.map((tag) => (
                    <span key={tag} className="rounded bg-bg-hover px-1.5 py-0.5 text-[11px] text-text-secondary">
                      {tag}
                    </span>
                  ))}
                </div>
              ) : null}
              <p className="mt-2 text-xs text-text-muted">{completeCount} acceptance criteria</p>
            </div>
          </div>

          <div className="rounded-lg border border-border bg-bg-surface p-4">
            <h3 className="text-sm font-semibold">Definition of Ready <HelpTip term="dor" /></h3>
            <ul className="mt-2 space-y-2 text-xs">
              <li className={title.trim() ? "text-status-success" : "text-text-muted"}>{title.trim() ? "✓" : "○"} Title</li>
              <li className={completeCount > 0 ? "text-status-success" : "text-text-muted"}>
                {completeCount > 0 ? "✓" : "○"} At least one complete acceptance criterion
              </li>
              <li className={linesList(values.files).length > 0 ? "text-status-success" : "text-text-muted"}>
                {linesList(values.files).length > 0 ? "✓" : "○"} Files and packages (recommended)
              </li>
              <li className={linesList(values.testPlan).length > 0 ? "text-status-success" : "text-text-muted"}>
                {linesList(values.testPlan).length > 0 ? "✓" : "○"} The proving test (recommended)
              </li>
              <li className={linesList(values.rollback).length > 0 ? "text-status-success" : "text-text-muted"}>
                {linesList(values.rollback).length > 0 ? "✓" : "○"} Rollback (recommended)
              </li>
            </ul>
            {issues.length > 0 ? (
              <p className="mt-3 text-xs text-text-muted">{issues.join(" ")}</p>
            ) : (
              <p className="mt-3 text-xs text-status-success">Ready to run.</p>
            )}
            <p className="mt-3 text-[11px] text-text-muted">
              Saving as draft always works with a title; ready and start need the items above.
            </p>
          </div>
        </aside>
      </div>
      {error ? (
        <p role="alert" className="mx-5 mb-3 rounded-md border border-status-danger/30 bg-status-danger/10 p-3 text-sm text-status-danger">
          {error}
        </p>
      ) : null}
    </SidePanel>
  );
}
