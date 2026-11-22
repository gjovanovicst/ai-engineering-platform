import Link from "next/link";

import contracts from "@/design/contracts.json";

export const metadata = { title: "Playground · Design system" };

export default function PlaygroundIndex() {
  return (
    <div className="page-stack">
      <header className="page-hero mb-6">
        <h1 className="text-[22px] font-semibold">Component playground</h1>
        <p className="mt-1 text-[13px] text-text-secondary">
          Every primitive with its contract: the variants, the props, the states and the a11y notes.
          The source is <code className="font-mono text-xs">design/contracts.json</code>.
        </p>
      </header>
      <ul className="grid gap-2 sm:grid-cols-2">
        {contracts.components.map((component) => (
          <li key={component.name}>
            <Link
              href={`/design-system/playground/${component.name}`}
              className="block rounded-lg border border-border bg-bg-surface px-3 py-2 hover:bg-bg-hover"
            >
              <span className="text-sm font-medium">{component.name}</span>
              <span className="mt-0.5 block text-xs text-text-secondary">{component.summary}</span>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}
