import { beforeEach, describe, expect, it } from "vitest";

import {
  applyAssistantForm,
  assistantFormContext,
  currentAssistantForm,
  registerAssistantForm,
  resetAssistantForms,
} from "./assistant-form";

describe("the assistant form bridge", () => {
  beforeEach(() => resetAssistantForms());

  it("exposes the fields and applies only the registered ones", () => {
    const applied: Record<string, unknown>[] = [];
    const unregister = registerAssistantForm({
      id: "task-composer",
      title: "New task",
      fields: [
        { name: "title", label: "Title", type: "text" },
        { name: "priority", label: "Priority", type: "select", options: ["low", "med", "high"] },
      ],
      apply: (values) => applied.push(values),
    });
    expect(assistantFormContext()?.fields.map((field) => field.name)).toEqual(["title", "priority"]);
    const result = applyAssistantForm({ title: "Dummy task", nope: 1, priority: "medium" });
    // "medium" is not an allowed option: it is ignored, never written.
    expect(result).toEqual({ applied: ["title"], ignored: ["nope", "priority"] });
    const ok = applyAssistantForm({ priority: "med" });
    expect(ok).toEqual({ applied: ["priority"], ignored: [] });
    // two applied batches: the title, then the priority
    expect(applied).toHaveLength(2);
    unregister();
    expect(currentAssistantForm()).toBeNull();
  });

  it("does nothing without a registered form", () => {
    expect(applyAssistantForm({ title: "x" })).toEqual({ applied: [], ignored: ["title"] });
    expect(assistantFormContext()).toBeNull();
  });
});
