// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { InvitesPanel } from "./invites-panel";

const listInvites = vi.fn(async () => ({ data: [] }));
const createInvite = vi.fn(async () => ({ id: "i1", token: "t" }));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    listInvites: (...args: unknown[]) => listInvites(...(args as [])),
    createInvite: (...args: unknown[]) => createInvite(...(args as [])),
  };
});

function renderPanel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <InvitesPanel orgID="o" />
    </QueryClientProvider>,
  );
}

describe("InvitesPanel", () => {
  it("requires a valid email before inviting", async () => {
    renderPanel();
    await userEvent.click(await screen.findByRole("button", { name: /invite/i }));
    await screen.findByRole("dialog");
    await userEvent.click(screen.getByRole("button", { name: "Send invite" }));
    expect(await screen.findByText("Enter a valid email address.")).toBeInTheDocument();
    expect(createInvite).not.toHaveBeenCalled();
  });
});
