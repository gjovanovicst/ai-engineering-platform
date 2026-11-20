/** What the current page shows: the assistant names "this run" / "this task"
 * from the route (no page change needed, the URL is the source of truth). */
export type PageSelection = {
  project_id?: string;
  task_id?: string;
  run_id?: string;
  pr_id?: string;
};

/** Reads the selection from the pathname; the unknown shapes stay empty. */
export function pageSelection(pathname: string): PageSelection {
  const selection: PageSelection = {};
  const project = pathname.match(/^\/projects\/([^/]+)/)?.[1];
  if (project) selection.project_id = project;
  const run = pathname.match(/^\/projects\/[^/]+\/runs\/([^/]+)/)?.[1];
  if (run) selection.run_id = run;
  const pull = pathname.match(/^\/projects\/[^/]+\/pulls\/([^/]+)/)?.[1];
  if (pull) selection.pr_id = pull;
  const task = pathname.match(/^\/projects\/[^/]+\/tasks\/([^/]+)/)?.[1];
  if (task) selection.task_id = task;
  return selection;
}
