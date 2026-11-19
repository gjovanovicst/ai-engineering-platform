"use client";

import { useId, useState } from "react";

import { glossary } from "@/lib/glossary";

/** An inline "?" that explains a term on hover or focus (keyboard included). */
export function HelpTip({ term }: { term: keyof typeof glossary }) {
  const id = useId();
  const [open, setOpen] = useState(false);
  const entry = glossary[term];
  if (!entry) return null;
  return (
    <span className="relative inline-flex align-middle">
      <button
        type="button"
        aria-label={`What is ${entry.term}?`}
        aria-describedby={open ? id : undefined}
        onMouseEnter={() => setOpen(true)}
        onMouseLeave={() => setOpen(false)}
        onFocus={() => setOpen(true)}
        onBlur={() => setOpen(false)}
        onKeyDown={(event) => {
          if (event.key === "Escape") setOpen(false);
        }}
        className="ml-1 inline-flex h-4 w-4 cursor-help items-center justify-center rounded-full border border-border text-[10px] font-semibold text-text-muted hover:border-brand hover:text-brand"
      >
        ?
      </button>
      {open ? (
        <span
          id={id}
          role="tooltip"
          className="absolute bottom-full left-1/2 z-40 mb-2 w-64 -translate-x-1/2 rounded-md border border-border bg-bg-surface p-2.5 text-left text-xs font-normal text-text-secondary shadow-lg"
        >
          <strong className="mb-1 block text-text-primary">{entry.term}</strong>
          {entry.definition}
        </span>
      ) : null}
    </span>
  );
}
