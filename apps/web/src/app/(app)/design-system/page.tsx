"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { DesignStandardsPanel } from "@/components/design-standards-panel";
import { Tabs } from "@/components/ui/tabs";
import { TokenRegistry } from "@/components/token-registry";
import { colorGroups, layout, typeScale } from "@/design/registry";
import { ensureSession } from "@/lib/api";
import type { Claims } from "@/lib/auth";

export default function DesignSystemPage() {
  const router = useRouter();
  const [claims, setClaims] = useState<Claims | null>(null);
  const [tab, setTab] = useState<"standards" | "tokens" | "baseline">("standards");

  useEffect(() => {
    ensureSession().then((result) => {
      if (!result) {
        router.replace("/login");
        return;
      }
      setClaims(result);
    });
  }, [router]);

  return (
    <div className="page-stack">
      <header className="page-hero mb-6">
        <h1 className="text-[22px] font-semibold">Design standards</h1>
        <p className="mt-1 text-[13px] text-text-secondary">
          Versioned standards projects inherit, plus your organisation token registry.{" "}
          <Link href="/components" className="text-brand hover:underline">
            Components
          </Link>
          {" · "}
          <Link href="/design-system/playground" className="text-brand hover:underline">
            Playground
          </Link>
        </p>
      </header>

      <Tabs
        ariaLabel="Design system sections"
        value={tab}
        onChange={(id) => setTab(id as typeof tab)}
        items={[
          { id: "standards", label: "Standards" },
          { id: "tokens", label: "Tokens" },
          { id: "baseline", label: "Baseline" },
        ]}
      />

      {claims && tab === "standards" ? <DesignStandardsPanel orgID={claims.org} /> : null}
      {claims && tab === "tokens" ? <TokenRegistry orgID={claims.org} /> : null}

      {tab === "baseline" ? (<>
      <section className="mb-8">
        <h2 className="mb-3 text-sm font-semibold">Built-in colour baseline</h2>
        <div className="space-y-5">
          {colorGroups.map((group) => (
            <div key={group.label}>
              <h3 className="mb-2 text-xs font-semibold uppercase text-text-muted">{group.label}</h3>
              <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
                {group.swatches.map((swatch) => (
                  <div key={swatch.name} className="rounded-lg border border-border bg-bg-surface p-3">
                    <div className={`h-10 w-full rounded-md ${swatch.className}`} />
                    <p className="mt-2 font-mono text-[11px] text-text-secondary">{swatch.name}</p>
                    <p className="font-mono text-[11px] text-text-muted">{swatch.value}</p>
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      </section>

      <section className="mb-8">
        <h2 className="mb-3 text-sm font-semibold">Typography baseline</h2>
        <ul className="divide-y divide-border overflow-hidden rounded-lg border border-border bg-bg-surface">
          {typeScale.map((item) => (
            <li key={item.name} className="flex flex-wrap items-baseline gap-3 px-4 py-3">
              <span className={`flex-1 ${item.className}`}>{item.sample}</span>
              <code className="font-mono text-[11px] text-text-muted">{item.name}</code>
              <span className="w-28 text-xs text-text-secondary">{item.usage}</span>
            </li>
          ))}
        </ul>
      </section>

      <section>
        <h2 className="mb-3 text-sm font-semibold">Layout baseline</h2>
        <dl className="grid gap-3 sm:grid-cols-3">
          {Object.entries(layout).map(([name, value]) => (
            <div key={name} className="rounded-lg border border-border bg-bg-surface p-3">
              <dt className="text-xs text-text-muted">{name}</dt>
              <dd className="mt-1 font-mono text-sm">{value}</dd>
            </div>
          ))}
        </dl>
      </section>
      </>) : null}
    </div>
  );
}
