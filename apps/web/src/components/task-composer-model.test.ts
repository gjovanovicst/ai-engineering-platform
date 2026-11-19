import { describe, expect, it } from "vitest";

import { completeAcceptance, dependencyRefs, dorIssues, linesList, linesText, tagList, taskInput } from "./task-composer-model";

describe("task composer model", () => {
  it("requires a title and one complete acceptance criterion for ready", () => {
    expect(dorIssues("", [])).toHaveLength(2);
    expect(dorIssues("Add health endpoint", [{ given: "a", when: "b", then: "" }])).toHaveLength(1);
    expect(dorIssues("Add health endpoint", [{ given: "a", when: "b", then: "c" }])).toHaveLength(0);
  });

  it("keeps only complete acceptance criteria", () => {
    const items = [
      { given: "a", when: "b", then: "c" },
      { given: "a", when: "", then: "c" },
    ];
    expect(completeAcceptance(items)).toHaveLength(1);
  });

  it("splits labels and appends the design references to the description", () => {
    expect(tagList("auth, ui ,, ")).toEqual(["auth", "ui"]);
    const input = taskInput({
      title: "  Add health  ",
      description: "body",
      references: "https://figma",
      priority: "high",
      tags: "api",
      acceptance: [{ given: "a", when: "b", then: "c" }],
      dependsOn: ["T-1"],
    });
    expect(input.title).toBe("Add health");
    expect(input.description).toContain("Design references");
    expect(input.tags).toEqual(["api"]);
    expect(input.depends_on).toEqual(["T-1"]);
  });

  it("maps dependency ids to refs", () => {
    const tasks = [
      { id: "1", ref: "T-1" },
      { id: "2", ref: "T-2" },
    ] as never;
    expect(dependencyRefs({ depends_on: ["2"] } as never, tasks)).toEqual(["T-2"]);
  });
});

describe("scope helpers", () => {
  it("joins and splits the scope entries", () => {
    expect(linesText(["a", "b"])).toBe("a\nb");
    expect(linesList(" a \n\nb ")).toEqual(["a", "b"]);
    expect(linesText(undefined)).toBe("");
  });

  it("includes the scope arrays in the payload", () => {
    const input = taskInput({
      title: "Add health",
      description: "",
      references: "",
      priority: "med",
      tags: "",
      acceptance: [{ given: "a", when: "b", then: "c" }],
      dependsOn: [],
      files: "apps/api/main.go",
      outOfScope: "auth",
      testPlan: "TestHealth",
      rollback: "revert",
    });
    expect(input.files).toEqual(["apps/api/main.go"]);
    expect(input.out_of_scope).toEqual(["auth"]);
    expect(input.test_plan).toEqual(["TestHealth"]);
    expect(input.rollback).toEqual(["revert"]);
  });
});
