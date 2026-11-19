// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { SSOPanel } from "./sso-panel";

const getSSO = vi.fn(async () => ({ entity_id: "", metadata_url: "", sso_url: "", certificate_pem: "", enabled: false }));
const saveSSO = vi.fn(async () => ({}));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    getSSO: (...args: unknown[]) => getSSO(...(args as [])),
    saveSSO: (...args: unknown[]) => saveSSO(...(args as [])),
  };
});

function renderPanel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <SSOPanel orgID="o" />
    </QueryClientProvider>,
  );
}

describe("SSOPanel", () => {
  it("rejects an entity id longer than 300 characters", async () => {
    renderPanel();
    fireEvent.change(await screen.findByLabelText("Entity ID"), { target: { value: "x".repeat(301) } });
    await userEvent.click(screen.getByRole("button", { name: "Save SSO" }));
    expect(await screen.findByText("At most 300 characters.")).toBeInTheDocument();
    expect(saveSSO).not.toHaveBeenCalled();
  });
});
