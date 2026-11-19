// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ScimPanel } from "./scim-panel";

const listScimTokens = vi.fn(async () => ({ data: [] }));
const createScimToken = vi.fn(async () => ({ id: "s1", token: "t" }));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    listScimTokens: (...args: unknown[]) => listScimTokens(...(args as [])),
    createScimToken: (...args: unknown[]) => createScimToken(...(args as [])),
  };
});

function renderPanel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <ScimPanel orgID="o" />
    </QueryClientProvider>,
  );
}

describe("ScimPanel", () => {
  it("requires the token name before creating", async () => {
    renderPanel();
    await userEvent.click(await screen.findByRole("button", { name: "Create token" }));
    expect(await screen.findByText("A name is required.")).toBeInTheDocument();
    expect(createScimToken).not.toHaveBeenCalled();
  });
});
