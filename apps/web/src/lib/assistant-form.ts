"use client";

import { useEffect } from "react";

/** The field a form exposes to the assistant: a name, a label and a type the
 * apply understands. The values themselves stay in the form. */
export type AssistantFormField = {
  name: string;
  label: string;
  type: "text" | "textarea" | "list" | "select" | "number";
  /** The allowed values: the assistant picks one, never a near-miss. */
  options?: string[];
};

export type AssistantForm = {
  id: string;
  title: string;
  fields: AssistantFormField[];
  /** Writes the known values into the form; unknown names are ignored. */
  apply: (values: Record<string, unknown>) => void;
};

// One drawer at a time: the last registration wins, the unmount clears it.
let current: AssistantForm | null = null;

export function registerAssistantForm(form: AssistantForm): () => void {
  current = form;
  return () => {
    if (current?.id === form.id) current = null;
  };
}

export function currentAssistantForm(): AssistantForm | null {
  return current;
}

/** Clears the registry (the tests reset it between cases). */
export function resetAssistantForms(): void {
  current = null;
}

/** The shape the assistant panel sends as `ui_context.form`. */
export function assistantFormContext(): {
  id: string;
  fields: { name: string; type: string; options?: string[] }[];
} | null {
  const form = currentAssistantForm();
  return form
    ? {
        id: form.id,
        fields: form.fields.map((field) => ({
          name: field.name,
          type: field.type,
          ...(field.options ? { options: field.options } : {}),
        })),
      }
    : null;
}

/** Applies the assistant's proposal: only the registered fields, only the
 * known kinds. Returns what was written and what was ignored. */
export function applyAssistantForm(values: Record<string, unknown>): { applied: string[]; ignored: string[] } {
  const form = currentAssistantForm();
  if (!form) return { applied: [], ignored: Object.keys(values) };
  const fields = new Map(form.fields.map((field) => [field.name, field]));
  const applied: string[] = [];
  const ignored: string[] = [];
  for (const [name, value] of Object.entries(values)) {
    const field = fields.get(name);
    // Unknown name, or a value outside the allowed options: never applied.
    if (!field || (field.options && !field.options.includes(String(value)))) {
      ignored.push(name);
      continue;
    }
    applied.push(name);
  }
  if (applied.length > 0) {
    form.apply(Object.fromEntries(applied.map((name) => [name, values[name]])));
  }
  return { applied, ignored };
}

/** Registers the form while it is mounted (the composer's drawer). */
export function useAssistantForm(form: AssistantForm): void {
  useEffect(() => registerAssistantForm(form), [form]);
}
