// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { DesignStandardsPanel } from "./design-standards-panel";

const listDesignStandards = vi.fn(async () => ({ data: [] }));
const listDesignTokens = vi.fn(async () => ({ data: [{ name: "color-brand", value: "#2563eb", category: "color" }] }));
const createDesignStandard = vi.fn(async () => ({ id: "s1" }));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    listDesignStandards: (...args: unknown[]) => listDesignStandards(...(args as [])),
    listDesignTokens: (...args: unknown[]) => listDesignTokens(...(args as [])),
    createDesignStandard: (...args: unknown[]) => createDesignStandard(...(args as [])),
  };
});

function renderPanel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <DesignStandardsPanel orgID="o" />
    </QueryClientProvider>,
  );
}

describe("DesignStandardsPanel", () => {
  it("requires a name of at least 3 characters", async () => {
    renderPanel();
    await userEvent.click(await screen.findByRole("button", { name: /new standard/i }));
    await screen.findByRole("dialog");
    await userEvent.click(screen.getByRole("button", { name: "Create standard" }));
    expect(await screen.findByText("The name needs at least 3 characters.")).toBeInTheDocument();
    expect(createDesignStandard).not.toHaveBeenCalled();
  });
});
