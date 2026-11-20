"use client";

import { useEffect } from "react";

/** The field a form exposes to the assistant: a name, a label and a type the
 * apply understands. The values themselves stay in the form. */
export type AssistantFormField = {
  name: string;
  label: string;
  type: "text" | "textarea" | "list" | "select" | "number";
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

/** The shape the assistant panel sends as `ui_context.form`. */
export function assistantFormContext(): { id: string; fields: string[] } | null {
  const form = currentAssistantForm();
  return form ? { id: form.id, fields: form.fields.map((field) => field.name) } : null;
}

/** Applies the assistant's proposal: only the registered fields, only the
 * known kinds. Returns what was written and what was ignored. */
export function applyAssistantForm(values: Record<string, unknown>): { applied: string[]; ignored: string[] } {
  const form = currentAssistantForm();
  if (!form) return { applied: [], ignored: Object.keys(values) };
  const known = new Set(form.fields.map((field) => field.name));
  const applied = Object.keys(values).filter((name) => known.has(name));
  const ignored = Object.keys(values).filter((name) => !known.has(name));
  if (applied.length > 0) form.apply(values);
  return { applied, ignored };
}

/** Registers the form while it is mounted (the composer's drawer). */
export function useAssistantForm(form: AssistantForm): void {
  useEffect(() => registerAssistantForm(form), [form]);
}
