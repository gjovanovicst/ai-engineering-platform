// @vitest-environment jsdom
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { NextAction } from "./next-action";

describe("NextAction", () => {
  it("runs the chosen action", async () => {
    const onClick = vi.fn();
    render(<NextAction title="Approve the plan" description="The agent waits for a human." actions={[{ label: "Approve", onClick }]} />);
    expect(screen.getByText("Approve the plan")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Approve" }));
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("renders nothing without actions", () => {
    const { container } = render(<NextAction title="x" description="y" actions={[]} />);
    expect(container).toBeEmptyDOMElement();
  });
});
