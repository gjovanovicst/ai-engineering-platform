# 10 · Component contracts (supplement to §3)

Continuation of [06-design-system.md §3](06-design-system.md#3-component-contracts). Same format:
TS interface + variants + behavior. **Tokens exclusively from §1** — no new colors are defined
here, only mapping onto existing ones.

The components are derived from the mockups ([design/mockups/](../design/mockups/)) and cover
26 items that §3 did not define (10.1–10.26).

---

## Navigation

### 10.1. Tabs
```tsx
type TabsProps = {
  items: { id: string; label: string; badge?: number; disabled?: boolean }[];
  value: string;
  onChange: (id: string) => void;
  variant?: 'underline' | 'pill';   // default: 'underline'
};
```
- Underline: `border-b border-border`, active `border-b-2 border-brand text-text-primary`,
  inactive `text-text-secondary hover:text-text-primary`.
- Pill: `bg-bg-hover rounded-md p-0.5`, active `bg-bg-surface shadow-sm`.
- `role="tablist"`, `aria-selected`, ←/→ arrows move focus.

### 10.2. Breadcrumbs
```tsx
type BreadcrumbsProps = { items: { label: string; href?: string }[]; max?: number };
```
`text-xs text-text-muted`, separator `/`, last item `text-text-secondary` without a link.
`max` (default 4) → the middle items collapse into "…" with a tooltip. `aria-label="Breadcrumb"`.

### 10.3. CommandPalette (⌘K)
```tsx
type CommandPaletteProps = {
  open: boolean; onClose: () => void;
  groups: { label: string; items: { id: string; label: string; hint?: string; run: () => void }[] }[];
};
```
- Overlay `bg-black/40 backdrop-blur-sm`, panel `max-w-xl rounded-xl shadow-lg`.
- Input with `autoFocus`; fuzzy filter; ↑/↓ + Enter; Escape closes.
- Empty result → "No results" (not an empty panel).
- The shortcut is shown as `<Kbd>⌘K</Kbd>` in the topbar.

### 10.4. Switcher (Org / Project)
```tsx
type SwitcherProps = {
  value: { id: string; name: string; meta?: string };
  options: { id: string; name: string; meta?: string }[];
  onChange: (id: string) => void;
  searchable?: boolean;             // default: true for projects
};
```
Button `h-8 px-2.5 border border-border rounded-md text-sm`, chevron 14px.
Dropdown `w-64`, search input at the top if `searchable`, `max-h-80 overflow-y-auto`.
Switching org resets the cache (TanStack Query `queryClient.clear()`).

### 10.5. FilterBar
```tsx
type Filter = { id: string; label: string; type: 'select' | 'multi' | 'toggle' | 'search';
                options?: { value: string; label: string }[] };
type FilterBarProps = {
  filters: Filter[]; value: Record<string, unknown>;
  onChange: (id: string, value: unknown) => void;
  onClear?: () => void;
};
```
`flex flex-wrap gap-2`; active filter `border-brand text-brand bg-brand-subtle`.
Number of active filters > 0 → "Clear all" at the end. At 375px: horizontal scroll with
`-mx-4 px-4` (no wrapping into 3 rows).

### 10.6. ViewToggle
```tsx
type ViewToggleProps = { value: 'list' | 'board' | 'grid'; onChange: (v: string) => void;
                         options?: string[] };
```
`inline-flex border border-border rounded-md overflow-hidden`, active segment
`bg-bg-hover text-text-primary`; icons 16px + `aria-label` per segment.

---

## Domain (board and tasks)

### 10.7. KanbanBoard
```tsx
type KanbanBoardProps = {
  columns: { id: string; label: string; dotColor: string; count?: number }[];
  items: TaskCardData[];
  onMove: (taskId: string, toColumn: string, index: number) => Promise<void>;
  onSelect: (taskId: string) => void;
};
```
- Horizontal scroll (`overflow-x-auto`), columns `min-w-[280px]`.
- Drag & drop: optimistic move + rollback on `409` with a toast.
- Keyboard fallback: focus on a card → `M` opens the "Move to…" menu (a11y requirement).
- Empty column: dashed placeholder "Drop here", not an empty area.

### 10.8. KanbanColumn
```tsx
type KanbanColumnProps = {
  id: string; label: string; dotColor: string; count: number;
  children: React.ReactNode; onAdd?: () => void;
};
```
`bg-bg-hover rounded-lg p-3`; header `text-xs font-semibold uppercase text-text-muted`
+ dot 6px + count `bg-bg-surface px-1.5 rounded-full`.

### 10.9. TaskCard (complete)
```tsx
type TaskCardData = {
  id: string; ref: string; title: string;
  priority: 'low' | 'med' | 'high';
  tags: string[];
  assignee?: { name: string; avatarUrl?: string };
  agentRun?: { type: AgentType; status: AgentStatus; progress: number };
  tests?: { passed: number; total: number };
};
```
`bg-bg-surface border border-border rounded-lg p-3 cursor-pointer hover:shadow-md`.
Ref `text-xs text-text-muted font-mono`; title `text-sm font-medium` (max 2 lines,
`line-clamp-2`); priority badge on the right; tags `text-[11px] bg-bg-hover rounded px-1.5`.
Progress bar only if `agentRun.status === 'running'`.

### 10.10. Stepper (Wizard)
```tsx
type StepperProps = {
  steps: { id: string; label: string; description?: string }[];
  current: number;
  onStepClick?: (index: number) => void;   // only for already visited steps
};
```
- Completed: `bg-status-success text-white` + ✓; active: `bg-brand text-white` + number;
  future: `bg-bg-hover text-text-muted` + number. Line between them `bg-border` (completed
  `bg-status-success`).
- `aria-current="step"`; the Back/Next buttons are in the footer, not in the component.

---

## Forms

### 10.11. FormField
```tsx
type FormFieldProps = {
  label: string; htmlFor: string; required?: boolean;
  hint?: string; error?: string; children: React.ReactNode;
};
```
Label `text-xs font-medium text-text-secondary mb-1.5`; hint `text-xs text-text-muted`;
error `text-xs text-status-danger` + `aria-describedby`; `aria-invalid` on the input.

### 10.12. Input
```tsx
type InputProps = {
  size?: 'sm' | 'md';                 // h-8 / h-9
  invalid?: boolean; iconLeft?: React.ReactNode; iconRight?: React.ReactNode;
} & React.InputHTMLAttributes<HTMLInputElement>;
```
`bg-bg-surface border border-border rounded-md px-3 text-sm placeholder:text-text-muted`.
Focus: `ring-2 ring-brand ring-offset-2`. Invalid: `border-status-danger`.
Never `outline: none` without replacing it with a ring.

### 10.13. Select
```tsx
type SelectProps = {
  options: { value: string; label: string; disabled?: boolean }[];
  size?: 'sm' | 'md'; invalid?: boolean;
} & Omit<React.SelectHTMLAttributes<HTMLSelectElement>, 'size'>;
```
Uses a native `<select>` (a11y + mobile picker) with the same visual style as Input.
Custom dropdown only if search is needed — then `Combobox` (phase 2).

### 10.14. Checkbox
```tsx
type CheckboxProps = {
  checked: boolean | 'indeterminate';
  onCheckedChange: (v: boolean) => void;
  label?: string; description?: string; disabled?: boolean;
};
```
`h-4 w-4 rounded border-border text-brand`; `indeterminate` draws `—`.
Clicking the label associates it (same `id`), `aria-checked="mixed"` for indeterminate.

### 10.15. Radio
```tsx
type RadioGroupProps = {
  value: string; onValueChange: (v: string) => void;
  options: { value: string; label: string; description?: string }[];
  orientation?: 'horizontal' | 'vertical';
};
```
`role="radiogroup"`; arrow keys change the selection; focus ring on the active option.

### 10.16. Toggle (switch)
```tsx
type ToggleProps = { checked: boolean; onCheckedChange: (v: boolean) => void;
                     label?: string; disabled?: boolean };
```
`w-9 h-5 rounded-full bg-bg-hover data-[on]:bg-brand`, thumb `h-4 w-4 bg-white shadow-sm`.
`role="switch"` + `aria-checked`. Never for destructive actions (use Modal for those).

---

## Feedback

### 10.17. Toast
```tsx
type ToastProps = {
  variant: 'success' | 'error' | 'warning' | 'info';
  title: string; description?: string; duration?: number;   // default 5000
};
```
`fixed bottom-right z-50 w-[360px] rounded-lg border border-border bg-bg-surface shadow-lg`;
left edge `border-l-2 border-status-{variant}`; colored icon, text `text-text-primary`.
`role="status"` (info/success) / `role="alert"` (error/warning). Max 3 stacked,
older ones are retired. An "Undo" action is optional (5s window).

### 10.18. Alert
```tsx
type AlertProps = {
  variant: 'info' | 'success' | 'warning' | 'danger';
  title: string; description?: string;
  action?: { label: string; onClick: () => void };
  dismissible?: boolean;
};
```
`rounded-lg border p-4` + `bg-status-{v}/10 border-status-{v}/30 text-status-{v}` for the
icon/title; description `text-text-secondary`. Not used for toast messages.

### 10.19. ProgressBar
```tsx
type ProgressBarProps = {
  value: number; max?: number;          // 0..max
  size?: 'sm' | 'md';                   // h-1.5 / h-2
  tone?: 'brand' | 'success' | 'danger';
  showLabel?: boolean;
};
```
`bg-bg-hover rounded-full overflow-hidden`; fill `bg-brand` (gradient only for AI sections).
`role="progressbar"` + `aria-valuenow/min/max`. Label `text-xs text-text-muted`.

### 10.20. Sparkline
```tsx
type SparklineProps = { data: number[]; width?: number; height?: number;     // 96×24
                        tone?: 'brand' | 'success' | 'danger' };
```
Bar variant (as in the mockup): 7–14 bars `w-1.5 rounded-sm bg-brand/60`, heights
normalized to max. No axes, no tooltip; `aria-hidden` (the value is in the KpiCard label).

---

## Data

### 10.21. Pagination
```tsx
type PaginationProps = {
  hasNext: boolean; hasPrev: boolean;
  onNext: () => void; onPrev: () => void;
  pageSize?: number; total?: number;   // total only if known (expensive)
};
```
Keyset pagination: only Prev/Next (no page numbers). `aria-label="Pagination"`.

### 10.22. FileTree
```tsx
type FileTreeProps = {
  files: { path: string; status: 'added' | 'modified' | 'deleted'; additions?: number; deletions?: number }[];
  selected?: string; onSelect: (path: string) => void;
};
```
`font-mono text-xs`; folders collapsible; status colors: A `text-status-success`,
M `text-status-warning`, D `text-status-danger`; `+/−` on the right `text-text-muted`.

### 10.23. CodeBlock
```tsx
type CodeBlockProps = {
  code: string; language?: string;
  showLineNumbers?: boolean; maxHeight?: number; copyable?: boolean;
};
```
`bg-[#0A0E1A] rounded-md p-3 font-mono text-xs overflow-auto`; copy button in the corner
(`aria-label="Copy code"`, "Copied" confirmation for 2s).

---

## Overlay and misc

### 10.24. Tooltip / Popover
```tsx
type TooltipProps = { content: React.ReactNode; side?: 'top' | 'right' | 'bottom' | 'left';
                      children: React.ReactNode; delay?: number };   // default 300
```
Tooltip: `bg-bg-elevated border border-border rounded-md px-2 py-1 text-xs shadow-md`,
`role="tooltip"`, opens on focus too. Popover = the same + interactive content,
`role="dialog"` if it contains a form, Escape closes, click outside closes.

### 10.25. Avatar
```tsx
type AvatarProps = {
  name: string; src?: string; size?: 'xs' | 'sm' | 'md';   // 20 / 24 / 32
  status?: 'online' | 'away' | 'offline';
};
```
Initials on `bg-brand-subtle text-brand`; `alt={name}`; status dot 6px with a white
ring. Avatar group: max 3 + "+N" (`bg-bg-hover text-text-secondary`).

### 10.26. Kbd
```tsx
type KbdProps = { children: React.ReactNode };
```
`text-[11px] font-mono px-1.5 py-0.5 rounded border border-border bg-bg-hover text-text-muted`.
Used in CommandPalette and shortcut tooltips.
