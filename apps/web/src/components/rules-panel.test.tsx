// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { RulesPanel } from "./rules-panel";

const listRules = vi.fn(async () => ({ data: [] }));
const listRulePublishApprovals = vi.fn(async () => ({ data: [] }));
const createRule = vi.fn(async () => ({ id: "r1" }));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    listRules: (...args: unknown[]) => listRules(...(args as [])),
    listRulePublishApprovals: (...args: unknown[]) => listRulePublishApprovals(...(args as [])),
    createRule: (...args: unknown[]) => createRule(...(args as [])),
  };
});

function renderPanel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <RulesPanel orgID="o" />
    </QueryClientProvider>,
  );
}

describe("RulesPanel", () => {
  it("requires a rule name before creating", async () => {
    renderPanel();
    await userEvent.click(await screen.findByRole("button", { name: /new rule/i }));
    await screen.findByRole("dialog");
    await userEvent.click(screen.getByRole("button", { name: "Create rule" }));
    expect(await screen.findByText("A name is required.")).toBeInTheDocument();
    expect(createRule).not.toHaveBeenCalled();
  });
});
