# UI Redesign Handoff

This package is based on the supplied `aip-source-20261025.zip` source project and the 19 screenshots in `aip-ui-screenshots-20261025.zip`.

## Scope
- Updated shared application shell: workspace switcher, grouped sidebar navigation, breadcrumbs, global search, notifications, account menu, theme toggle and responsive navigation.
- Updated authentication presentation for login and registration.
- Refined shared design tokens and cross-page presentation: spacing, type hierarchy, cards, form focus states, tables, motion and reduced-motion support.
- Preserved existing page components, API calls, routes and business logic wherever possible.

## Screens reviewed as references
Login (light/dark), registration, offline, overview, projects, project board, search, agents, repositories, blueprints, design system, components, analytics, settings, run detail, pull-request detail, dark overview and dark project.

## Validation
Run from `apps/web`:
- `pnpm install --frozen-lockfile`
- `pnpm tsc`
- `pnpm lint`
- `pnpm test`
- `pnpm build`

These checks must be run in an environment with package registry access. Do not consider the redesign verified until the checks pass and the primary user flows are manually exercised.

## Focused UX pass (v3)

### Overview dashboard
- Replaced the onboarding-first layout with a workspace dashboard that surfaces real project/run metrics, project shortcuts, a setup checklist and the human-in-the-loop workflow.
- Values are sourced from the existing API hooks. Cost is only shown as a currency value when the API marks it as measured; otherwise the UI says it is unavailable rather than inventing a number.
- Empty and loading states remain explicit; project rows navigate to the existing project route.

### Project task board
- Reworked board columns into a horizontally scrollable, touch-friendlier layout with clearer stage counts, card metadata and empty states.
- Kept the existing valid-transition API mutation, optimistic update, blocked-reason prompt and query invalidation behavior.

### AI execution details
- Added an at-a-glance summary for status, agent/model, recorded usage and approval gate.
- Added a stage tracker derived from recorded run status/events. It is a lightweight activity summary, not a claim that the backend exposes a fully structured workflow timeline.
- Kept the existing plan approval, review findings, diff, artifacts, usage, context snapshot and live log sections.

### Verification
- The working environment did not have frontend dependencies installed, so a production build and test run could not be executed offline. Install dependencies using the repository's pnpm instructions, then run `pnpm --dir apps/web tsc --noEmit`, `pnpm --dir apps/web test` and `pnpm --dir apps/web build`.


## Workspace-wide polish (v4)

- Added consistent page headers, metric panels, empty states, focus-friendly controls and responsive layout behavior across Projects, Project details, Agents, Analytics, Search, Repositories, Blueprints, Design System, Components and Settings.
- Projects now include local search and status filtering, summary counts, clearer cards and a no-match state.
- Project details surface task counts by delivery stage before the task list/board. The existing task mutations and agent-run entry points remain in place.
- Agent runs include contextual summary counts and a retry affordance on query failure. Analytics, search and repositories use the same visual language as the overview.
- Blueprint creation remains a five-step wizard; saved blueprints are displayed as responsive cards with the existing version/publish/instantiate actions preserved.
- Responsive rules include small-screen stacking and reduced-motion handling. No fabricated backend metrics or sample production data were introduced.

## Verification status (v4)

The project was updated in the working source tree and packaged. Dependency installation could not be performed because the environment could not reach the npm registry; therefore TypeScript, lint, unit tests and production build remain unverified. Run the repository's pnpm commands in a network-enabled environment before merging.
