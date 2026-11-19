// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { RepoCredentialControl } from "./repo-credential-panel";
import type { Repo } from "@/lib/api";

const getRepoCredential = vi.fn(async () => ({ configured: false }));
const setRepoCredential = vi.fn(async () => ({ configured: true }));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    getRepoCredential: (...args: unknown[]) => getRepoCredential(...(args as [])),
    setRepoCredential: (...args: unknown[]) => setRepoCredential(...(args as [])),
  };
});

const repo = { id: "r1", name: "acme/app", provider: "github", default_branch: "main" } as Repo;

describe("RepoCredentialControl", () => {
  it("requires the token before saving", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={client}>
        <RepoCredentialControl orgID="o" projectID="p" repo={repo} />
      </QueryClientProvider>,
    );
    await userEvent.click(await screen.findByRole("button", { name: /configure git credential/i }));
    await screen.findByRole("dialog");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("A token is required.")).toBeInTheDocument();
    expect(setRepoCredential).not.toHaveBeenCalled();
  });
});
