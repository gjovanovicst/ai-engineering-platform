// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import ProjectsPage from "./page";

const push = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ push, replace: vi.fn() }) }));

const ensureSession = vi.fn(async () => ({ sub: "s", org: "o", role: "owner", sid: "session", exp: 0 }));
const listProjects = vi.fn(async () => ({ data: [] }));
const createProject = vi.fn(async () => ({ id: "p1", name: "Portal" }));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    ensureSession: (...args: unknown[]) => ensureSession(...(args as [])),
    listProjects: (...args: unknown[]) => listProjects(...(args as [])),
    createProject: (...args: unknown[]) => createProject(...(args as [])),
  };
});

function renderPage() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <ProjectsPage />
    </QueryClientProvider>,
  );
}

describe("ProjectsPage", () => {
  it("shows the name error before creating a project", async () => {
    renderPage();
    await userEvent.click(await screen.findByRole("button", { name: /new project/i }));
    await screen.findByRole("dialog");
    await userEvent.click(screen.getByRole("button", { name: "Create project" }));
    expect(await screen.findByText("A name is required.")).toBeInTheDocument();
    expect(createProject).not.toHaveBeenCalled();
  });
});
