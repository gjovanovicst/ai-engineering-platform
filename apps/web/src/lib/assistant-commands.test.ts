import { describe, expect, it } from "vitest";

import { assistantCommands, expandCommand } from "./assistant-commands";

describe("assistant commands", () => {
  it("lists the commands that map to the tools", () => {
    const names = assistantCommands.map((command) => command.name);
    expect(names).toContain("/create-task");
    expect(names).toContain("/create-project");
    expect(names).toContain("/retry");
    expect(names).toContain("/triage");
    expect(names).toContain("/standup");
    expect(names).toContain("/help");
  });

  it("expands a known command and keeps the arguments", () => {
    const result = expandCommand("/create-task Add SSO --project p1");
    expect(result?.command).toBe("/create-task");
    expect(result?.prompt).toContain("Add SSO --project p1");
    expect(result?.prompt).toContain("create_task");
  });

  it("returns null for free text and unknown commands", () => {
    expect(expandCommand("hello")).toBeNull();
    expect(expandCommand("/nope")).toBeNull();
  });
});
