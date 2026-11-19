// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { CommandPalette } from "./command-palette";

const push = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));

const listProjects = vi.fn(async () => ({ data: [] }));
const searchTasks = vi.fn(async () => ({ data: [] }));
const listOrgRuns = vi.fn(async () => ({ data: [] }));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    listProjects: (...args: unknown[]) => listProjects(...(args as [])),
    searchTasks: (...args: unknown[]) => searchTasks(...(args as [])),
    listOrgRuns: (...args: unknown[]) => listOrgRuns(...(args as [])),
  };
});

function renderPalette(onClose = vi.fn()) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <CommandPalette orgID="o" onClose={onClose} />
    </QueryClientProvider>,
  );
  return onClose;
}

describe("CommandPalette", () => {
  it("lists the actions and runs the second one with the keyboard", async () => {
    renderPalette();
    expect(await screen.findByRole("option", { name: /Go to projects/ })).toBeInTheDocument();
    await userEvent.type(screen.getByRole("combobox"), "agents");
    await userEvent.keyboard("{Enter}");
    expect(push).toHaveBeenCalledWith("/agents");
  });

  it("closes on Escape", async () => {
    const onClose = renderPalette();
    await screen.findByRole("combobox");
    await userEvent.keyboard("{Escape}");
    expect(onClose).toHaveBeenCalled();
  });
});
