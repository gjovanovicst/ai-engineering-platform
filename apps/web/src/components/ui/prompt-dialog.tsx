"use client";

import { useEffect, useState } from "react";
import { createPortal } from "react-dom";

import { Button } from "@/components/ui/button";

/** The design-system input prompt. Replaces window.prompt: a labelled input,
 * the overlay and Escape, on the app's tokens. */
export function PromptDialog({
  open,
  title,
  message,
  label,
  placeholder,
  confirmLabel = "Save",
  cancelLabel = "Cancel",
  required = false,
  onConfirm,
  onCancel,
}: {
  open: boolean;
  title: string;
  message?: string;
  label: string;
  placeholder?: string;
  confirmLabel?: string;
  cancelLabel?: string;
  required?: boolean;
  onConfirm: (value: string) => void;
  onCancel: () => void;
}) {
  const [value, setValue] = useState("");
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    if (!open) return;
    setValue("");
    setError(null);
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onCancel();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onCancel]);

  if (!open) return null;

  const submit = () => {
    const trimmed = value.trim();
    if (required && trimmed === "") {
      setError("This field is required.");
      return;
    }
    onConfirm(trimmed);
  };

  return createPortal(
    <div className="fixed inset-0 z-[120] grid place-items-center p-4" role="presentation">
      <button
        type="button"
        aria-label="Close the dialog"
        className="absolute inset-0"
        style={{ backgroundColor: "rgb(var(--overlay))" }}
        onClick={onCancel}
      />
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="prompt-dialog-title"
        className="relative w-full max-w-sm rounded-lg border border-border bg-bg-surface p-5 shadow-2xl"
      >
        <h2 id="prompt-dialog-title" className="text-base font-semibold">{title}</h2>
        {message ? <p className="mt-2 text-sm text-text-secondary">{message}</p> : null}
        <label htmlFor="prompt-dialog-input" className="mt-3 block text-xs font-medium text-text-secondary">
          {label}
        </label>
        <input
          id="prompt-dialog-input"
          aria-label={label}
          autoFocus
          value={value}
          placeholder={placeholder}
          onChange={(event) => {
            setValue(event.target.value);
            setError(null);
          }}
          onKeyDown={(event) => {
            if (event.key === "Enter") {
              event.preventDefault();
              submit();
            }
          }}
          className="mt-1 h-9 w-full rounded-md border border-border bg-bg-surface px-2 text-sm focus:outline-none focus:ring-2 focus:ring-brand"
        />
        {error ? <p role="alert" className="mt-1 text-xs text-status-danger">{error}</p> : null}
        <div className="mt-4 flex justify-end gap-2">
          <Button variant="secondary" onClick={onCancel}>{cancelLabel}</Button>
          <Button onClick={submit}>{confirmLabel}</Button>
        </div>
      </div>
    </div>,
    document.body,
  );
}
