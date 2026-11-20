// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { AssistantPanel } from "./assistant-panel";

const assistantStream = vi.fn(async (_org: string, input: { confirm?: boolean }, onEvent: (event: { type: string; text?: string; pending_tool?: string; conversation_id?: string }) => void) => {
  if (input.confirm) {
    onEvent({ type: "text", text: "Created T-9.", conversation_id: "c1" });
    return;
  }
  onEvent({ type: "pending", text: "I will create the task.", pending_tool: "create_task", conversation_id: "c1" });
});
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return { ...actual, assistantStream: (...args: unknown[]) => assistantStream(...(args as [string, { confirm?: boolean }, () => void])) };
});

describe("AssistantPanel", () => {
  it("shows the pending tool and asks for the confirmation", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <AssistantPanel orgID="o" onClose={() => undefined} />
      </QueryClientProvider>,
    );
    await userEvent.click(screen.getByRole("button", { name: /draft a task/i }));
    expect(await screen.findByText("I will create the task.")).toBeInTheDocument();
    expect(screen.getByText("Confirmation needed")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Confirm" }));
    expect(await screen.findByText("Created T-9.")).toBeInTheDocument();
    const lastCall = assistantStream.mock.calls.at(-1) as unknown as [string, { confirm?: boolean }];
    expect(lastCall[1].confirm).toBe(true);
  });
});
