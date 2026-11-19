// @vitest-environment jsdom
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { BYOKPanel } from "./byok-panel";

const getOrgCrypto = vi.fn(async () => ({ status: "disabled", kms_url: "", key_id: "" }));
const setOrgCrypto = vi.fn(async () => ({}));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    getOrgCrypto: (...args: unknown[]) => getOrgCrypto(...(args as [])),
    setOrgCrypto: (...args: unknown[]) => setOrgCrypto(...(args as [])),
  };
});

function renderPanel() {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <BYOKPanel orgID="o" />
    </QueryClientProvider>,
  );
}

describe("BYOKPanel", () => {
  it("requires the KMS URL before enabling", async () => {
    renderPanel();
    await userEvent.click(await screen.findByRole("button", { name: "Enable BYOK" }));
    expect(await screen.findByText("A KMS URL is required.")).toBeInTheDocument();
    expect(setOrgCrypto).not.toHaveBeenCalled();
  });
});
