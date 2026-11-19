// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import SearchPage from "./page";

const replace = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace }) }));

const ensureSession = vi.fn(async () => ({ sub: "s", org: "o", role: "owner", sid: "session", exp: 0 }));
const searchTasks = vi.fn(async () => ({ data: [] }));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    ensureSession: (...args: unknown[]) => ensureSession(...(args as [])),
    searchTasks: (...args: unknown[]) => searchTasks(...(args as [])),
  };
});

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <SearchPage />
    </QueryClientProvider>,
  );
}

describe("SearchPage", () => {
  it("requires at least two characters before searching", async () => {
    renderPage();
    await screen.findByLabelText("Search tasks");
    await userEvent.type(screen.getByLabelText("Search tasks"), "x");
    await userEvent.click(screen.getByRole("button", { name: "Search" }));
    expect(await screen.findByText("Type at least two characters.")).toBeInTheDocument();
    expect(searchTasks).not.toHaveBeenCalled();
  });
});
