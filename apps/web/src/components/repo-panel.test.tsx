// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { RepoPanel } from "./repo-panel";

const listRepos = vi.fn(async () => ({ data: [] }));
const connectRepo = vi.fn(async () => ({ id: "r1" }));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    listRepos: (...args: unknown[]) => listRepos(...(args as [])),
    connectRepo: (...args: unknown[]) => connectRepo(...(args as [])),
  };
});

function renderPanel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <RepoPanel orgID="o" projectID="p" canManage />
    </QueryClientProvider>,
  );
}

describe("RepoPanel", () => {
  it("requires the repository before connecting", async () => {
    renderPanel();
    await userEvent.click(await screen.findByRole("button", { name: /connect repository/i }));
    await screen.findByRole("dialog");
    await userEvent.click(screen.getByRole("button", { name: "Connect" }));
    expect(await screen.findByText("A repository is required.")).toBeInTheDocument();
    expect(connectRepo).not.toHaveBeenCalled();
  });
});
