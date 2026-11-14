import { describe, expect, it } from "vitest";

import { lineDiff } from "./rule-diff";

describe("lineDiff", () => {
  it("marks added and removed lines", () => {
    const rows = lineDiff("a\nb\nc", "a\nB\nc");
    expect(rows.filter((r) => r.type === "del").map((r) => r.text)).toEqual(["b"]);
    expect(rows.filter((r) => r.type === "add").map((r) => r.text)).toEqual(["B"]);
    expect(rows.filter((r) => r.type === "same").map((r) => r.text)).toEqual(["a", "c"]);
  });

  it("reports identical bodies as all same", () => {
    expect(lineDiff("x\ny", "x\ny").every((r) => r.type === "same")).toBe(true);
  });
});
