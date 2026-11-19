// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import AcceptInvitePage from "./page";

vi.mock("next/navigation", () => ({ useSearchParams: () => new URLSearchParams("org=o&token=t") }));

const ensureSession = vi.fn(async () => null);
const acceptInvitePublic = vi.fn(async () => ({ token: "j", org: "o", subject: "s", role: "member" }));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    ensureSession: (...args: unknown[]) => ensureSession(...(args as [])),
    acceptInvitePublic: (...args: unknown[]) => acceptInvitePublic(...(args as [])),
  };
});

describe("AcceptInvitePage", () => {
  it("rejects a short password before accepting", async () => {
    render(<AcceptInvitePage />);
    await screen.findByLabelText("Name");
    await userEvent.type(screen.getByLabelText("Name"), "Jane");
    await userEvent.type(screen.getByLabelText("Password"), "short");
    await userEvent.click(screen.getByRole("button", { name: /create account and join/i }));
    expect(await screen.findByText("At least 12 characters.")).toBeInTheDocument();
    expect(acceptInvitePublic).not.toHaveBeenCalled();
  });
});
