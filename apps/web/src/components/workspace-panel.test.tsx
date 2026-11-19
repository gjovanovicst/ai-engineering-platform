// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { WorkspacePanel } from "./workspace-panel";

const replace = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace }) }));

const getOrg = vi.fn(async () => ({ id: "o", name: "Acme", slug: "acme", region: "eu-central" }));
const renameOrg = vi.fn(async () => ({}));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    getOrg: (...args: unknown[]) => getOrg(...(args as [])),
    renameOrg: (...args: unknown[]) => renameOrg(...(args as [])),
  };
});

function renderPanel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <WorkspacePanel orgID="o" />
    </QueryClientProvider>,
  );
}

describe("WorkspacePanel", () => {
  it("requires a workspace name before saving", async () => {
    renderPanel();
    const name = await screen.findByLabelText("Name");
    await userEvent.clear(name);
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("A name is required.")).toBeInTheDocument();
    expect(renameOrg).not.toHaveBeenCalled();
  });
});
