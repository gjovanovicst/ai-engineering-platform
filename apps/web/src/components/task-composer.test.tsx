// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { TaskComposer } from "./task-composer";

const push = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ push }) }));

const createTask = vi.fn(async () => ({
  id: "t1", ref: "T-1", status: "draft", title: "Add a health endpoint",
  description: "", acceptance: [], priority: "med", tags: [], depends_on: [],
  org_id: "o", project_id: "p", created_at: "", updated_at: "",
}));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    createTask: (...args: unknown[]) => createTask(...(args as [])),
    transitionTask: vi.fn(async () => ({ id: "t1", ref: "T-1", status: "ready" })),
    startRun: vi.fn(async () => ({ id: "run-1" })),
    uploadTaskAttachment: vi.fn(async () => ({})),
  };
});

function renderComposer() {
  return render(
    <TaskComposer
      orgID="o"
      projectID="p"
      tasks={[]}
      onClose={() => undefined}
      onSaved={() => undefined}
    />,
  );
}

describe("TaskComposer", () => {
  it("shows the title error when saving a draft without a title", async () => {
    renderComposer();
    await screen.findByRole("dialog");
    await userEvent.click(screen.getByRole("button", { name: "Save draft" }));
    expect(await screen.findByText("A title is required.")).toBeInTheDocument();
    expect(createTask).not.toHaveBeenCalled();
  });

  it("starts planning with a title and a complete acceptance criterion", async () => {
    renderComposer();
    await screen.findByRole("dialog");
    await userEvent.type(screen.getByLabelText("Task title"), "Add a health endpoint");
    await userEvent.type(screen.getByLabelText("Given"), "the API runs");
    await userEvent.type(screen.getByLabelText("When"), "GET /health");
    await userEvent.type(screen.getByLabelText("Then"), "200 ok");
    await userEvent.click(screen.getByRole("button", { name: "Create & start planning" }));
    expect(await screen.findByRole("dialog")).toBeInTheDocument();
    expect(createTask).toHaveBeenCalled();
  });
});
