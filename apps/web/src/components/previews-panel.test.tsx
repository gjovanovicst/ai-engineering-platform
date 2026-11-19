// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { PreviewsPanel } from "./previews-panel";

const listPreviews = vi.fn(async () => ({ items: [] }));
const createPreview = vi.fn(async () => ({ id: "pv1" }));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    listPreviews: (...args: unknown[]) => listPreviews(...(args as [])),
    createPreview: (...args: unknown[]) => createPreview(...(args as [])),
  };
});

function renderPanel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <PreviewsPanel orgID="o" projectID="p" />
    </QueryClientProvider>,
  );
}

describe("PreviewsPanel", () => {
  it("requires the image before creating a preview", async () => {
    renderPanel();
    await userEvent.click(await screen.findByRole("button", { name: "Create preview" }));
    expect(await screen.findByText("An image is required.")).toBeInTheDocument();
    expect(createPreview).not.toHaveBeenCalled();
  });
});
