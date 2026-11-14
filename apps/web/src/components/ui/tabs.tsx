"use client";

type TabItem = { id: string; label: string; count?: number };

export function Tabs({
  items,
  value,
  onChange,
  ariaLabel,
  variant = "underline",
}: {
  items: TabItem[];
  value: string;
  onChange: (id: string) => void;
  ariaLabel: string;
  variant?: "underline" | "pills";
}) {
  const pills = variant === "pills";
  return (
    <div
      role="tablist"
      aria-label={ariaLabel}
      className={
        pills
          ? "mb-5 flex w-fit gap-1 no-scrollbar overflow-x-auto overflow-y-hidden rounded-lg bg-bg-hover p-1"
          : "mb-5 no-scrollbar flex gap-1 overflow-x-auto overflow-y-hidden border-b border-border"
      }
    >
      {items.map((item) => {
        const active = item.id === value;
        return (
          <button
            key={item.id}
            role="tab"
            type="button"
            aria-selected={active}
            onClick={() => onChange(item.id)}
            className={
              pills
                ? `flex items-center gap-1.5 whitespace-nowrap rounded-md px-3 py-1.5 text-xs font-medium transition-colors ${
                    active ? "bg-bg-surface text-text-primary shadow-sm" : "text-text-secondary hover:text-text-primary"
                  }`
                : `-mb-px flex items-center gap-1.5 whitespace-nowrap border-b-2 px-3 py-2.5 text-sm transition-colors ${
                    active
                      ? "border-brand text-text-primary"
                      : "border-transparent text-text-secondary hover:text-text-primary"
                  }`
            }
          >
            {item.label}
            {typeof item.count === "number" ? (
              <span className="rounded-full bg-bg-hover px-1.5 py-0.5 text-[10px] font-medium text-text-muted">
                {item.count}
              </span>
            ) : null}
          </button>
        );
      })}
    </div>
  );
}
