import { describe, expect, it } from "vitest";

import { commandSchema, componentEntrySchema, designTokenSchema, taskComposerSchema, type TaskComposerValues } from "./schemas";

function composer(overrides: Partial<TaskComposerValues> = {}): TaskComposerValues {
  return {
    title: "Add a health endpoint",
    description: "",
    references: "",
    priority: "med",
    tags: "",
    acceptance: [{ given: "the API runs", when: "GET /health", then: "200 ok" }],
    files: "",
    outOfScope: "",
    testPlan: "",
    rollback: "",
    dependsOn: [],
    ...overrides,
  };
}

describe("task composer schema", () => {
  it("rejects an empty title", () => {
    const result = taskComposerSchema.safeParse(composer({ title: "   " }));
    expect(result.success).toBe(false);
  });

  it("rejects a title longer than 200 characters", () => {
    expect(taskComposerSchema.safeParse(composer({ title: "x".repeat(201) })).success).toBe(false);
  });

  it("accepts a minimal payload", () => {
    expect(taskComposerSchema.safeParse(composer()).success).toBe(true);
  });
});

describe("command schema", () => {
  it("accepts a one-line command", () => {
    expect(commandSchema.safeParse({ command: "go test ./..." }).success).toBe(true);
  });

  it("rejects a multi-line command", () => {
    expect(commandSchema.safeParse({ command: "go test ./...\nrm -rf /" }).success).toBe(false);
  });

  it("rejects a command longer than 500 characters", () => {
    expect(commandSchema.safeParse({ command: "x".repeat(501) }).success).toBe(false);
  });
});

describe("registry schemas", () => {
  it("requires a component name and caps the purpose", () => {
    const base = {
      name: "Button", status: "approved", category: "form", purpose: "", rules: "",
      framework: "", package: "", import_path: "", version: "", docs: "", example: "",
    };
    expect(componentEntrySchema.safeParse(base).success).toBe(true);
    expect(componentEntrySchema.safeParse({ ...base, name: "  " }).success).toBe(false);
    expect(componentEntrySchema.safeParse({ ...base, purpose: "x".repeat(501) }).success).toBe(false);
  });

  it("requires a token name and value", () => {
    expect(designTokenSchema.safeParse({ name: "color-brand", value: "#2563eb", category: "color" }).success).toBe(true);
    expect(designTokenSchema.safeParse({ name: "", value: "#2563eb", category: "color" }).success).toBe(false);
    expect(designTokenSchema.safeParse({ name: "a", value: "", category: "color" }).success).toBe(false);
  });
});
