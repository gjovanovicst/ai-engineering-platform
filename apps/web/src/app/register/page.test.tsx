// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import RegisterPage from "./page";

const replace = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace }) }));

const registerAccount = vi.fn(async () => ({}));
const login = vi.fn(async () => ({}));
vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return {
    ...actual,
    register: (...args: unknown[]) => registerAccount(...(args as [])),
    login: (...args: unknown[]) => login(...(args as [])),
  };
});

describe("RegisterPage", () => {
  it("rejects a short password before any request", async () => {
    render(<RegisterPage />);
    await userEvent.type(screen.getByLabelText("Your name"), "Jane");
    await userEvent.type(screen.getByLabelText("Work email"), "a@b.co");
    await userEvent.type(screen.getByLabelText("Password"), "short");
    await userEvent.click(screen.getByRole("button", { name: /create account/i }));
    expect(await screen.findByText("At least 12 characters.")).toBeInTheDocument();
    expect(registerAccount).not.toHaveBeenCalled();
  });
});
