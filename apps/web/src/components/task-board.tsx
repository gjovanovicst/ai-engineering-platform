"use client";

import { useState } from "react";

import { PromptDialog } from "@/components/ui/prompt-dialog";
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { ApiError, transitionTask, type Task } from "@/lib/api";

const columns: { status: string; label: string }[] = [
  { status: "draft", label: "Draft" },
  { status: "ready", label: "Ready" },
  { status: "planning", label: "Planning" },
  { status: "in_progress", label: "In progress" },
  { status: "review", label: "Review" },
  { status: "done", label: "Done" },
  { status: "blocked", label: "Blocked" },
];

const priorityStyles: Record<string, string> = {
  high: "bg-status-warning/10 text-status-warning",
  med: "bg-bg-hover text-text-muted",
  low: "bg-bg-hover text-text-muted",
};

export function TaskBoard({
  orgID,
  projectID,
  tasks,
  onOpen,
}: {
  orgID: string;
  projectID: string;
  tasks: Task[];
  onOpen: (task: Task) => void;
}) {
  const queryClient = useQueryClient();
  const [dragging, setDragging] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [blockPrompt, setBlockPrompt] = useState<{ task: string; to: string } | null>(null);

  const move = useMutation({
    mutationFn: (input: { task: Task; to: string; reason?: string }) =>
      transitionTask(orgID, projectID, input.task.id, input.to, input.reason),
    onMutate: async (input) => {
      // Optimistic: move the card now, roll back by refetching on error.
      queryClient.setQueryData<{ data: Task[] }>(["tasks", orgID, projectID], (current) =>
        current
          ? { ...current, data: current.data.map((t) => (t.id === input.task.id ? { ...t, status: input.to } : t)) }
          : current,
      );
    },
    onSuccess: () => setError(null),
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : "Could not move the task");
      queryClient.invalidateQueries({ queryKey: ["tasks", orgID, projectID] });
    },
    onSettled: () => queryClient.invalidateQueries({ queryKey: ["tasks", orgID, projectID] }),
  });

  return (
    <>
    <section className="mt-8">
      <header className="mb-3 flex items-center justify-between">
        <div>
          <h2 className="text-sm font-semibold">Board</h2>
          <p className="text-xs text-text-secondary">Drag a card to change its status (valid transitions only).</p>
        </div>
      </header>

      {error ? (
        <p role="alert" className="mb-3 rounded-md border border-status-danger/30 bg-status-danger/10 p-2 text-xs text-status-danger">
          {error}
        </p>
      ) : null}

      <div className="task-board-grid">
        {columns.map((column) => {
          const items = tasks.filter((task) => task.status === column.status);
          return (
            <div
              key={column.status}
              onDragOver={(event) => event.preventDefault()}
              onDrop={(event) => {
                event.preventDefault();
                const id = dragging ?? event.dataTransfer.getData("text/plain");
                const task = tasks.find((t) => t.id === id);
                setDragging(null);
                if (!task || task.status === column.status) return;
                if (column.status === "blocked") {
                  // a reason is required: ask through the design dialog
                  setBlockPrompt({ task: task.id, to: column.status });
                  return;
                }
                move.mutate({ task, to: column.status });
              }}
              className="task-board-column"
            >
              <h3 className="task-board-column-head">
                <span><i className={`task-priority-dot ${column.status === "blocked" ? "task-priority-high" : column.status === "review" ? "task-priority-med" : ""}`} />{column.label}</span>
                <span className="task-board-count">{items.length}</span>
              </h3>
              <ul className="task-board-list">
                {items.map((task) => (
                  <li key={task.id}>
                    <button
                      type="button"
                      draggable
                      onDragStart={(event) => {
                        setDragging(task.id);
                        event.dataTransfer.setData("text/plain", task.id);
                      }}
                      onDragEnd={() => setDragging(null)}
                      onClick={() => onOpen(task)}
                      className="task-board-card cursor-grab active:cursor-grabbing"
                    >
                      <span className="task-board-card-meta">
                        <span className="font-mono">{task.ref}</span>
                        <span className={`rounded px-1.5 py-0.5 text-[9px] uppercase ${priorityStyles[task.priority] ?? ""}`}>
                          {task.priority}
                        </span>
                      </span>
                      <span className="task-board-card-title">{task.title}</span>
                      <span className="task-board-card-meta mt-3"><span><i className={`task-priority-dot ${task.priority === "high" ? "task-priority-high" : task.priority === "med" ? "task-priority-med" : ""}`} />{task.status.replaceAll("_", " ")}</span><span>Open ↗</span></span>
                    </button>
                  </li>
                ))}
                {items.length === 0 ? <li className="task-board-empty">No tasks in this stage</li> : null}
              </ul>
            </div>
          );
        })}
      </div>
    </section>
    <PromptDialog
      open={blockPrompt !== null}
      title="Why is this task blocked?"
      message="The reason is recorded in the audit trail."
      label="Blocked reason"
      placeholder="Waiting for the API contract…"
      confirmLabel="Block the task"
      required
      onConfirm={(reason) => {
        const target = blockPrompt;
        setBlockPrompt(null);
        if (target) {
          const task = tasks.find((entry) => entry.id === target.task);
          if (task) move.mutate({ task, to: target.to, reason });
        }
      }}
      onCancel={() => setBlockPrompt(null)}
    />
    </>
  );
}
