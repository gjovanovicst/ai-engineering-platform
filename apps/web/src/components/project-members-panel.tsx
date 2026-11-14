"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { ApiError, listProjectMembers, removeProjectMember, setProjectMember } from "@/lib/api";

export function ProjectMembersPanel({ orgID, projectID }: { orgID: string; projectID: string }) {
  const queryClient = useQueryClient();
  const [userID, setUserID] = useState("");
  const [role, setRole] = useState("member");
  const [error, setError] = useState<string | null>(null);
  const members = useQuery({ queryKey: ["project-members", orgID, projectID], queryFn: () => listProjectMembers(orgID, projectID) });
  const invalidate = () => queryClient.invalidateQueries({ queryKey: ["project-members", orgID, projectID] });

  const add = useMutation({
    mutationFn: () => setProjectMember(orgID, projectID, userID.trim(), role),
    onSuccess: () => { setUserID(""); setError(null); invalidate(); },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not set the project role"),
  });
  const remove = useMutation({
    mutationFn: (id: string) => removeProjectMember(orgID, projectID, id),
    onSuccess: () => { setError(null); invalidate(); },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not remove the project member"),
  });

  return (
    <section className="mt-5 rounded-lg border border-border bg-bg-surface p-4">
      <h2 className="text-sm font-semibold">Project access</h2>
      <p className="mt-1 text-xs text-text-secondary">
        Once a project has members, only they (and org admins) can open it. Viewers are read-only.
      </p>
      <div className="mt-3 flex flex-wrap items-end gap-2">
        <input aria-label="User id" value={userID} onChange={(e) => setUserID(e.target.value)} placeholder="user uuid" className="h-9 min-w-[260px] flex-1 rounded-md border border-border bg-bg-surface px-3 font-mono text-xs" />
        <select aria-label="Role" value={role} onChange={(e) => setRole(e.target.value)} className="h-9 rounded-md border border-border bg-bg-surface px-2 text-xs">
          <option value="admin">admin</option><option value="designer">designer</option><option value="member">member</option><option value="viewer">viewer</option>
        </select>
        <Button variant="secondary" loading={add.isPending} onClick={() => add.mutate()}>Add member</Button>
      </div>
      {error ? <p role="alert" className="mt-2 text-xs text-status-danger">{error}</p> : null}
      {members.isSuccess && members.data.items.length === 0 ? (
        <p className="mt-3 text-xs text-text-muted">No explicit members: the project is open to the organisation.</p>
      ) : null}
      {members.isSuccess && members.data.items.length > 0 ? (
        <ul className="mt-3 divide-y divide-border rounded-md border border-border">
          {members.data.items.map((member) => (
            <li key={member.user_id} className="flex flex-wrap items-center gap-2 px-3 py-2 text-xs">
              <span className="flex-1 font-mono text-text-muted">{member.user_id}</span>
              <span className="rounded bg-bg-hover px-1.5 py-0.5">{member.role}</span>
              <Button variant="secondary" onClick={() => remove.mutate(member.user_id)}>Remove</Button>
            </li>
          ))}
        </ul>
      ) : null}
    </section>
  );
}
