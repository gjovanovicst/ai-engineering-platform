// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { EnvironmentsPanel } from "./environments-panel";

const listEnvironments = vi.fn(async () => ({ data: [] }));
const createEnvironment = vi.fn(async () => ({ id: "e1" }));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    listEnvironments: (...args: unknown[]) => listEnvironments(...(args as [])),
    createEnvironment: (...args: unknown[]) => createEnvironment(...(args as [])),
  };
});

function renderPanel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <EnvironmentsPanel orgID="o" projectID="p" />
    </QueryClientProvider>,
  );
}

describe("EnvironmentsPanel", () => {
  it("requires the environment URL before adding", async () => {
    renderPanel();
    await userEvent.click(await screen.findByRole("button", { name: /add environment/i }));
    await screen.findByRole("dialog");
    await userEvent.click(screen.getByRole("button", { name: /^add environment$/i }));
    expect(await screen.findByText("A URL is required.")).toBeInTheDocument();
    expect(createEnvironment).not.toHaveBeenCalled();
  });
});
