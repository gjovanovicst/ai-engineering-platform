"use client";

import { SidePanel } from "@/components/ui/side-panel";
import { glossary } from "@/lib/glossary";

const shortcuts = [
  { keys: "⌘ K", action: "Search tasks and projects" },
  { keys: "?", action: "Open this help panel" },
  { keys: "Esc", action: "Close a panel or dialog" },
  { keys: "Enter", action: "Submit the focused form" },
];

const docs = [
  { label: "Quickstart", href: "https://github.com/gjovanovicst/ai-engineering-platform/blob/main/docs/user/01-quickstart.md" },
  { label: "Tasks and the board", href: "https://github.com/gjovanovicst/ai-engineering-platform/blob/main/docs/user/04-tasks-and-board.md" },
  { label: "Runs and approvals", href: "https://github.com/gjovanovicst/ai-engineering-platform/blob/main/docs/user/05-runs-and-approvals.md" },
  { label: "Pull requests", href: "https://github.com/gjovanovicst/ai-engineering-platform/blob/main/docs/user/06-pull-requests.md" },
  { label: "Troubleshooting", href: "https://github.com/gjovanovicst/ai-engineering-platform/blob/main/docs/user/13-troubleshooting.md" },
];

/** Docs, shortcuts and the glossary of the terms the UI uses. */
export function HelpPanel({ onClose }: { onClose: () => void }) {
  return (
    <SidePanel
      title="Help"
      description="The docs, the keyboard shortcuts and the terms this workspace uses."
      onClose={onClose}
      footer={<span className="text-xs text-text-muted">Every screen has a next action; the help tips explain the jargon.</span>}
    >
      <div className="space-y-6 p-5 text-sm">
        <section>
          <h3 className="text-xs font-semibold uppercase tracking-wider text-text-muted">Documentation</h3>
          <ul className="mt-2 space-y-1.5">
            {docs.map((doc) => (
              <li key={doc.href}>
                <a href={doc.href} target="_blank" rel="noreferrer" className="text-brand hover:underline">
                  {doc.label}
                </a>
              </li>
            ))}
          </ul>
        </section>
        <section>
          <h3 className="text-xs font-semibold uppercase tracking-wider text-text-muted">Keyboard shortcuts</h3>
          <ul className="mt-2 space-y-1.5">
            {shortcuts.map((shortcut) => (
              <li key={shortcut.keys} className="flex items-center justify-between gap-3 text-xs">
                <span className="text-text-secondary">{shortcut.action}</span>
                <kbd className="rounded border border-border bg-bg-subtle px-1.5 py-0.5 font-mono text-[11px]">{shortcut.keys}</kbd>
              </li>
            ))}
          </ul>
        </section>
        <section>
          <h3 className="text-xs font-semibold uppercase tracking-wider text-text-muted">Glossary</h3>
          <dl className="mt-2 space-y-2">
            {Object.values(glossary).map((entry) => (
              <div key={entry.term} className="text-xs">
                <dt className="font-semibold text-text-primary">{entry.term}</dt>
                <dd className="text-text-secondary">{entry.definition}</dd>
              </div>
            ))}
          </dl>
        </section>
      </div>
    </SidePanel>
  );
}
