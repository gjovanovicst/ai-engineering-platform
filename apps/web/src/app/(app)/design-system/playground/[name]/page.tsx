"use client";

import Link from "next/link";
import { notFound, useParams } from "next/navigation";

import contracts from "@/design/contracts.json";
import { playgroundExamples } from "@/design/playground";

export default function ComponentPlayground() {
  const { name } = useParams<{ name: string }>();
  const component = contracts.components.find((entry) => entry.name === name);
  const example = playgroundExamples[name ?? ""];

  if (!component) notFound();

  return (
    <div className="page-stack">
      <header className="mb-6">
        <Link href="/design-system/playground" className="text-xs text-brand hover:underline">
          ← Playground
        </Link>
        <h1 className="mt-1 text-[22px] font-semibold">{component.name}</h1>
        <p className="mt-1 text-[13px] text-text-secondary">{component.summary}</p>
        <p className="mt-1 font-mono text-xs text-text-muted">{component.file}</p>
      </header>

      <section className="mb-8">
        <h2 className="mb-3 text-sm font-semibold">Example</h2>
        <div className="rounded-lg border border-border bg-bg-surface p-5">
          {example ? example() : (
            <p className="text-xs text-text-muted">No example registered for this component.</p>
          )}
        </div>
      </section>

      <section className="mb-8">
        <h2 className="mb-3 text-sm font-semibold">Props</h2>
        <div className="overflow-x-auto rounded-lg border border-border">
          <table className="w-full text-left text-sm">
            <thead className="bg-bg-subtle text-xs text-text-secondary">
              <tr>
                <th className="px-3 py-2">Name</th>
                <th className="px-3 py-2">Type</th>
                <th className="px-3 py-2">Required</th>
                <th className="px-3 py-2">Description</th>
              </tr>
            </thead>
            <tbody>
              {component.props.map((prop) => (
                <tr key={prop.name} className="border-t border-border">
                  <td className="px-3 py-2 font-mono text-xs">{prop.name}</td>
                  <td className="px-3 py-2 font-mono text-xs text-text-secondary">{prop.type}</td>
                  <td className="px-3 py-2 text-xs">{prop.required ? "yes" : "—"}</td>
                  <td className="px-3 py-2 text-xs text-text-secondary">{prop.description}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      {component.states.length > 0 ? (
        <section className="mb-8">
          <h2 className="mb-3 text-sm font-semibold">States</h2>
          <ul className="flex flex-wrap gap-1.5">
            {component.states.map((state) => (
              <li key={state} className="rounded-full border border-border bg-bg-subtle px-2 py-0.5 text-xs text-text-secondary">
                {state}
              </li>
            ))}
          </ul>
        </section>
      ) : null}

      <section>
        <h2 className="mb-3 text-sm font-semibold">Accessibility</h2>
        <ul className="list-disc space-y-1 pl-5 text-sm text-text-secondary">
          {component.a11y.map((note) => <li key={note}>{note}</li>)}
        </ul>
      </section>
    </div>
  );
}
