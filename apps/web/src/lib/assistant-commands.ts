/** The slash commands the assistant panel understands. Each maps to the tool
 * the model may call; the client expands it into a deterministic prompt. */
export type AssistantCommand = { name: string; description: string; usage: string };

export const assistantCommands: AssistantCommand[] = [
  { name: "/create-task", description: "Draft a task (with acceptance criteria)", usage: "/create-task <title> [--project <id>]" },
  { name: "/create-project", description: "Draft a project", usage: "/create-project <name>" },
  { name: "/retry", description: "Retry a failed run", usage: "/retry <run-id>" },
  { name: "/cancel", description: "Cancel a running run", usage: "/cancel <run-id>" },
  { name: "/explain", description: "Explain a run (the failure and the next step)", usage: "/explain <run-id>" },
  { name: "/whats-next", description: "What needs attention in this workspace", usage: "/whats-next" },
  { name: "/projects", description: "List the projects with their status", usage: "/projects" },
  { name: "/tasks", description: "List a project's tasks (ref, status, acceptance)", usage: "/tasks <project-id>" },
  { name: "/runs", description: "The recent runs in the workspace", usage: "/runs" },
  { name: "/run", description: "One run: status, phase, plan, review", usage: "/run <run-id>" },
  { name: "/triage", description: "Duplicates, empty acceptance, blocked and junk drafts", usage: "/triage" },
  { name: "/standup", description: "What happened today: failed runs, blocked, done", usage: "/standup" },
  { name: "/help", description: "List the commands", usage: "/help" },
];

/** Expands a slash command into the assistant prompt, or null for free text. */
export function expandCommand(input: string): { command: string; prompt: string } | null {
  const trimmed = input.trim();
  if (!trimmed.startsWith("/")) return null;
  const head = trimmed.split(/\s+/)[0];
  const command = assistantCommands.find((entry) => entry.name === head);
  if (!command) return null;
  const args = trimmed.slice(head.length).trim();
  const prompts: Record<string, string> = {
    "/create-task": `Create a task: ${args || "(no arguments; ask me for the title and the project)"}. Call create_task, show the preview and wait for my confirmation.`,
    "/create-project": `Create a project: ${args || "(no arguments; ask me for the name)"}. Call create_project, show the preview and wait for my confirmation.`,
    "/retry": `Retry the failed run ${args || "(no run id; ask me)"}. Call retry_run, show the preview and wait for my confirmation.`,
    "/cancel": `Cancel the run ${args || "(no run id; ask me)"}. Call cancel_run, show the preview and wait for my confirmation.`,
    "/explain": `Explain what happened in the run ${args || "(no run id; ask me)"} and what to do next.`,
    "/whats-next": "What needs my attention in this workspace today?",
    "/projects": "List the projects in this workspace with their status and a one-line summary of each.",
    "/tasks": `List the tasks of the project ${args || "(no project id; ask me)"} with the ref, the status and whether each has a complete acceptance criterion.`,
    "/runs": "List the recent runs in this workspace: the id, the project, the agent type and the status.",
    "/run": `Read the run ${args || "(no run id; ask me)"} and explain its status, phase and result.`,
    "/triage": "Triage the workspace: find the likely duplicate tasks, the tasks with empty or placeholder acceptance criteria, the blocked ones and the junk drafts. Group them by project and name the refs.",
    "/standup": "Write a short standup: what failed today, what is blocked, what is running and what was done. Cite the run ids and the task refs.",
    "/help": "List your slash commands and what each does.",
  };
  return { command: command.name, prompt: prompts[command.name] ?? args };
}
