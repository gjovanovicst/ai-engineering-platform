# 06 · Design System — AI Implementation Spec

> Source of truth for every UI decision. An AI agent implementing a screen MUST use
> the values and contracts defined here. If a value does not exist, it is **not invented**;
> either ask a question, or use the nearest defined token and record that in the PR.

## 0. Rules (MUST / NEVER)

**MUST**
- Only tokens from §1 — no hardcoded color, font, or spacing.
- Every component per the contract in §3.
- Every screen per the composition in §5.
- Light + dark through CSS variables (§1.3).
- Test at 375px, 768px, 1280px+.
- Every action has a loading, empty, and error state.

**NEVER**
- A new color/spacing/font without updating §1.
- Inline style for colors/spacing — only Tailwind classes or a token.
- `<div>` without a semantic reason (`section`, `article`, `nav`, `header`).
- An estimated value displayed as measured — every metric carries
  `source: 'measured' | 'estimated' | 'n/a'`.
- Animations longer than 300ms (exception: skeleton).
- Drop shadow on cards (shadows only for elevated: modal, dropdown, tooltip).

## 1. Design tokens

### 1.1. JSON (source of truth → `web/src/design/tokens.json`)

```json
{
  "color": {
    "bg": {
      "canvas":   { "light": "#F8FAFC", "dark": "#0B1120" },
      "surface":  { "light": "#FFFFFF", "dark": "#111827" },
      "elevated": { "light": "#FFFFFF", "dark": "#1E293B" },
      "hover":    { "light": "#F1F5F9", "dark": "#1F2937" },
      "selected": { "light": "#EEF2FF", "dark": "rgba(129,140,248,0.12)" }
    },
    "border": {
      "default": { "light": "#E2E8F0", "dark": "#1F2937" },
      "strong":  { "light": "#CBD5E1", "dark": "#334155" }
    },
    "text": {
      "primary":   { "light": "#0F172A", "dark": "#F1F5F9" },
      "secondary": { "light": "#475569", "dark": "#94A3B8" },
      "muted":     { "light": "#94A3B8", "dark": "#64748B" },
      "inverse":   { "light": "#FFFFFF", "dark": "#0B1120" }
    },
    "brand": {
      "default": { "light": "#6366F1", "dark": "#818CF8" },
      "hover":   { "light": "#4F46E5", "dark": "#A5B4FC" },
      "subtle":  { "light": "#EEF2FF", "dark": "rgba(129,140,248,0.12)" }
    },
    "ai": {
      "default":  { "light": "#8B5CF6", "dark": "#A78BFA" },
      "gradient": "linear-gradient(135deg, #6366F1, #8B5CF6)",
      "subtle":   { "light": "#F5F3FF", "dark": "rgba(139,92,246,0.12)" }
    },
    "status": {
      "success": { "light": "#10B981", "dark": "#34D399" },
      "warning": { "light": "#F59E0B", "dark": "#FBBF24" },
      "danger":  { "light": "#EF4444", "dark": "#F87171" },
      "info":    { "light": "#3B82F6", "dark": "#60A5FA" }
    }
  },
  "font": {
    "family": { "sans": "Inter, system-ui, sans-serif", "mono": "JetBrains Mono, monospace" },
    "size": { "xs": "12px", "sm": "13px", "base": "14px", "md": "16px", "lg": "18px",
              "xl": "20px", "2xl": "24px", "3xl": "30px", "4xl": "36px" },
    "weight": { "regular": 400, "medium": 500, "semibold": 600, "bold": 700 },
    "lineHeight": { "tight": 1.25, "normal": 1.5, "relaxed": 1.6 }
  },
  "space": { "0": "0px", "1": "4px", "2": "8px", "3": "12px", "4": "16px",
             "5": "20px", "6": "24px", "8": "32px", "10": "40px", "12": "48px" },
  "radius": { "sm": "6px", "md": "8px", "lg": "12px", "xl": "16px", "full": "9999px" },
  "shadow": {
    "none": "none",
    "sm": "0 1px 2px rgba(0,0,0,0.05)",
    "md": "0 4px 12px rgba(15,23,42,0.06)",
    "lg": "0 10px 24px rgba(15,23,42,0.10)"
  },
  "layout": {
    "sidebarWidth": "240px", "sidebarCollapsed": "64px", "topbarHeight": "56px",
    "contentMaxWidth": "1440px", "contentPaddingX": "32px", "contentPaddingXMobile": "16px"
  },
  "breakpoint": { "sm": "640px", "md": "768px", "lg": "1024px", "xl": "1280px", "2xl": "1536px" },
  "motion": { "fast": "100ms", "base": "150ms", "slow": "200ms",
              "easing": "cubic-bezier(0.16, 1, 0.3, 1)" }
}
```

### 1.2. Tailwind (`web/tailwind.config.ts`)

```ts
export default {
  darkMode: ['class'],
  content: ['./src/**/*.{ts,tsx}'],
  theme: {
    extend: {
      colors: {
        bg: { canvas: 'rgb(var(--bg-canvas) / <alpha-value>)',
              surface: 'rgb(var(--bg-surface) / <alpha-value>)',
              elevated: 'rgb(var(--bg-elevated) / <alpha-value>)',
              hover: 'rgb(var(--bg-hover) / <alpha-value>)',
              selected: 'rgb(var(--bg-selected) / <alpha-value>)' },
        border: { DEFAULT: 'rgb(var(--border) / <alpha-value>)',
                  strong: 'rgb(var(--border-strong) / <alpha-value>)' },
        text: { primary: 'rgb(var(--text-primary) / <alpha-value>)',
                secondary: 'rgb(var(--text-secondary) / <alpha-value>)',
                muted: 'rgb(var(--text-muted) / <alpha-value>)',
                inverse: 'rgb(var(--text-inverse) / <alpha-value>)' },
        brand: { DEFAULT: 'rgb(var(--brand) / <alpha-value>)',
                 hover: 'rgb(var(--brand-hover) / <alpha-value>)',
                 subtle: 'rgb(var(--brand-subtle) / <alpha-value>)' },
        ai: { DEFAULT: 'rgb(var(--ai) / <alpha-value>)',
              subtle: 'rgb(var(--ai-subtle) / <alpha-value>)' },
        status: { success: 'rgb(var(--status-success) / <alpha-value>)',
                  warning: 'rgb(var(--status-warning) / <alpha-value>)',
                  danger: 'rgb(var(--status-danger) / <alpha-value>)',
                  info: 'rgb(var(--status-info) / <alpha-value>)' },
      },
      fontFamily: { sans: ['Inter', 'system-ui', 'sans-serif'], mono: ['JetBrains Mono', 'monospace'] },
      borderRadius: { sm: '6px', md: '8px', lg: '12px', xl: '16px' },
      boxShadow: { sm: '0 1px 2px rgba(0,0,0,0.05)',
                   md: '0 4px 12px rgba(15,23,42,0.06)',
                   lg: '0 10px 24px rgba(15,23,42,0.10)' },
      transitionDuration: { fast: '100ms', base: '150ms', slow: '200ms' },
    },
  },
};
```

### 1.3. CSS variables (`web/src/app/globals.css`)

```css
:root {
  --bg-canvas: 248 250 252;  --bg-surface: 255 255 255;  --bg-elevated: 255 255 255;
  --bg-hover: 241 245 249;   --bg-selected: 238 242 255;
  --border: 226 232 240;     --border-strong: 203 213 225;
  --text-primary: 15 23 42;  --text-secondary: 71 85 105;
  --text-muted: 148 163 184; --text-inverse: 255 255 255;
  --brand: 99 102 241;       --brand-hover: 79 70 229;   --brand-subtle: 238 242 255;
  --ai: 139 92 246;          --ai-subtle: 245 243 255;
  --status-success: 16 185 129; --status-warning: 245 158 11;
  --status-danger: 239 68 68;   --status-info: 59 130 246;
}
.dark {
  --bg-canvas: 11 17 32;     --bg-surface: 17 24 39;     --bg-elevated: 30 41 59;
  --bg-hover: 31 41 55;      --bg-selected: 30 41 59;
  --border: 31 41 55;        --border-strong: 51 65 85;
  --text-primary: 241 245 249; --text-secondary: 148 163 184;
  --text-muted: 100 116 139; --text-inverse: 11 17 32;
  --brand: 129 140 248;      --brand-hover: 165 180 252; --brand-subtle: 30 41 59;
  --ai: 167 139 250;         --ai-subtle: 30 41 59;
  --status-success: 52 211 153; --status-warning: 251 191 36;
  --status-danger: 248 113 113; --status-info: 96 165 250;
}
```

## 2. Layout primitives

### 2.1. AppShell (required for every screen)

```tsx
export function AppShell({ children }: { children: React.ReactNode }) {
  return (
    <div className="grid h-screen grid-cols-[240px_1fr] grid-rows-[56px_1fr]">
      <header className="col-span-2 flex items-center gap-4 border-b border-border bg-bg-surface px-5">
        <Logo /><OrgSwitcher /><ProjectSwitcher /><GlobalSearch />
        <div className="ml-auto flex items-center gap-3">
          <NotificationsButton /><ThemeToggle /><UserMenu />
        </div>
      </header>
      <aside className="border-r border-border bg-bg-surface p-3"><MainNav /></aside>
      <main className="overflow-y-auto px-8 py-6">{children}</main>
    </div>
  );
}
```

### 2.2. PageHeader (at the top of every page)

```tsx
<PageHeader
  breadcrumb={[{ label: 'CRM Platform', href: '/projects/1' }, { label: 'Backlog' }]}
  title="Backlog"
  description="Tasks for this project"
  actions={<><Button variant="secondary">View: Board</Button><Button variant="primary">＋ New task</Button></>}
/>
```
Style: `padding: 20px 32px 16px; border-bottom: 1px solid var(--border); background: var(--bg-surface)`.

### 2.3. ContentGrid
`>= 1024px` → 2 columns; `< 1024px` → stack.

## 3. Component contracts

Every component implements the interface below and **all** listed variants.
Components not in §3 (Tabs, Kanban, forms, Toast, FileTree…) are in
[10-components.md](10-components.md) — same format, same tokens.

### 3.1. Button
```tsx
type ButtonProps = {
  variant: 'primary' | 'secondary' | 'ghost' | 'danger';
  size?: 'sm' | 'md' | 'lg';
  loading?: boolean; disabled?: boolean;
  iconLeft?: React.ReactNode; iconRight?: React.ReactNode;
  children: React.ReactNode; onClick?: () => void;
};
```
| variant | bg | text | border | hover |
|---|---|---|---|---|
| primary | `bg-brand` | `text-inverse` | none | `bg-brand-hover` |
| secondary | `bg-surface` | `text-primary` | `border-border` | `bg-bg-hover` |
| ghost | transparent | `text-secondary` | none | `bg-bg-hover` |
| danger | `bg-status-danger/10` | `text-status-danger` | `border-status-danger/30` | `bg-status-danger/20` |

Sizes: `sm h-8 px-3 text-xs gap-1.5` · `md h-9 px-4 text-sm gap-2` · `lg h-10 px-5 text-base gap-2`.
Loading: `iconLeft` → `<Spinner />`, `disabled=true`.

### 3.2. Badge / StatusBadge
```tsx
type BadgeStatus = 'draft'|'pending'|'planning'|'running'|'reviewing'
                 | 'passed'|'failed'|'merged'|'blocked'|'warning';
type BadgeProps = { status: BadgeStatus; label?: string; showDot?: boolean; pulse?: boolean };
```
| status | bg | text |
|---|---|---|
| draft | `bg-bg-hover` | `text-text-muted` |
| pending / warning | `bg-status-warning/10` | `text-status-warning` |
| planning / reviewing | `bg-ai-subtle` | `text-ai` |
| running | `bg-brand-subtle` | `text-brand` (pulse) |
| passed / merged | `bg-status-success/10` | `text-status-success` |
| failed / blocked | `bg-status-danger/10` | `text-status-danger` |

Format: `inline-flex items-center gap-1.5 px-2 py-0.5 rounded-md text-xs font-medium`.

### 3.3. Card
`default p-4` · `kpi p-4` (label/value/trend/sparkline) · `interactive p-4 cursor-pointer hover:shadow-md hover:border-border-strong`.
Base: `bg-bg-surface border border-border rounded-lg`.

### 3.4. KpiCard (required on Dashboard/Analytics)
```tsx
type KpiCardProps = {
  label: string; value: string | number; suffix?: string;
  trend?: { direction: 'up'|'down'; value: string; sentiment: 'good'|'bad'|'neutral' };
  sparkline?: number[];
  source: 'measured' | 'estimated' | 'n/a';   // REQUIRED
};
```
Layout: label 12px muted · value 26px semibold · trend 12px · sparkline · `DataSourceBadge` in the corner.

### 3.5. DataSourceBadge
`measured` → `text-status-success` + ✓ · `estimated` → `text-status-warning` + ≈ · `n/a` → `text-text-muted` + —.
Format: `inline-flex items-center gap-1 text-[10px] font-medium px-1.5 py-0.5 rounded bg-bg-hover`.

### 3.6. AgentCard
```tsx
type AgentType = 'planning'|'development'|'testing'|'review'|'documentation';
type AgentStatus = 'queued'|'running'|'awaiting_approval'|'failed'|'completed';
type AgentCardProps = {
  type: AgentType; status: AgentStatus; projectName: string; taskRef: string;
  progress?: number; model: string; elapsed: string; cost?: string;
  onStop?: () => void; onView?: () => void;
};
```
Type badge: planning `bg-ai-subtle text-ai` · development `bg-brand-subtle text-brand` ·
testing `bg-status-info/10 text-status-info` · review `bg-status-warning/10 text-status-warning` ·
documentation `bg-bg-hover text-text-secondary`.

### 3.7. PhaseList
`done` green ✓ · `active` indigo ◐ + pulse, text-primary medium · `pending` gray + number ·
`failed` red ✗.

### 3.8. LogViewer
`bg-[#0A0E1A] rounded-md p-3 font-mono text-xs leading-relaxed`; level tag colors:
READ `#60A5FA` · NEW `#34D399` · EDIT `#FBBF24` · RUN `#A78BFA` · ERROR `#F87171`;
timestamp muted, message primary; `autoScroll` default `true`, `maxHeight` 240.

### 3.9. DiffViewer
`+` `bg-status-success/10 text-status-success` · `−` `bg-status-danger/10 text-status-danger`;
`mode: 'unified' | 'split'`; approve/reject actions only when the PR gate is active.

### 3.10. DataTable
Header `bg-bg-hover text-xs font-semibold uppercase tracking-wide text-text-muted`;
row `border-b border-border hover:bg-bg-hover`; cell `px-4 py-3 text-sm`;
empty → `EmptyState` (never an empty `<div>`).

### 3.11. EmptyState / 3.12. LoadingSkeleton / 3.13. Modal-Drawer
- EmptyState: icon + title + description + CTA.
- Skeleton: `animate-pulse bg-bg-hover rounded`, variants `text|card|table|chart`.
- Modal: overlay `bg-black/40 backdrop-blur-sm`, panel `bg-bg-elevated rounded-xl shadow-lg`,
  `role="dialog" aria-modal="true"`, focus trap, Escape.
- Drawer: `fixed right-0 top-0 h-full w-[480px]`.

## 4. State machine

```
loading → empty | error | ready
```

| State | Display |
|---|---|
| loading | `LoadingSkeleton` of the appropriate variant |
| empty | `EmptyState` + CTA |
| error | `Alert variant="danger"` + `Button` "Retry" |
| ready | content |

## 5. Screen compositions

Declarative tree; the agent implements **exactly** this. All lists have the §4 states.
Visual reference: [`design/mockups/`](../design/mockups/) (15 screens + the first three in
[`design/previews/`](../design/previews/)). Mockups are not authoritative where they diverge
from this document — see "Known deviations" in `design/mockups/README.md`.

### 5.1. Dashboard (`/`)
```yaml
layout: AppShell > PageHeader > Stack(gap: 6)
PageHeader:
  title: "Good morning, {user.firstName}"
  description: "{pending} tasks awaiting review · {running} agents running · {failed} failed builds"
Grid(4): KpiCard(active projects, AI runs 24h, test success %, AI spend 30d)
Grid([1.4fr, 1fr]):
  Panel "Awaiting your approval" → ApprovalItem list
  Panel "AI spend this month" → SpendProgress
Section "Active AI executions" → AgentCard list
Panel "Recent activity" → ActivityItem list
```
The numbers in the description are links to filtered lists. `items.length === 0` → `EmptyState` **inside** the panel.

### 5.2. Projects (`/projects`)
PageHeader(actions: Import, ＋ New project) · FilterBar(search, status, tech, owner) ·
ViewToggle(grid default | table). Grid: `ProjectCard` + a "create" card with a dashed border.
Table: `[name, blueprint, techStack, tasksOpen, testRate, lastActivity, owner, status]`.

### 5.3. Project Overview (`/projects/:id`)
PageHeader(title, description: status · tech · blueprint vX) · Tabs `[Overview, Backlog, Agents, Tests, Repos, Settings]` ·
ContentGrid([2fr,1fr]): main `Product description`, `Recent changes`, `Backlog snapshot`; side `Active agents`, `Tests & builds`, `Configuration`.

### 5.4. Backlog (`/projects/:id/backlog`)
ViewToggle(list | board, default board) · FilterBar(search, assignee, priority, sprint).
Board columns: `todo` muted · `planning` ai · `in_progress` brand · `review` warning;
`minWidth 280, bg bg-hover, radius lg, padding 3`.
```tsx
type TaskCardProps = {
  id: string; title: string; priority: 'low'|'med'|'high'; tags: string[];
  assignee?: { name: string; avatar?: string };
  agentRun?: { type: AgentType; status: AgentStatus; progress: number };
  showProgress?: boolean;
};
```
`TaskDetailDrawer` width 560, right.

### 5.5. Agent Execution Details (`/agents/:runId`)
PageHeader(breadcrumb AI Agents → Run #{id}; actions "View diff", "■ Stop" if running) ·
StatsBar(4): Status badge · Model · Elapsed (mono) · Cost (mono).
Tabs `[Overview, Plan, Files & diff, Logs, Tests, Context]`.
- Overview: ContentGrid([2fr,1fr]) → `Execution phases` (PhaseList), `Live log` (LogViewer),
  `Created & modified files`; side: `Usage`, `Context used`, `Safety`.
- Files & diff: SplitPane(FileTree | DiffViewer).
- Context: `RuleCard { level, name, version, preview }` — **must** show the exact versions
  the run used (audit requirement, see [03-data-model.md](03-data-model.md)).

### 5.6. Templates (`/templates`)
Grid(3) BlueprintCard + `TemplateBuilderDialog` (5 steps: Basics, Technologies,
Design System & Components, Test Strategy, Rules & AI Context; footer Cancel/Back/Next-Finish).

### 5.7. Design System (`/design-system`)
Tabs `[Tokens, Components, Themes, Guidelines]`; Tokens: Color, Typography, Spacing, Radius, Shadow.

### 5.8. Components (`/components`)
Grid(4) `ComponentPreviewCard`; drawer: LivePreview (light/dark × desktop/mobile),
PropsTable, EventsSlots, UsageExamples, AiUsageNotes, VersionHistory, Accessibility.

### 5.9. Environments (`/environments`)
DataTable `[name, status, lastBuild, lastDeploy, version, actions]`; pending approval →
`Alert variant="warning"`; `ReleaseTimeline`.

### 5.10. Analytics (`/analytics`)
PageHeader(actions: RangePicker, ProjectFilter, Export) · Grid(3) 6× KpiCard ·
`Component & template usage` table · `Quality trend` LineChart.

### 5.11. Settings (`/settings/:section`)
`SettingsLayout(sidebar: SecondaryNav)` — secondary nav: members · ai_providers · git ·
rules · security · billing · notifications.

```yaml
members:
  body: DataTable(columns: [user, role, last_active, actions]) + Button "Invite"
  invite_modal: { email, role, message }
ai_providers:
  body: List(providers) + CapabilityMatrix(rows: model, cols: [streaming, tools, vision, context])
  rule: a secret is NEVER displayed after entry (only "•••• set 21.10.")
git:
  body: List(installations) + Button "Connect GitHub App"
rules:
  body: MarkdownEditor + VersionHistory + Button "Publish v{n}"
  rule: publish requires approval (rule_publish) at the org level
security:
  body: DataTable(audit) with filters [action, actor, from, to] + CSV export
billing / notifications:
  body: simple forms; no dedicated components
```

### 5.12. Login (`/login`)
```yaml
layout: no AppShell; centered card max-width: 400px
children:
  Logo
  Heading: "Welcome back" + sub: "Sign in to your account"
  Form: [Email, Password, Remember me | Forgot password?]
  Button: { variant: primary, label: "Sign in", fullWidth: true }
  Divider: "or continue with"
  SSO: [GitHub, Google]
  Footer: "New here? Create an account"
```
- Error (wrong credentials) → `Alert variant="danger"` above the form, focus on the first field.
- Loading: button `loading`, fields `disabled`.
- "Create account" goes to `/register` (self-serve: account + new org). Joining an existing org goes through an invite link, not through this screen. See [12-auth.md](12-auth.md).

### 5.13. Task Detail (drawer)
```yaml
component: Drawer(width: 560, side: right, open: !!selectedTaskId)
header: { ref: "T-88", title, close }
meta: [StatusBadge, PriorityBadge, Avatar+assignee]
sections:
  - Description
  - Acceptance criteria: checklist (checked/unchecked), read-only except when the task is a draft
  - Related: [requirement, PR link, linked tests]
  - AiPlanPanel:            # only when run.status = awaiting_approval && type = plan
      title: "AI plan"
      subtitle: "Generated by {AgentType} Agent · {n} steps"
      steps: ordered list
      actions: [Approve plan (primary), Edit (secondary), Reject (ghost)]
  - Activity: timeline (run events + comments)
footer_sticky: [Cancel, "▶ Run development agent" (primary)]
```
Rules:
- `AiPlanPanel` has `border-ai/40 bg-ai-subtle/40` — the only element with an AI accent in the drawer.
- "Run development agent" is `disabled` while the task is not `ready` or already has an active run;
  the tooltip explains why (never silently disabled).
- Every action (approve/reject/run) writes `audit_log` and `run_events` where applicable.
- The drawer opens from the board and from the list; the URL state is `?task=T-88` (shareable link).

### 5.14. Repositories (`/repositories`)
```yaml
PageHeader: { title: "Repositories", actions: [Button "Connect repository"] }
body:
  if repos.length === 0: EmptyState(title: "No repositories connected", action: "Connect GitHub")
  Grid(2): RepoCard { name, provider, defaultBranch, openPRs, lastSync, status }
  Panel: "Recent pull requests" → DataTable([#, title, task, checks, review, updated])
```
`RepoCard` with the `interactive` variant leads to the PR list; sync status has a `LiveIndicator`.

### 5.15. PR Detail (`/repositories/:prId`)
```yaml
PageHeader:
  breadcrumb: [{ repo }, { "#142" }]
  title: PR title
  status: StatusBadge(awaiting_review)
  description: "{head} → {base} · opened by {actor} · {time}"
  actions: [Button "View on GitHub" (secondary), Button "Merge" (primary, disabled while checks are not green)]
ChecksPanel: List([
  { name: "Build", state: passed, meta: "#234 · 2m 14s" },
  { name: "Unit tests", state: passed, meta: "184/184" },
  { name: "E2E tests", state: running, meta: "8/16" },
  { name: "Lint", state: passed }, { name: "Type check", state: passed },
  { name: "Review agent", state: passed, meta: "3 issues · view report" }
])
FilesChanged: List([{ status: modified|added|deleted, path, additions, deletions }])
DiffPreview: DiffViewer(selectedFile)
side:
  ReviewAgentPanel: { issues: [{ severity, text }], meta: "{time} · {cost}" }
  ReviewersPanel: [{ user, state: requested|approved|done }]
  LinkedPanel: [{ task }, { requirement }, { tests }]
```
Rules:
- "Merge" is a **human** action; it calls the GitHub merge API, then `task.done`. An agent
  cannot perform it (see [01-asdlc.md](01-asdlc.md#g3--pr-approval-human--agent-review)).
- While any required check is `pending|running`, the button is `disabled` with a reason.
- The "Review agent" panel shows `blocker` findings in red; they block the merge until they are
  resolved or explicitly waived (a waiver requires a reason and `admin`).

### 5.16. Empty states, modals, toast
Required catalog of states (every screen uses these primitives, not its own variants):

| State | Component | Content |
|---|---|---|
| no data | `EmptyState` | icon, title, description, primary CTA |
| no search results | `EmptyState` | "No results for '{q}'", CTA "Clear filters" |
| loading | `LoadingSkeleton` | variant `text|card|table|chart` |
| error | `Alert variant="danger"` | title, description, "Retry" |
| confirmation (non-destructive) | `Modal size=sm` | consequences of the action, Cancel + primary |
| destructive | `Modal` | red button, description of irreversibility, name entry if needed |
| success / error / warning / info | `Toast` | `role="status"` (info) / `role="alert"` (error), auto-dismiss 5s, max 3 stacked |

Toast position: `bottom-right`, `shadow-lg`, `rounded-lg`, color only on the icon and the left
edge (`border-l-2`), text always `text-primary` (readability).

## 6. Responsive

| Breakpoint | Behavior |
|---|---|
| `< 640px` | sidebar → hamburger; grids → 1 column; **DataTable → card list** (not horizontal scroll) |
| `640–1024px` | sidebar collapsed 64px (icons); ContentGrid → 1 column; KPI grid → 2 |
| `>= 1024px` | full layout |
| `>= 1440px` | `max-width: 1440px`, centered |

## 7. Accessibility

- Every interactive element has an `aria-label` or visible text.
- Focus: `focus-visible:ring-2 focus-visible:ring-brand focus-visible:ring-offset-2`.
- Contrast ≥ WCAG AA (4.5:1 text, 3:1 UI).
- Modal: `role="dialog" aria-modal="true"`, focus trap.
- Status badge never color alone — always text or an icon (color-blind safe).
- Tab order follows the visual order; Escape closes overlays.

## 8. Anti-patterns

| ❌ Don't | ✅ Do |
|---|---|
| `<div style={{color:'#6366F1'}}>` | `className="text-brand"` |
| estimated value without a label | `<KpiCard source="estimated" />` |
| empty `<div>` when there is no data | `<EmptyState />` |
| `shadow-lg` on a card | `border border-border` |
| 500ms animation | `transition duration-150` |
| `EventSource` without auth | `fetch` + stream (see [api/events.md](../api/events.md)) |

## 9. Checklist before "done" for a screen

- [ ] Tokens only from §1 (grep for hex colors in `web/src/components` returns 0)
- [ ] light + dark verified
- [ ] loading / empty / error implemented
- [ ] 375px / 768px / 1280px screenshot in the PR
- [ ] keyboard: all actions reachable via Tab/Enter/Escape
- [ ] no `console.log`, no `any`
- [ ] the component has a test (render + interaction)

## 10. Verification

```bash
pnpm -C web lint && pnpm -C web tsc --noEmit && pnpm -C web test
grep -rE '#[0-9A-Fa-f]{6}' web/src/components && echo "FAIL: hardcoded color" || true
```
