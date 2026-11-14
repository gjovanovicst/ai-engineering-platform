"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { ApiError, listMembers, removeMember, updateMemberRole } from "@/lib/api";

const assignableRoles = ["admin", "designer", "member", "viewer"];

export function MembersPanel({ orgID }: { orgID: string }) {
  const queryClient = useQueryClient();
  const [error, setError] = useState<string | null>(null);
  const [draftRoles, setDraftRoles] = useState<Record<string, string>>({});
  const members = useQuery({ queryKey: ["members", orgID], queryFn: () => listMembers(orgID) });

  const changeRole = useMutation({
    mutationFn: (input: { userID: string; role: string }) => updateMemberRole(orgID, input.userID, input.role),
    onSuccess: () => {
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["members", orgID] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not change the role"),
  });

  const remove = useMutation({
    mutationFn: (userID: string) => removeMember(orgID, userID),
    onSuccess: () => {
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["members", orgID] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not remove the member"),
  });

  return (
    <section className="rounded-lg border border-border bg-bg-surface">
      <header className="border-b border-border p-4">
        <h2 className="text-sm font-semibold">Members</h2>
        <p className="mt-1 text-xs text-text-secondary">Everyone with access to this workspace.</p>
      </header>
      {members.isPending ? <p className="p-4 text-sm text-text-muted">Loading…</p> : null}
      {members.isError ? (
        <p role="alert" className="p-4 text-sm text-status-danger">Could not load members.</p>
      ) : null}
      {error ? (
        <p role="alert" className="border-b border-border p-3 text-xs text-status-danger">
          {error}
        </p>
      ) : null}
      {members.isSuccess ? (
        <ul className="divide-y divide-border">
          {members.data.data.map((member) => {
            const draft = draftRoles[member.user_id] ?? member.role;
            return (
              <li key={member.user_id} className="flex flex-wrap items-center gap-2 px-4 py-2.5 text-sm">
                <span className="flex-1">
                  {member.name}
                  <span className="ml-2 text-xs text-text-muted">{member.email}</span>
                </span>
                {member.role === "owner" ? (
                  <span className="rounded bg-bg-hover px-1.5 py-0.5 text-[11px] uppercase text-text-muted">
                    owner
                  </span>
                ) : (
                  <>
                    <select
                      aria-label={`Role for ${member.email}`}
                      value={draft}
                      onChange={(e) => setDraftRoles({ ...draftRoles, [member.user_id]: e.target.value })}
                      className="h-8 rounded-md border border-border bg-bg-surface px-2 text-xs"
                    >
                      {assignableRoles.map((role) => (
                        <option key={role} value={role}>
                          {role}
                        </option>
                      ))}
                    </select>
                    <Button
                      variant="secondary"
                      className="h-7 px-2 text-xs"
                      loading={changeRole.isPending && changeRole.variables?.userID === member.user_id}
                      onClick={() => changeRole.mutate({ userID: member.user_id, role: draft })}
                    >
                      Save
                    </Button>
                    <Button
                      variant="danger"
                      className="h-7 px-2 text-xs"
                      loading={remove.isPending && remove.variables === member.user_id}
                      onClick={() => remove.mutate(member.user_id)}
                    >
                      Remove
                    </Button>
                  </>
                )}
              </li>
            );
          })}
        </ul>
      ) : null}
    </section>
  );
}
