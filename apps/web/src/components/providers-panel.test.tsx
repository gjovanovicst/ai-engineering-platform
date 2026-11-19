// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { ProvidersPanel } from "./providers-panel";

const listProviders = vi.fn(async () => ({ data: [] }));
const listModelPrices = vi.fn(async () => ({ data: [] }));
const createProvider = vi.fn(async () => ({ id: "p1" }));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    listProviders: (...args: unknown[]) => listProviders(...(args as [])),
    listModelPrices: (...args: unknown[]) => listModelPrices(...(args as [])),
    createProvider: (...args: unknown[]) => createProvider(...(args as [])),
  };
});

function renderPanel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <ProvidersPanel orgID="o" />
    </QueryClientProvider>,
  );
}

describe("ProvidersPanel", () => {
  it("requires the provider name before saving", async () => {
    renderPanel();
    await userEvent.click(await screen.findByRole("button", { name: /add provider/i }));
    await screen.findByRole("dialog");
    await userEvent.click(screen.getByRole("button", { name: "Save provider" }));
    expect(await screen.findByText("A name is required.")).toBeInTheDocument();
    expect(createProvider).not.toHaveBeenCalled();
  });
});
