// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { GitHubAppPanel } from "./github-app-panel";

const getGitHubApp = vi.fn(async () => ({ configured: true, github_app: { slug: "acme", app_id: 1 } }));
const createGitHubAppManifest = vi.fn(async () => ({}));
const setGitHubInstallation = vi.fn(async () => ({}));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    getGitHubApp: (...args: unknown[]) => getGitHubApp(...(args as [])),
    createGitHubAppManifest: (...args: unknown[]) => createGitHubAppManifest(...(args as [])),
    setGitHubInstallation: (...args: unknown[]) => setGitHubInstallation(...(args as [])),
  };
});

function renderPanel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <GitHubAppPanel orgID="o" />
    </QueryClientProvider>,
  );
}

describe("GitHubAppPanel", () => {
  it("requires a numeric installation id", async () => {
    renderPanel();
    await userEvent.click(await screen.findByRole("button", { name: "Save installation" }));
    expect(await screen.findByText("The installation id is a number.")).toBeInTheDocument();
    expect(setGitHubInstallation).not.toHaveBeenCalled();
  });
});
