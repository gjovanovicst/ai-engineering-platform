import type { Acceptance, Task } from "@/lib/api";

export type TaskComposerAction = "draft" | "ready" | "started";

/** Splits the comma-separated label input into trimmed, non-empty tags. */
export function tagList(raw: string): string[] {
  return raw
    .split(",")
    .map((tag) => tag.trim())
    .filter(Boolean);
}

/** The acceptance criteria that are complete (given, when and then present). */
export function completeAcceptance(items: Acceptance[]): Acceptance[] {
  return items.filter((a) => a.given.trim() && a.when.trim() && a.then.trim());
}

/** The Definition of Ready issues for a task; empty means ready. */
export function dorIssues(title: string, acceptance: Acceptance[]): string[] {
  const issues: string[] = [];
  if (!title.trim()) issues.push("A title is required.");
  if (title.trim().length > 200) issues.push("The title is longer than 200 characters.");
  if (completeAcceptance(acceptance).length === 0) {
    issues.push("At least one acceptance criterion needs given, when and then.");
  }
  return issues;
}

/** Joins the scope entries for a textarea (one per line). */
export function linesText(values: string[] | undefined): string {
  return (values ?? []).join("\n");
}

/** Splits a textarea into the scope entries. */
export function linesList(raw: string): string[] {
  return raw
    .split("\n")
    .map((line) => line.trim())
    .filter(Boolean);
}

/** The create/update payload derived from the composer state. */
export function taskInput(input: {
  title: string;
  description: string;
  references: string;
  priority: "low" | "med" | "high";
  tags: string;
  acceptance: Acceptance[];
  dependsOn: string[];
  files?: string;
  outOfScope?: string;
  testPlan?: string;
  rollback?: string;
}) {
  const description = [
    input.description.trim(),
    input.references.trim() ? `\n\nDesign references\n${input.references.trim()}` : "",
  ]
    .filter(Boolean)
    .join("");
  return {
    title: input.title.trim(),
    description,
    priority: input.priority,
    tags: tagList(input.tags),
    acceptance: completeAcceptance(input.acceptance),
    depends_on: input.dependsOn,
    files: linesList(input.files ?? ""),
    out_of_scope: linesList(input.outOfScope ?? ""),
    test_plan: linesList(input.testPlan ?? ""),
    rollback: linesList(input.rollback ?? ""),
  };
}

/** The refs of the dependencies, derived from the task's dependency ids. */
export function dependencyRefs(task: Task | undefined, tasks: Task[]): string[] {
  if (!task?.depends_on?.length) return [];
  return tasks.filter((t) => task.depends_on?.includes(t.id)).map((t) => t.ref);
}
