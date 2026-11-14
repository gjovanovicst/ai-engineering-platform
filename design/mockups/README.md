# Design mockups (reference, non-binding)

HTML mockups from the design conversation, rendered to PNG for quick review. They serve as a
**visual reference** for the implementation; [design system](../../docs/06-design-system.md) is authoritative.

- `all-mockups.html` — an interactive file with 15 screens (tabs at the top). Open it in a browser.
- `screens/*.png` — one screenshot per screen (1440×900 @2x).

## The 15 new screens

| # | Screen | File | Docs |
|---|---|---|---|
| 1 | Projects list | `screens/1-projects.png` | [§5.2](../../docs/06-design-system.md#52-projects-projects) |
| 2 | Project Overview | `screens/2-project-overview.png` | [§5.3](../../docs/06-design-system.md#53-project-overview-projectsid) |
| 3 | Task Detail drawer | `screens/3-task-detail.png` | [§5.13](../../docs/06-design-system.md#513-task-detail-drawer) |
| 4 | Templates list | `screens/4-templates.png` | [§5.6](../../docs/06-design-system.md#56-templates-templates) |
| 5 | Template Builder (5 steps) | `screens/5-template-builder.png` | [§5.6](../../docs/06-design-system.md#56-templates-templates) |
| 6 | Design System (tokens) | `screens/6-design-system.png` | [§5.7](../../docs/06-design-system.md#57-design-system-design-system) |
| 7 | Components registry | `screens/7-components.png` | [§5.8](../../docs/06-design-system.md#58-components-components) |
| 8 | Component detail | `screens/8-component-detail.png` | [§5.8](../../docs/06-design-system.md#58-components-components) |
| 9 | Repositories | `screens/9-repositories.png` | [§5.14](../../docs/06-design-system.md#514-repositories-repositories) |
| 10 | PR detail + checks | `screens/10-pr-detail.png` | [§5.15](../../docs/06-design-system.md#515-pr-detail-repositoriesprid) |
| 11 | Environments | `screens/11-environments.png` | [§5.9](../../docs/06-design-system.md#59-environments-environments) |
| 12 | Analytics | `screens/12-analytics.png` | [§5.10](../../docs/06-design-system.md#510-analytics-analytics) |
| 13 | Settings (Members) | `screens/13-settings.png` | [§5.11](../../docs/06-design-system.md#511-settings-settingssection) |
| 14 | Login | `screens/14-login.png` | [§5.12](../../docs/06-design-system.md#512-login-login) |
| 15 | Empty states / modals / toast | `screens/15-states.png` | [§5.16](../../docs/06-design-system.md#516-empty-states-modals-toast) |

The first three screens (Dashboard, Agent Execution, Kanban) are in [../previews/](../previews/).

## Known deviations in the mockups (do not copy 1:1)

1. **Icons are emoji** (`📋`, `🔍`, `⚙`); in the implementation use **Lucide** icons only,
   stroke 1.5, 16px in content / 20px in the sidebar.
2. **Colors are inline** in a few places (`#FEF2F2`, `#991B1B` in the error state); they must be
   replaced with tokens (`status-danger/10`, `text-status-danger`).
3. **The "Merge" button** in the PR detail is a human action (it calls the GitHub merge); the agent
   never performs it on its own. See [ASDLC §5](../../docs/01-asdlc.md#g3--pr-approval-human--agent-review).
4. The animations in the mockup (`pulse`) are illustrative; in code it is `animate-pulse` from Tailwind.
5. Numbers and names are fictional (Acme, CRM Platform); use the seed from
   [testing §9](../../docs/07-testing.md#9-test-data).
