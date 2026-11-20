import type { InputHTMLAttributes } from "react";

export function Input({ className = "", ...props }: InputHTMLAttributes<HTMLInputElement>) {
  const invalid = Boolean(props["aria-invalid"]);
  return (
    <input
      {...props}
      className={`h-9 w-full rounded-md border bg-bg-surface px-3 text-sm text-text-primary placeholder:text-text-muted focus:outline-none focus:ring-2 focus:ring-offset-2 ${
        invalid
          ? "border-status-danger focus:ring-status-danger"
          : "border-border focus:ring-brand"
      } ${className}`}
    />
  );
}
