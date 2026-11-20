"use client";

import { useEffect } from "react";
import { createPortal } from "react-dom";

import { Button } from "@/components/ui/button";

/** The design-system confirmation. Replaces window.confirm: it carries the
 * app's tokens, the overlay and the focus handling. */
export function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel = "Discard",
  cancelLabel = "Keep editing",
  onConfirm,
  onCancel,
}: {
  open: boolean;
  title: string;
  message: string;
  confirmLabel?: string;
  cancelLabel?: string;
  onConfirm: () => void;
  onCancel: () => void;
}) {
  useEffect(() => {
    if (!open) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") onCancel();
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [open, onCancel]);

  if (!open) return null;

  return createPortal(
    <div className="fixed inset-0 z-[120] grid place-items-center p-4" role="presentation">
      <button
        type="button"
        aria-label="Close the confirmation"
        className="absolute inset-0"
        style={{ backgroundColor: "rgb(var(--overlay))" }}
        onClick={onCancel}
      />
      <div
        role="alertdialog"
        aria-modal="true"
        aria-labelledby="confirm-dialog-title"
        aria-describedby="confirm-dialog-message"
        className="relative w-full max-w-sm rounded-lg border border-border bg-bg-surface p-5 shadow-2xl"
      >
        <h2 id="confirm-dialog-title" className="text-base font-semibold">{title}</h2>
        <p id="confirm-dialog-message" className="mt-2 text-sm text-text-secondary">{message}</p>
        <div className="mt-4 flex justify-end gap-2">
          <Button variant="secondary" onClick={onCancel}>{cancelLabel}</Button>
          <Button variant="danger" onClick={onConfirm}>{confirmLabel}</Button>
        </div>
      </div>
    </div>,
    document.body,
  );
}
