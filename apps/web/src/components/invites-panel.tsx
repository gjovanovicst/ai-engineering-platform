"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { SidePanel } from "@/components/ui/side-panel";
import { ApiError, createInvite, listInvites } from "@/lib/api";
import { useZodForm } from "@/lib/form";
import { inviteSchema, type InviteValues } from "@/lib/schemas";

const roles = ["admin", "designer", "member", "viewer"];

export function InvitesPanel({ orgID }: { orgID: string }) {
  const queryClient = useQueryClient();
  const {
    register,
    handleSubmit,
    reset,
    formState: { errors },
  } = useZodForm<InviteValues>(inviteSchema, { defaultValues: { email: "", role: "member" } });
  const [showForm, setShowForm] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [link, setLink] = useState<string | null>(null);

  const invites = useQuery({ queryKey: ["invites", orgID], queryFn: () => listInvites(orgID) });

  const create = useMutation({
    mutationFn: (input: InviteValues) => createInvite(orgID, input.email, input.role),
    onSuccess: (invite) => {
      const origin = typeof window === "undefined" ? "" : window.location.origin;
      setLink(`${origin}/invites/accept?org=${orgID}&token=${invite.token}`);
      reset({ email: "" });
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["invites", orgID] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not create the invite"),
  });

  return (
    <section className="rounded-lg border border-border bg-bg-surface">
      <header className="flex items-start justify-between gap-3 border-b border-border p-4">
        <div>
          <h2 className="text-sm font-semibold">Team invites</h2>
          <p className="mt-1 text-xs text-text-secondary">
            Invite a teammate by email. The link expires in 7 days.
          </p>
        </div>
        <Button variant="secondary" onClick={() => setShowForm(true)}>+ Invite</Button>
      </header>

      {showForm ? (
      <SidePanel
        title="Invite a teammate"
        description="The invite link expires in 7 days. Roles can be changed later."
        onClose={() => setShowForm(false)}
        footer={
          <>
            <Button variant="secondary" onClick={() => setShowForm(false)}>Cancel</Button>
            <Button type="submit" form="invite-form" loading={create.isPending}>Send invite</Button>
          </>
        }
      >
      <form
        id="invite-form"
        className="space-y-4 p-5"
        onSubmit={handleSubmit((values) => create.mutate(values))}
      >
        <Field label="Email" htmlFor="invite-email" error={errors.email?.message}>
          <Input
            id="invite-email"
            type="email"
            aria-invalid={Boolean(errors.email)}
            {...register("email")}
            placeholder="dev@acme.io"
          />
        </Field>
        <Field label="Role" htmlFor="invite-role">
          <select
            id="invite-role"
            {...register("role")}
            className="h-9 w-full rounded-md border border-border bg-bg-surface px-3 text-sm"
          >
            {roles.map((r) => (
              <option key={r} value={r}>
                {r}
              </option>
            ))}
          </select>
        </Field>
      </form>
      </SidePanel>
      ) : null}

      {error ? (
        <p role="alert" className="px-4 pb-3 text-xs text-status-danger">
          {error}
        </p>
      ) : null}

      {link ? (
        <div className="mx-4 mb-4 rounded-md border border-status-success/30 bg-status-success/10 p-3 text-xs">
          <p className="font-medium text-status-success">Invite created.</p>
          <p className="mt-1 break-all font-mono text-text-secondary">{link}</p>
        </div>
      ) : null}

      <div className="border-t border-border">
        {invites.isSuccess && invites.data.data.length === 0 ? (
          <p className="p-4 text-sm text-text-secondary">No invites yet.</p>
        ) : null}
        {invites.isSuccess && invites.data.data.length > 0 ? (
          <ul className="divide-y divide-border">
            {invites.data.data.map((invite) => (
              <li key={invite.id} className="flex items-center gap-3 px-4 py-2.5 text-sm">
                <span className="flex-1">{invite.email}</span>
                <span className="rounded bg-bg-hover px-1.5 py-0.5 text-[11px] uppercase text-text-muted">
                  {invite.role}
                </span>
                <span className="text-xs text-text-muted">
                  {invite.accepted_at ? "accepted" : "pending"}
                </span>
              </li>
            ))}
          </ul>
        ) : null}
      </div>
    </section>
  );
}
