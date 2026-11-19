// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { DRPanel } from "./dr-panel";

const getDRStatus = vi.fn(async () => ({
  wal_ok: true, wal_archived: 1, wal_failed: 0, wal_age_seconds: 3600,
  base_ok: true, base_backup: "nightly", base_age_seconds: 3600,
}));
const listRestoreRequests = vi.fn(async () => ({ data: [] }));
const createRestoreRequest = vi.fn(async () => ({}));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    getDRStatus: (...args: unknown[]) => getDRStatus(...(args as [])),
    listRestoreRequests: (...args: unknown[]) => listRestoreRequests(...(args as [])),
    createRestoreRequest: (...args: unknown[]) => createRestoreRequest(...(args as [])),
  };
});

function renderPanel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <DRPanel orgID="o" />
    </QueryClientProvider>,
  );
}

describe("DRPanel", () => {
  it("requires a target time before requesting a restore", async () => {
    renderPanel();
    await userEvent.click(await screen.findByRole("button", { name: "Request restore" }));
    expect(await screen.findByText("Choose a target time to restore to.")).toBeInTheDocument();
    expect(createRestoreRequest).not.toHaveBeenCalled();
  });
});
