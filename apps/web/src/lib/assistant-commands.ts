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
  };
  return { command: command.name, prompt: prompts[command.name] ?? args };
}
