import tokens from "./tokens.json";

export type ColorGroup = {
  label: string;
  swatches: { name: string; value: string; className: string }[];
};

const color = tokens.color;

export const colorGroups: ColorGroup[] = [
  {
    label: "Background",
    swatches: [
      { name: "bg-canvas", value: color.bg.canvas, className: "bg-bg-canvas border border-border" },
      { name: "bg-surface", value: color.bg.surface, className: "bg-bg-surface border border-border" },
      { name: "bg-hover", value: color.bg.hover, className: "bg-bg-hover" },
      { name: "bg-selected", value: color.bg.selected, className: "bg-bg-selected" },
    ],
  },
  {
    label: "Text",
    swatches: [
      { name: "text-primary", value: color.text.primary, className: "bg-text-primary" },
      { name: "text-secondary", value: color.text.secondary, className: "bg-text-secondary" },
      { name: "text-muted", value: color.text.muted, className: "bg-text-muted" },
    ],
  },
  {
    label: "Brand and AI",
    swatches: [
      { name: "brand", value: color.brand.default, className: "bg-brand" },
      { name: "brand-hover", value: color.brand.hover, className: "bg-brand-hover" },
      { name: "brand-subtle", value: color.brand.subtle, className: "bg-brand-subtle" },
      { name: "ai", value: color.ai.default, className: "bg-ai" },
      { name: "ai-subtle", value: color.ai.subtle, className: "bg-ai-subtle" },
    ],
  },
  {
    label: "Status",
    swatches: [
      { name: "success", value: color.status.success, className: "bg-status-success" },
      { name: "warning", value: color.status.warning, className: "bg-status-warning" },
      { name: "danger", value: color.status.danger, className: "bg-status-danger" },
      { name: "info", value: color.status.info, className: "bg-status-info" },
    ],
  },
];

export const layout = tokens.layout;

export const typeScale = [
  { name: "text-[22px]", usage: "Page title", sample: "Projects", className: "text-[22px] font-semibold" },
  { name: "text-lg", usage: "Dialog title", sample: "Plan approval", className: "text-lg font-semibold" },
  { name: "text-sm", usage: "Body", sample: "The agent is waiting for a human decision.", className: "text-sm" },
  { name: "text-xs", usage: "Meta, labels", sample: "run.execute · 4.6s", className: "text-xs text-text-muted" },
  { name: "font-mono", usage: "Refs, ids, diffs", sample: "T-142 / aip/cd220edc", className: "font-mono text-xs" },
];

export const componentContracts = [
  {
    name: "Button",
    purpose: "The only interactive action element. One primary per view.",
    rules: "Variants primary/secondary/ghost/danger; loading shows …; default type=button.",
  },
  {
    name: "Input",
    purpose: "Single-line text entry, always inside a Field.",
    rules: "Never use placeholders as labels; errors are announced via role=alert.",
  },
  {
    name: "Field",
    purpose: "Label + control + error message.",
    rules: "label[for] is required; error text is visible and announced.",
  },
  {
    name: "Status chip",
    purpose: "Read-only state (run, task, approval).",
    rules: "Colour plus text; never colour alone (a11y).",
  },
  {
    name: "Dialog / drawer",
    purpose: "Focused task (task drawer, approvals).",
    rules: "aria-modal, Escape closes, focus is trapped and restored.",
  },
  {
    name: "Diff viewer",
    purpose: "Show exactly what the agent changed.",
    rules: "Additions/deletions use colour and a +/− prefix.",
  },
];
