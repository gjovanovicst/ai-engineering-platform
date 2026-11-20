import { describe, expect, it } from "vitest";

import {
  applyAssistantForm,
  assistantFormContext,
  currentAssistantForm,
  registerAssistantForm,
} from "./assistant-form";

describe("the assistant form bridge", () => {
  it("exposes the fields and applies only the registered ones", () => {
    const applied: Record<string, unknown>[] = [];
    const unregister = registerAssistantForm({
      id: "task-composer",
      title: "New task",
      fields: [{ name: "title", label: "Title", type: "text" }],
      apply: (values) => applied.push(values),
    });
    expect(assistantFormContext()).toEqual({ id: "task-composer", fields: ["title"] });
    const result = applyAssistantForm({ title: "Dummy task", nope: 1 });
    expect(result).toEqual({ applied: ["title"], ignored: ["nope"] });
    expect(applied).toHaveLength(1);
    unregister();
    expect(currentAssistantForm()).toBeNull();
  });

  it("does nothing without a registered form", () => {
    expect(applyAssistantForm({ title: "x" })).toEqual({ applied: [], ignored: ["title"] });
    expect(assistantFormContext()).toBeNull();
  });
});
