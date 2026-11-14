import type { ReactNode } from "react";

type Variant = "info" | "success" | "warning" | "danger";

const variants: Record<Variant, string> = {
  info: "border-brand/30 bg-brand-subtle/40 text-text-primary",
  success: "border-status-success/30 bg-status-success/10 text-status-success",
  warning: "border-status-warning/30 bg-status-warning/10 text-status-warning",
  danger: "border-status-danger/30 bg-status-danger/10 text-status-danger",
};

export function Alert({
  variant = "info",
  title,
  children,
}: {
  variant?: Variant;
  title?: string;
  children?: ReactNode;
}) {
  return (
    <div role={variant === "danger" ? "alert" : "status"} className={`rounded-md border p-3 text-sm ${variants[variant]}`}>
      {title ? <p className="font-medium">{title}</p> : null}
      {children ? <div className={title ? "mt-1 text-xs" : "text-xs"}>{children}</div> : null}
    </div>
  );
}
