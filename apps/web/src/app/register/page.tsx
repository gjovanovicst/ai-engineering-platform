"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { BrandMark } from "@/components/brand-mark";
import { ApiError, login, register } from "@/lib/api";

export default function RegisterPage() {
  const router = useRouter();
  const [form, setForm] = useState({ name: "", email: "", org_name: "", password: "" });
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  function update(key: keyof typeof form) {
    return (event: React.ChangeEvent<HTMLInputElement>) =>
      setForm((current) => ({ ...current, [key]: event.target.value }));
  }

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault();
    setError(null);
    setBusy(true);
    try {
      await register(form);
      await login(form.email, form.password);
      router.replace("/projects");
    } catch (err) {
      setError(
        err instanceof ApiError
          ? err.status === 400
            ? err.message
            : "Could not create the account. If the email exists, sign in instead."
          : "Registration failed",
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <main className="auth-page">
      <section className="auth-showcase">
        <a href="/login" className="auth-brand">
          <BrandMark className="auth-brand-mark" />
          <span><strong>AI Engineering</strong><small>PLATFORM</small></span>
        </a>
        <div className="auth-showcase-copy">
          <span className="auth-eyebrow"><span /> YOUR ENGINEERING WORKSPACE</span>
          <h1>Build with<br /><em>consistency.</em><br />Ship with confidence.</h1>
          <p>Bring your projects, AI agents, engineering standards and quality gates into one connected workspace.</p>
          <div className="auth-benefits">
            <div><span>✓</span><strong>Start with a blueprint</strong><small>Build on proven patterns instead of starting from scratch.</small></div>
            <div><span>✓</span><strong>Keep your team in control</strong><small>Human approval stays in the loop.</small></div>
            <div><span>✓</span><strong>Measure what matters</strong><small>Understand quality, throughput and AI usage.</small></div>
          </div>
        </div>
        <div className="auth-showcase-footer"><span>● Secure workspace</span><span>● Human-controlled AI</span><span>● Built for engineering teams</span></div>
        <div className="auth-orb auth-orb-one" /><div className="auth-orb auth-orb-two" />
      </section>
      <section className="auth-panel">
        <div className="auth-mobile-brand"><BrandMark className="auth-brand-mark" /><strong>AI Engineering Platform</strong></div>
        <div className="auth-form-wrap">
          <div className="auth-form-heading">
            <span className="auth-form-icon" aria-hidden="true">✦</span>
            <h2>Create your workspace</h2>
            <p>Set up your account and invite your team later.</p>
          </div>
          {error ? <div role="alert" className="mb-4 rounded-lg border border-status-danger/30 bg-status-danger/10 px-3 py-2.5 text-sm text-status-danger">{error}</div> : null}
          <form onSubmit={onSubmit} className="auth-form">
            <Field label="Your name" htmlFor="name"><Input id="name" autoComplete="name" required value={form.name} onChange={update("name")} placeholder="Jane Smith" /></Field>
            <Field label="Work email" htmlFor="email"><Input id="email" type="email" autoComplete="email" required value={form.email} onChange={update("email")} placeholder="you@company.com" /></Field>
            <Field label="Workspace name" htmlFor="org_name"><Input id="org_name" value={form.org_name} onChange={update("org_name")} placeholder="Acme Engineering" /></Field>
            <Field label="Password" htmlFor="password"><Input id="password" type="password" autoComplete="new-password" required minLength={12} value={form.password} onChange={update("password")} placeholder="At least 12 characters" /></Field>
            <Button type="submit" loading={busy} className="w-full auth-submit">Create account <span aria-hidden="true">→</span></Button>
          </form>
          <p className="auth-register">Already have an account? <Link href="/login">Sign in</Link></p>
          <p className="auth-privacy">Protected by your organization’s access policies.</p>
        </div>
      </section>
    </main>
  );
}
