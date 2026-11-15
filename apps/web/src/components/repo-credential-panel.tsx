"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { SidePanel } from "@/components/ui/side-panel";
import {
  ApiError,
  deleteRepoCredential,
  getRepoCredential,
  setRepoCredential,
  type Repo,
} from "@/lib/api";

/**
 * Per-repository token used before the GitHub App and the global token.
 * Only owners and admins render this control (the API enforces the same rule).
 */
export function RepoCredentialControl({
  orgID,
  projectID,
  repo,
}: {
  orgID: string;
  projectID: string;
  repo: Repo;
}) {
  const queryClient = useQueryClient();
  const [open, setOpen] = useState(false);
  const [token, setToken] = useState("");
  const [error, setError] = useState<string | null>(null);
  const queryKey = ["repo-credential", orgID, projectID, repo.id];

  const status = useQuery({
    queryKey,
    queryFn: () => getRepoCredential(orgID, projectID, repo.id),
  });

  const save = useMutation({
    mutationFn: () => setRepoCredential(orgID, projectID, repo.id, token),
    onSuccess: () => {
      setOpen(false);
      setToken("");
      setError(null);
      queryClient.invalidateQueries({ queryKey });
    },
    onError: (err) =>
      setError(err instanceof ApiError ? err.message : "Could not store the credential"),
  });

  const remove = useMutation({
    mutationFn: () => deleteRepoCredential(orgID, projectID, repo.id),
    onSuccess: () => {
      setOpen(false);
      setError(null);
      queryClient.invalidateQueries({ queryKey });
    },
    onError: (err) =>
      setError(err instanceof ApiError ? err.message : "Could not remove the credential"),
  });

  const configured = status.data?.configured ?? false;

  return (
    <>
      <Button
        variant="secondary"
        onClick={() => setOpen(true)}
        aria-label={`Configure git credential for ${repo.name}`}
      >
        {status.isPending ? "…" : configured ? `Token ${status.data?.hint ?? ""}` : "Credential"}
      </Button>

      {open ? (
        <SidePanel
          title="Repository credential"
          description="Used before the GitHub App and the global token. Stored encrypted; the token is never shown again."
          onClose={() => setOpen(false)}
          footer={
            <>
              {configured ? (
                <Button
                  type="button"
                  variant="danger"
                  loading={remove.isPending}
                  onClick={() => remove.mutate()}
                >
                  Remove credential
                </Button>
              ) : null}
              <Button type="button" variant="secondary" onClick={() => setOpen(false)}>
                Cancel
              </Button>
              <Button
                type="submit"
                form={`repo-credential-form-${repo.id}`}
                loading={save.isPending}
                disabled={token.trim().length === 0}
              >
                Save
              </Button>
            </>
          }
        >
          <form
            id={`repo-credential-form-${repo.id}`}
            className="space-y-4 p-5"
            onSubmit={(event) => {
              event.preventDefault();
              save.mutate();
            }}
          >
            <div className="rounded-lg border border-border bg-bg-subtle p-3 text-xs text-text-secondary">
              <p className="font-medium text-text-primary">{repo.name}</p>
              {status.isError ? (
                <p role="alert" className="mt-1 text-status-danger">
                  Could not load the credential status.
                </p>
              ) : status.isPending ? (
                <p className="mt-1">Loading status…</p>
              ) : configured ? (
                <p className="mt-1">
                  Own credential configured ({status.data?.hint}).{" "}
                  {status.data?.updated_at
                    ? `Updated ${new Date(status.data.updated_at).toLocaleString()}.`
                    : null}
                </p>
              ) : (
                <p className="mt-1">
                  No own credential: the repository uses the GitHub App or the organisation
                  token.
                </p>
              )}
            </div>

            <Field label="Token" htmlFor={`repo-credential-token-${repo.id}`}>
              <Input
                id={`repo-credential-token-${repo.id}`}
                type="password"
                autoComplete="off"
                placeholder="github_pat_… or glpat-…"
                value={token}
                onChange={(event) => setToken(event.target.value)}
              />
            </Field>

            {error ? (
              <p role="alert" className="text-xs text-status-danger">
                {error}
              </p>
            ) : null}
          </form>
        </SidePanel>
      ) : null}
    </>
  );
}
