"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import {
  ApiError,
  downloadTaskAttachment,
  listRunChecks,
  listTaskAttachments,
  listTaskRuns,
  uploadTaskAttachment,
  transitionTask,
  type Task,
} from "@/lib/api";

const statusStyles: Record<string, string> = {
  draft: "bg-bg-hover text-text-muted",
  ready: "bg-status-warning/10 text-status-warning",
  planning: "bg-ai-subtle text-ai",
  in_progress: "bg-brand-subtle text-brand",
  review: "bg-status-warning/10 text-status-warning",
  done: "bg-status-success/10 text-status-success",
  blocked: "bg-status-danger/10 text-status-danger",
  canceled: "bg-bg-hover text-text-muted",
};

const runStyles: Record<string, string> = {
  queued: "text-text-muted",
  running: "text-brand",
  awaiting_approval: "text-status-warning",
  succeeded: "text-status-success",
  failed: "text-status-danger",
  canceled: "text-text-muted",
};

export function TaskDrawer({
  orgID,
  projectID,
  task,
  onClose,
  onEdit,
}: {
  orgID: string;
  projectID: string;
  task: Task;
  onClose: () => void;
  onEdit: (task: Task) => void;
}) {
  const queryClient = useQueryClient();
  const [mounted, setMounted] = useState(false);
  useEffect(() => setMounted(true), []);
  const [attachmentFile, setAttachmentFile] = useState<File | null>(null);
  const [attachmentError, setAttachmentError] = useState<string | null>(null);
  const attachments = useQuery({
    queryKey: ["task-attachments", orgID, projectID, task.id],
    queryFn: () => listTaskAttachments(orgID, projectID, task.id),
  });
  const transition = useMutation({
    mutationFn: (to: string) => transitionTask(orgID, projectID, task.id, to),
    onSuccess: () => {
      void queryClient.invalidateQueries();
      onClose();
    },
  });
  const uploadAttachment = useMutation({
    mutationFn: (file: File) => uploadTaskAttachment(orgID, projectID, task.id, file),
    onSuccess: () => {
      setAttachmentFile(null);
      setAttachmentError(null);
      queryClient.invalidateQueries({ queryKey: ["task-attachments", orgID, projectID, task.id] });
    },
    onError: (err) => setAttachmentError(err instanceof ApiError ? err.message : "Could not upload the file"),
  });

  const closeRef = useRef<HTMLButtonElement | null>(null);
  const panelRef = useRef<HTMLElement | null>(null);

  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    closeRef.current?.focus();
    return () => previous?.focus?.();
  }, []);

  useEffect(() => {
    const panel = panelRef.current;
    if (!panel) return;
    const handler = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
        return;
      }
      if (event.key !== "Tab") return;
      const focusable = panel.querySelectorAll<HTMLElement>(
        'a[href], button:not([disabled]), input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
      );
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };
    panel.addEventListener("keydown", handler);
    return () => panel.removeEventListener("keydown", handler);
  }, [onClose]);

  const runs = useQuery({
    queryKey: ["task-runs", orgID, task.id],
    queryFn: () => listTaskRuns(orgID, projectID, task.ref),
  });
  const latestRun = runs.data?.data[0];
  const tests = useQuery({
    queryKey: ["run-checks", orgID, latestRun?.id],
    enabled: Boolean(latestRun?.id),
    queryFn: () => listRunChecks(orgID, latestRun!.id),
  });

  useEffect(() => {
    const handler = (event: KeyboardEvent) => {
      if (event.key === "Escape") onClose();
    };
    document.addEventListener("keydown", handler);
    return () => document.removeEventListener("keydown", handler);
  }, [onClose]);

  if (!mounted) return null;

  return createPortal(
    <div className="fixed inset-0 z-40 flex justify-end">
      <button
        type="button"
        aria-label="Close task details"
        onClick={onClose}
        className="absolute inset-0 bg-black/40"
      />
      <aside
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby="task-drawer-title"
        className="relative z-50 flex h-full w-full max-w-md flex-col border-l border-border bg-bg-surface shadow-xl"
      >
        <header className="flex items-start justify-between border-b border-border p-4">
          <div>
            <p className="font-mono text-xs text-text-muted">{task.ref}</p>
            <h2 id="task-drawer-title" className="mt-1 text-lg font-semibold">
              {task.title}
            </h2>
          </div>
          <button
            ref={closeRef}
            type="button"
            onClick={onClose}
            className="rounded p-1 text-text-muted hover:bg-bg-hover"
            aria-label="Close task details"
          >
            ✕
          </button>
        </header>

        <div className="flex-1 space-y-6 overflow-y-auto overscroll-contain p-4">
          <div className="flex flex-wrap items-center gap-2 text-xs">
            <span className={`rounded-md px-2 py-0.5 font-medium ${statusStyles[task.status] ?? ""}`}>
              {task.status}
            </span>
            <span className="rounded-md bg-bg-hover px-2 py-0.5 uppercase text-text-muted">
              {task.priority}
            </span>
          </div>

          {task.description ? (
            <p className="text-sm text-text-secondary">{task.description}</p>
          ) : null}

          {task.acceptance && task.acceptance.length > 0 ? (
            <section>
              <h3 className="mb-2 text-xs font-semibold uppercase text-text-muted">
                Acceptance criteria
              </h3>
              <ul className="space-y-1 text-sm text-text-secondary">
                {task.acceptance.map((ac, index) => (
                  <li key={index}>
                    Given {ac.given}, when {ac.when}, then {ac.then}.
                  </li>
                ))}
              </ul>
            </section>
          ) : null}

          {(task.files?.length ?? 0) + (task.out_of_scope?.length ?? 0) + (task.test_plan?.length ?? 0) + (task.rollback?.length ?? 0) > 0 ? (
            <section>
              <h3 className="mb-2 text-xs font-semibold uppercase text-text-muted">Scope and proof</h3>
              <div className="space-y-2 text-sm text-text-secondary">
                {task.files?.length ? (
                  <div><p className="text-xs font-medium text-text-muted">Files</p><ul className="list-inside list-disc">{task.files.map((file) => <li key={file} className="font-mono text-xs">{file}</li>)}</ul></div>
                ) : null}
                {task.out_of_scope?.length ? (
                  <div><p className="text-xs font-medium text-text-muted">Out of scope</p><ul className="list-inside list-disc">{task.out_of_scope.map((item) => <li key={item}>{item}</li>)}</ul></div>
                ) : null}
                {task.test_plan?.length ? (
                  <div><p className="text-xs font-medium text-text-muted">The test that proves it</p><ul className="list-inside list-disc">{task.test_plan.map((item) => <li key={item}>{item}</li>)}</ul></div>
                ) : null}
                {task.rollback?.length ? (
                  <div><p className="text-xs font-medium text-text-muted">Rollback</p><ul className="list-inside list-disc">{task.rollback.map((item) => <li key={item}>{item}</li>)}</ul></div>
                ) : null}
              </div>
            </section>
          ) : null}

          <section>
            <div className="mb-2 flex items-center justify-between">
              <h3 className="text-xs font-semibold uppercase text-text-muted">Details</h3>
              <Button variant="secondary" className="h-7 px-2 text-xs" onClick={() => onEdit(task)}>
                Edit
              </Button>
            </div>
            <p className="text-xs text-text-secondary">{task.description || "No description."}</p>
          </section>

          <section>
            <section>
          <h3 className="mb-2 text-xs font-semibold uppercase text-text-muted">Tests</h3>
          {tests.isSuccess && tests.data.data.length === 0 ? (
            <p className="text-xs text-text-secondary">No test result recorded yet.</p>
          ) : null}
          <ul className="space-y-2">
            {tests.isSuccess
              ? tests.data.data.map((check) => (
                  <li key={`${check.name}-${check.commit_sha}`} className="rounded border border-border p-2 text-xs">
                    <span
                      className={`rounded px-1.5 py-0.5 text-[11px] ${
                        check.status === "passed"
                          ? "bg-status-success/10 text-status-success"
                          : check.status === "failed"
                            ? "bg-status-danger/10 text-status-danger"
                            : "bg-bg-hover text-text-muted"
                      }`}
                    >
                      {check.status}
                    </span>
                    <span className="ml-2 font-mono">{check.name}</span>
                    {check.output ? (
                      <pre className="mt-2 max-h-40 overflow-auto whitespace-pre-wrap text-[11px] text-text-secondary">
                        {check.output}
                      </pre>
                    ) : null}
                  </li>
                ))
              : null}
          </ul>
        </section>

        <h3 className="mb-2 text-xs font-semibold uppercase text-text-muted">Attachments</h3>
        <p className="mb-2 text-xs text-text-secondary">
          Wireframes, screenshots or specs. PNG, JPEG, WebP, PDF or text, up to 5 MiB.
        </p>
        <div className="mb-2 flex flex-wrap items-center gap-2">
          <input
            type="file"
            aria-label="Attachment file"
            accept="image/png,image/jpeg,image/webp,application/pdf,text/plain"
            onChange={(event) => setAttachmentFile(event.target.files?.[0] ?? null)}
            className="text-xs"
          />
          <Button
            variant="secondary"
            className="h-8 px-2 text-xs"
            disabled={!attachmentFile}
            loading={uploadAttachment.isPending}
            onClick={() => attachmentFile && uploadAttachment.mutate(attachmentFile)}
          >
            Upload
          </Button>
        </div>
        {attachmentError ? <p role="alert" className="mb-2 text-xs text-status-danger">{attachmentError}</p> : null}
        {attachments.isSuccess && attachments.data.data.length === 0 ? (
          <p className="mb-4 text-xs text-text-muted">No attachments yet.</p>
        ) : null}
        {attachments.isSuccess && attachments.data.data.length > 0 ? (
          <ul className="mb-4 space-y-1">
            {attachments.data.data.map((attachment) => (
              <li key={attachment.id} className="flex items-center gap-2 rounded border border-border p-2 text-xs">
                <span className="flex-1 truncate">{attachment.filename}</span>
                <span className="text-text-muted">{Math.max(1, Math.round(attachment.size_bytes / 1024))} KiB</span>
                <button
                  type="button"
                  className="text-brand hover:underline"
                  onClick={async () => {
                    try {
                      const blob = await downloadTaskAttachment(orgID, projectID, task.id, attachment.id);
                      const url = URL.createObjectURL(blob);
                      const link = document.createElement("a");
                      link.href = url;
                      link.download = attachment.filename;
                      link.click();
                      URL.revokeObjectURL(url);
                    } catch (err) {
                      setAttachmentError(err instanceof ApiError ? err.message : "Could not download the file");
                    }
                  }}
                >
                  Download
                </button>
              </li>
            ))}
          </ul>
        ) : null}

        <h3 className="mb-2 text-xs font-semibold uppercase text-text-muted">Runs</h3>
            {runs.isPending ? <p className="text-sm text-text-muted">Loading…</p> : null}
            {runs.isError ? (
              <p role="alert" className="text-sm text-status-danger">
                Could not load runs.
              </p>
            ) : null}
            {runs.isSuccess && runs.data.data.length === 0 ? (
              <p className="text-sm text-text-secondary">No runs yet.</p>
            ) : null}
            {runs.isSuccess && runs.data.data.length > 0 ? (
              <ul className="space-y-2">
                {runs.data.data.map((run) => (
                  <li key={run.id} className="rounded-md border border-border p-2 text-sm">
                    <div className="flex items-center justify-between">
                      <span className="font-medium">{run.agent_type}</span>
                      <span className={`text-xs ${runStyles[run.status] ?? ""}`}>{run.status}</span>
                    </div>
                    <Link
                      href={`/projects/${projectID}/runs/${run.id}`}
                      className="mt-1 block text-xs text-brand hover:underline"
                    >
                      Open run {run.id.slice(0, 8)} →
                    </Link>
                  </li>
                ))}
              </ul>
            ) : null}
          </section>
        </div>
        {taskTransitions(task.status).length > 0 ? (
          <footer className="flex items-center justify-end gap-2 border-t border-border bg-bg-subtle px-4 py-3">
            {taskTransitions(task.status).map((next) => (
              <Button
                key={next.to}
                variant={next.to === "in_progress" ? "primary" : "secondary"}
                loading={transition.isPending}
                onClick={() => transition.mutate(next.to)}
              >
                {next.label}
              </Button>
            ))}
          </footer>
        ) : null}
      </aside>
    </div>,
    document.body,
  );
}

/** The human transitions the drawer offers for a status. */
function taskTransitions(status: string): { label: string; to: string }[] {
  switch (status) {
    case "draft":
      return [{ label: "Move to ready", to: "ready" }];
    case "ready":
      return [{ label: "Start", to: "in_progress" }];
    case "in_progress":
      return [{ label: "Move to review", to: "review" }];
    case "blocked":
      return [{ label: "Unblock", to: "ready" }];
    default:
      return [];
  }
}
