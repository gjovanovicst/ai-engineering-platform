import type { ButtonHTMLAttributes } from "react";

type Variant = "primary" | "secondary" | "ghost" | "danger";

const variants: Record<Variant, string> = {
  primary: "bg-brand text-text-inverse hover:bg-brand-hover border border-transparent",
  secondary: "bg-bg-surface text-text-primary border border-border hover:bg-bg-hover",
  ghost: "bg-transparent text-text-secondary hover:bg-bg-hover border border-transparent",
  danger:
    "bg-status-danger/10 text-status-danger border border-status-danger/30 hover:bg-status-danger/20",
};

export function Button({
  variant = "primary",
  className = "",
  loading = false,
  disabled,
  type = "button",
  children,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement> & { variant?: Variant; loading?: boolean }) {
  return (
    <button
      {...props}
      type={type}
      disabled={disabled || loading}
      className={`inline-flex h-9 items-center justify-center gap-2 rounded-md px-4 text-sm font-medium transition duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-60 ${variants[variant]} ${className}`}
    >
      {loading ? "…" : children}
    </button>
  );
}
