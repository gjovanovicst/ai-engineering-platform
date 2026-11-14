"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { ComponentRegistry } from "@/components/component-registry";
import { Tabs } from "@/components/ui/tabs";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { ensureSession } from "@/lib/api";
import type { Claims } from "@/lib/auth";

export default function ComponentsPage() {
  const router = useRouter();
  const [claims, setClaims] = useState<Claims | null>(null);
  const [tab, setTab] = useState<"registry" | "primitives">("registry");

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
        <h1 className="text-[22px] font-semibold">Component registry</h1>
        <p className="mt-1 text-[13px] text-text-secondary">
          Approved, deprecated and forbidden components the projects and agents may use.
        </p>
      </header>

      <Tabs
        ariaLabel="Component sections"
        value={tab}
        onChange={(id) => setTab(id as typeof tab)}
        items={[
          { id: "registry", label: "Registry" },
          { id: "primitives", label: "Primitives" },
        ]}
      />

      {claims && tab === "registry" ? <ComponentRegistry orgID={claims.org} /> : null}

      {tab === "primitives" ? (<>
      <section className="rounded-lg border border-border bg-bg-surface p-4">
        <h2 className="mb-3 text-sm font-semibold">Primitives</h2>
        <div className="flex flex-wrap items-center gap-3">
          <Button>Primary</Button>
          <Button variant="secondary">Secondary</Button>
          <Button variant="ghost">Ghost</Button>
          <Button variant="danger">Danger</Button>
          <Button disabled>Disabled</Button>
          <Button loading>Loading</Button>
        </div>
      </section>

      <section className="rounded-lg border border-border bg-bg-surface p-4">
        <h2 className="mb-3 text-sm font-semibold">Field and Input</h2>
        <div className="grid gap-4 md:grid-cols-2">
          <Field label="Email" htmlFor="demo-email">
            <Input id="demo-email" placeholder="dev@acme.io" />
          </Field>
          <Field label="With error" htmlFor="demo-error" error="A reason is required">
            <Input id="demo-error" aria-invalid />
          </Field>
          <Field label="Disabled" htmlFor="demo-disabled">
            <Input id="demo-disabled" disabled value="read only" />
          </Field>
        </div>
      </section>
      </>) : null}
    </div>
  );
}
