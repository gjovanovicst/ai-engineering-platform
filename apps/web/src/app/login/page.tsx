"use client";

import { useRouter } from "next/navigation";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { BrandMark } from "@/components/brand-mark";
import { API_URL, ApiError, login } from "@/lib/api";
import { useZodForm } from "@/lib/form";
import { loginSchema, type LoginValues } from "@/lib/schemas";

export default function LoginPage() {
  const router = useRouter();
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useZodForm<LoginValues>(loginSchema, { defaultValues: { email: "", password: "" } });
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const onSubmit = handleSubmit(async (values) => {
    setError(null);
    setBusy(true);
    try {
      await login(values.email, values.password);
      router.replace("/overview");
    } catch (err) {
      setError(err instanceof ApiError ? err.message : "Sign-in failed");
    } finally {
      setBusy(false);
    }
  });

  return (
    <main className="auth-page">
      <section className="auth-showcase">
        <a href="/login" className="auth-brand">
          <BrandMark className="auth-brand-mark" />
          <span><strong>AI Engineering</strong><small>PLATFORM</small></span>
        </a>
        <div className="auth-showcase-copy">
          <span className="auth-eyebrow"><span /> THE NEXT WAY TO BUILD SOFTWARE</span>
          <h1>From idea to<br /><em>production.</em><br />With AI in the loop.</h1>
          <p>One connected workspace for your engineering process — from project blueprints to tested, reviewed and ready-to-ship code.</p>
          <div className="auth-benefits">
            <div><span>✓</span><strong>Consistent by design</strong><small>Reusable blueprints, rules and components.</small></div>
            <div><span>✓</span><strong>AI with guardrails</strong><small>Human approvals and an auditable trail.</small></div>
            <div><span>✓</span><strong>Quality built in</strong><small>Tests and reviews at every important step.</small></div>
          </div>
        </div>
        <div className="auth-showcase-footer"><span>● Secure workspace</span><span>● Human-controlled AI</span><span>● Built for engineering teams</span></div>
        <div className="auth-orb auth-orb-one" /><div className="auth-orb auth-orb-two" />
      </section>
      <section className="auth-panel">
        <div className="auth-mobile-brand"><BrandMark className="auth-brand-mark" /><strong>AI Engineering Platform</strong></div>
        <div className="auth-form-wrap">
          <div className="auth-form-heading">
            <span className="auth-form-icon" aria-hidden="true">↗</span>
            <h2>Welcome back</h2>
            <p>Sign in to your engineering workspace.</p>
          </div>
          {error ? <div role="alert" className="mb-4 rounded-lg border border-status-danger/30 bg-status-danger/10 px-3 py-2.5 text-sm text-status-danger">{error}</div> : null}
          <form onSubmit={onSubmit} className="auth-form">
            <Field label="Work email" htmlFor="email" error={errors.email?.message}>
              <Input id="email" type="email" autoComplete="email" aria-invalid={Boolean(errors.email)} {...register("email")} placeholder="you@company.com" />
            </Field>
            <Field label="Password" htmlFor="password" error={errors.password?.message}>
              <Input id="password" type="password" autoComplete="current-password" aria-invalid={Boolean(errors.password)} {...register("password")} placeholder="Enter your password" />
            </Field>
            <div className="auth-form-meta"><label><input type="checkbox" /> Remember me</label><a href="/register">Need access?</a></div>
            <Button type="submit" loading={busy} className="w-full auth-submit">Sign in to workspace <span aria-hidden="true">→</span></Button>
          </form>
          {process.env.NEXT_PUBLIC_OIDC_PROVIDERS ? (
            <div className="auth-providers"><div className="auth-divider"><span>or continue with</span></div>
              {process.env.NEXT_PUBLIC_OIDC_PROVIDERS.split(",").map((provider) => <a key={provider} href={`${API_URL}/api/v1/auth/oidc/${provider.trim()}/start`}>Continue with {provider.trim()}</a>)}
            </div>
          ) : null}
          <p className="auth-register">New to the platform? <a href="/register">Create an account</a></p>
          <p className="auth-privacy">Protected by your organization’s access policies.</p>
        </div>
      </section>
    </main>
  );
}
