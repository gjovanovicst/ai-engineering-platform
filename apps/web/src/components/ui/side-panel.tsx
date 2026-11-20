"use client";

import { useEffect, useState } from "react";
import { createPortal } from "react-dom";

import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";

const panelWidths = {
  md: "max-w-[640px]",
  lg: "max-w-[960px]",
  xl: "max-w-[1200px]",
} as const;

export function SidePanel({
  title,
  description,
  onClose,
  children,
  footer,
  size = "md",
  dirty = false,
  z = 100,
}: {
  title: string;
  description?: string;
  onClose: () => void;
  children: React.ReactNode;
  footer?: React.ReactNode;
  size?: keyof typeof panelWidths;
  /** A changed form: ask before the panel is closed and the input is lost. */
  dirty?: boolean;
  /** The stacking order: the assistant opens above a drawer (z 130). */
  z?: number;
}) {
  const [mounted, setMounted] = useState(false);
  const [confirmOpen, setConfirmOpen] = useState(false);

  useEffect(() => {
    setMounted(true);
  }, []);

  const close = () => {
    if (dirty) {
      setConfirmOpen(true);
      return;
    }
    onClose();
  };

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") close();
    };
    document.addEventListener("keydown", onKey);
    const previous = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    return () => {
      document.removeEventListener("keydown", onKey);
      document.body.style.overflow = previous;
    };
  });

  if (!mounted) return null;

  return createPortal(
    <>
      <ConfirmDialog
        open={confirmOpen}
        title="Discard the changes?"
        message="You have unsaved changes. Closing now loses them."
        confirmLabel="Discard"
        cancelLabel="Keep editing"
        onConfirm={() => {
          setConfirmOpen(false);
          onClose();
        }}
        onCancel={() => setConfirmOpen(false)}
      />
    <div className="fixed inset-0 z-[100] flex justify-end" style={{ zIndex: z }} role="presentation">
      <button
        type="button"
        aria-label="Close panel"
        className="absolute inset-0"
        style={{ backgroundColor: "rgb(var(--overlay))" }}
        onClick={close}
      />
      <aside
        role="dialog"
        aria-modal="true"
        aria-labelledby="side-panel-title"
        className={`relative flex h-full w-full ${panelWidths[size]} flex-col border-l border-border bg-bg-surface shadow-2xl`}
      >
        <header className="flex items-start justify-between gap-4 border-b border-border px-5 py-4">
          <div>
            <h2 id="side-panel-title" className="text-base font-semibold">{title}</h2>
            {description ? <p className="mt-1 text-xs text-text-secondary">{description}</p> : null}
          </div>
          <div className="flex shrink-0 items-center gap-1">
            <button
              type="button"
              aria-label="Open the assistant"
              title="Open the assistant (⌘/)"
              onClick={() => window.dispatchEvent(new CustomEvent("aip:assistant"))}
              className="grid h-8 w-8 place-items-center rounded-md text-text-muted transition-colors hover:bg-bg-hover hover:text-brand focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/30"
            >
              <span aria-hidden="true" className="text-base leading-none">✦</span>
            </button>
            <button
              type="button"
              aria-label="Close panel"
              onClick={close}
              className="grid h-8 w-8 place-items-center rounded-md text-text-muted transition-colors hover:bg-bg-hover hover:text-text-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand/30"
            >
              <span aria-hidden="true" className="text-lg leading-none">×</span>
            </button>
          </div>
        </header>
        <div className="flex-1 overflow-y-auto overscroll-contain">{children}</div>
        <footer className="flex items-center justify-end gap-2 border-t border-border bg-bg-subtle px-5 py-3">
          {footer ?? (
            <Button variant="secondary" onClick={close}>
              Close
            </Button>
          )}
        </footer>
      </aside>
    </div>
    </>,
    document.body,
  );
}
