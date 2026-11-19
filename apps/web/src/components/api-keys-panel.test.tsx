// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { APIKeysPanel } from "./api-keys-panel";

const listAPIKeys = vi.fn(async () => ({ items: [] }));
const createAPIKey = vi.fn(async () => ({ id: "k1", token: "t" }));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    listAPIKeys: (...args: unknown[]) => listAPIKeys(...(args as [])),
    createAPIKey: (...args: unknown[]) => createAPIKey(...(args as [])),
  };
});

function renderPanel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <APIKeysPanel orgID="o" />
    </QueryClientProvider>,
  );
}

describe("ApiKeysPanel", () => {
  it("requires a key name before creating", async () => {
    renderPanel();
    await userEvent.click(await screen.findByRole("button", { name: "Create key" }));
    expect(await screen.findByText("A name is required.")).toBeInTheDocument();
    expect(createAPIKey).not.toHaveBeenCalled();
  });
});
