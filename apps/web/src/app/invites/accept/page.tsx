"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { ApiError, acceptInvite, acceptInvitePublic, ensureSession } from "@/lib/api";
import { useZodForm } from "@/lib/form";
import { acceptInviteSchema, type AcceptInviteValues } from "@/lib/schemas";
import { setSession } from "@/lib/session";

type State = "checking" | "has-session" | "no-session" | "done" | "error";

function AcceptInvite() {
  const params = useSearchParams();
  const orgID = params.get("org") ?? "";
  const token = params.get("token") ?? "";
  const [state, setState] = useState<State>("checking");
  const [message, setMessage] = useState("");
  const {
    register,
    handleSubmit,
    formState: { errors },
  } = useZodForm<AcceptInviteValues>(acceptInviteSchema, { defaultValues: { name: "", password: "" } });
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    ensureSession().then((result) => setState(result ? "has-session" : "no-session"));
  }, []);

  async function acceptExisting() {
    setBusy(true);
    try {
      const result = await acceptInvite(orgID, token);
      setMessage(`You joined the workspace as ${result.role}.`);
      setState("done");
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Could not accept the invite");
      setState("error");
    } finally {
      setBusy(false);
    }
  }

  const registerAndAccept = handleSubmit(async (values) => {
    setBusy(true);
    try {
      const pair = await acceptInvitePublic(token, values.name, values.password);
      setSession(pair);
      setMessage("Your account is ready and you joined the workspace.");
      setState("done");
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : "Could not accept the invite");
      setState("error");
    } finally {
      setBusy(false);
    }
  });

  return (
    <main className="mx-auto flex min-h-screen max-w-md flex-col justify-center p-6">
      <h1 className="text-xl font-semibold">Workspace invitation</h1>

      {state === "checking" ? <p className="mt-3 text-sm text-text-secondary">Checking your session…</p> : null}

      {state === "has-session" ? (
        <>
          <p className="mt-3 text-sm text-text-secondary">
            You are signed in. Accepting adds you to the workspace with the invited role.
          </p>
          <Button className="mt-4" loading={busy} onClick={acceptExisting}>
            Accept invitation
          </Button>
        </>
      ) : null}

      {state === "no-session" ? (
        <form className="mt-4 space-y-3" onSubmit={registerAndAccept}>
          <p className="text-sm text-text-secondary">
            Create an account with the invited email to join the workspace.
          </p>
          <Field label="Name" htmlFor="invite-name" error={errors.name?.message}>
            <Input id="invite-name" aria-invalid={Boolean(errors.name)} {...register("name")} placeholder="Jane Smith" />
          </Field>
          <Field label="Password" htmlFor="invite-password" error={errors.password?.message}>
            <Input
              id="invite-password"
              type="password"
              autoComplete="new-password"
              aria-invalid={Boolean(errors.password)}
              {...register("password")}
              placeholder="At least 12 characters"
            />
          </Field>
          <Button type="submit" loading={busy}>
            Create account and join
          </Button>
        </form>
      ) : null}

      {state === "done" ? (
        <>
          <p className="mt-3 text-sm text-status-success">{message}</p>
          <Link href="/projects" className="mt-4 text-sm text-brand hover:underline">
            Go to projects →
          </Link>
        </>
      ) : null}

      {state === "error" ? (
        <>
          <p role="alert" className="mt-3 text-sm text-status-danger">
            {message}
          </p>
          <button
            type="button"
            onClick={() => setState("no-session")}
            className="mt-4 text-sm text-brand hover:underline"
          >
            Try with a new account
          </button>
        </>
      ) : null}
    </main>
  );
}

export default function AcceptInvitePage() {
  return (
    <Suspense fallback={<main className="p-6 text-sm text-text-secondary">Loading…</main>}>
      <AcceptInvite />
    </Suspense>
  );
}
