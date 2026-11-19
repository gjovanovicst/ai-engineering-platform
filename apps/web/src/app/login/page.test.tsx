// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import LoginPage from "./page";

const replace = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace }) }));

const login = vi.fn(async () => ({}));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return { ...actual, login: (...args: unknown[]) => login(...(args as [])) };
});

describe("LoginPage", () => {
  it("shows the field errors before any request", async () => {
    render(<LoginPage />);
    await userEvent.click(screen.getByRole("button", { name: /sign in to workspace/i }));
    expect(await screen.findByText("Enter a valid email address.")).toBeInTheDocument();
    expect(await screen.findByText("A password is required.")).toBeInTheDocument();
    expect(login).not.toHaveBeenCalled();
  });

  it("signs in with a valid email and password", async () => {
    render(<LoginPage />);
    await userEvent.type(screen.getByLabelText("Work email"), "a@b.co");
    await userEvent.type(screen.getByLabelText("Password"), "secret");
    await userEvent.click(screen.getByRole("button", { name: /sign in to workspace/i }));
    expect(login).toHaveBeenCalledWith("a@b.co", "secret");
    expect(replace).toHaveBeenCalledWith("/overview");
  });
});
