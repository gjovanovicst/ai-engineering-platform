// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { AgentPluginsPanel } from "./agent-plugins-panel";

const listAgentPlugins = vi.fn(async () => ({ items: [] }));
const listProjects = vi.fn(async () => ({ data: [] }));
const createAgentPlugin = vi.fn(async () => ({ id: "p1" }));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    listAgentPlugins: (...args: unknown[]) => listAgentPlugins(...(args as [])),
    listProjects: (...args: unknown[]) => listProjects(...(args as [])),
    createAgentPlugin: (...args: unknown[]) => createAgentPlugin(...(args as [])),
  };
});

function renderPanel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <AgentPluginsPanel orgID="o" />
    </QueryClientProvider>,
  );
}

describe("AgentPluginsPanel", () => {
  it("requires the plugin name before registering", async () => {
    renderPanel();
    await userEvent.click(await screen.findByRole("button", { name: "Register" }));
    expect(await screen.findByText("A name is required.")).toBeInTheDocument();
    expect(createAgentPlugin).not.toHaveBeenCalled();
  });
});
