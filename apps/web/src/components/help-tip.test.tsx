// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";

import { HelpTip } from "./ui/help-tip";

describe("HelpTip", () => {
  it("shows the definition on focus and hides it with Escape", async () => {
    render(<HelpTip term="run" />);
    const button = screen.getByRole("button", { name: "What is Run?" });
    await userEvent.tab();
    expect(button).toHaveFocus();
    const tooltip = await screen.findByRole("tooltip");
    expect(tooltip).toHaveTextContent("Run");
    await userEvent.keyboard("{Escape}");
    expect(screen.queryByRole("tooltip")).not.toBeInTheDocument();
  });
});
