// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));

import { TaskDrawer } from "./task-drawer";
import type { Task } from "@/lib/api";

const task = {
  id: "t1",
  org_id: "o",
  project_id: "p",
  ref: "T-1",
  title: "Test task",
  description: "",
  status: "ready",
  priority: "med",
  acceptance: [],
} as unknown as Task;

function renderDrawer(onClose: () => void) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <TaskDrawer orgID="o" projectID="p" task={task} onClose={onClose} onEdit={() => undefined} />
    </QueryClientProvider>,
  );
}

describe("TaskDrawer", () => {
  it("closes on the Close button", async () => {
    const onClose = vi.fn();
    renderDrawer(onClose);
    await userEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalled();
  });

  it("offers the transitions in the footer", () => {
    renderDrawer(() => undefined);
    expect(screen.getByRole("button", { name: /start planning/i })).toBeInTheDocument();
  });
});
