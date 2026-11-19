// @vitest-environment jsdom
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { CommandForm } from "./command-form";

describe("CommandForm", () => {
  it("rejects a command longer than 500 characters before saving", async () => {
    const onSave = vi.fn(async () => undefined);
    render(<CommandForm id="test-command" title="Test command" description="The gate." initial="" onSave={onSave} />);
    fireEvent.change(screen.getByLabelText("Test command"), { target: { value: "x".repeat(501) } });
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(await screen.findByText("At most 500 characters.")).toBeInTheDocument();
    expect(onSave).not.toHaveBeenCalled();
  });

  it("saves a valid command", async () => {
    const onSave = vi.fn(async () => undefined);
    render(<CommandForm id="test-command" title="Test command" description="The gate." initial="" onSave={onSave} />);
    await userEvent.type(screen.getByLabelText("Test command"), "go test ./...");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onSave).toHaveBeenCalledWith("go test ./...");
  });
});
