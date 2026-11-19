"use client";

import { Button } from "@/components/ui/button";

export type NextActionItem = {
  label: string;
  onClick: () => void;
  variant?: "primary" | "secondary" | "danger";
  loading?: boolean;
};

/** The one thing to do next on a page, with its alternatives. */
export function NextAction({
  title,
  description,
  actions,
}: {
  title: string;
  description: string;
  actions: NextActionItem[];
}) {
  if (actions.length === 0) return null;
  return (
    <section
      aria-label="Next action"
      className="mb-5 rounded-lg border border-brand/40 bg-brand/5 p-4"
    >
      <p className="text-xs font-semibold uppercase tracking-wider text-brand">Next</p>
      <h2 className="mt-1 text-sm font-semibold">{title}</h2>
      <p className="mt-1 text-xs text-text-secondary">{description}</p>
      <div className="mt-3 flex flex-wrap items-center gap-2">
        {actions.map((action) => (
          <Button
            key={action.label}
            variant={action.variant ?? "secondary"}
            loading={action.loading}
            onClick={action.onClick}
          >
            {action.label}
          </Button>
        ))}
      </div>
    </section>
  );
}
