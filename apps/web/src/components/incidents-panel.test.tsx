// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { IncidentsPanel } from "./incidents-panel";

const listIncidents = vi.fn(async () => ({ data: [] }));
const createIncident = vi.fn(async () => ({ id: "i1" }));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    listIncidents: (...args: unknown[]) => listIncidents(...(args as [])),
    createIncident: (...args: unknown[]) => createIncident(...(args as [])),
  };
});

function renderPanel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <IncidentsPanel orgID="o" />
    </QueryClientProvider>,
  );
}

describe("IncidentsPanel", () => {
  it("requires the title before opening an incident", async () => {
    renderPanel();
    await userEvent.click(await screen.findByRole("button", { name: "Open incident" }));
    expect(await screen.findByText("A title is required.")).toBeInTheDocument();
    expect(createIncident).not.toHaveBeenCalled();
  });
});
