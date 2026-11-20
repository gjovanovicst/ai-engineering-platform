import { describe, expect, it } from "vitest";

import { steps } from "./page";

describe("setup wizard", () => {
  it("walks the four setup steps in order", () => {
    expect(steps.map((step) => step.key)).toEqual(["provider", "repository", "test", "sandbox"]);
  });

  it("each step has a title and a detail", () => {
    for (const step of steps) {
      expect(step.title.length).toBeGreaterThan(0);
      expect(step.detail.length).toBeGreaterThan(0);
    }
  });
});
