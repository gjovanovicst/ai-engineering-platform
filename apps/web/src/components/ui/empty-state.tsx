import type { ReactNode } from "react";

export function EmptyState({
  title,
  description,
  action,
}: {
  title: string;
  description?: string;
  action?: ReactNode;
}) {
  return (
    <div className="rounded-lg border border-dashed border-border bg-bg-surface p-8 text-center">
      <p className="text-sm font-medium text-text-primary">{title}</p>
      {description ? <p className="mx-auto mt-1 max-w-md text-xs text-text-secondary">{description}</p> : null}
      {action ? <div className="mt-4 flex justify-center">{action}</div> : null}
    </div>
  );
}
