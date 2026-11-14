"use client";

import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { RuleDiff } from "@/components/rule-diff";
import { Button } from "@/components/ui/button";
import { Field } from "@/components/ui/field";
import { Input } from "@/components/ui/input";
import { SidePanel } from "@/components/ui/side-panel";
import {
  ApiError,
  createRule,
  decideRulePublishApproval,
  listRulePublishApprovals,
  listRuleVersions,
  listRules,
  publishRuleVersion,
  saveRuleDraft,
  type Rule,
} from "@/lib/api";

const levels = ["organization", "project", "module", "task"];

export function RulesPanel({ orgID }: { orgID: string }) {
  const queryClient = useQueryClient();
  const [selected, setSelected] = useState<Rule | null>(null);
  const [form, setForm] = useState({ name: "", level: "organization", body: "" });
  const [showForm, setShowForm] = useState(false);
  const [draft, setDraft] = useState("");
  const [compare, setCompare] = useState<{ left: number; right: number }>({ left: 0, right: 0 });
  const [error, setError] = useState<string | null>(null);

  const rules = useQuery({ queryKey: ["rules", orgID], queryFn: () => listRules(orgID) });
  const active = selected ?? rules.data?.data[0] ?? null;

  const versions = useQuery({
    queryKey: ["rule-versions", orgID, active?.id],
    enabled: Boolean(active?.id),
    queryFn: () => listRuleVersions(orgID, active!.id),
  });

  const create = useMutation({
    mutationFn: () => createRule(orgID, form),
    onSuccess: () => {
      setForm({ name: "", level: "organization", body: "" });
      setShowForm(false);
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["rules", orgID] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not create the rule"),
  });

  const saveDraft = useMutation({
    mutationFn: () => saveRuleDraft(orgID, active!.id, draft),
    onSuccess: () => {
      setDraft("");
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["rule-versions", orgID, active?.id] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not save the draft"),
  });

  const pending = useQuery({ queryKey: ["rule-approvals", orgID], queryFn: () => listRulePublishApprovals(orgID) });

  const decide = useMutation({
    mutationFn: (input: { id: string; decision: "approved" | "rejected" }) =>
      decideRulePublishApproval(orgID, input.id, input.decision),
    onSuccess: () => {
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["rule-approvals", orgID] });
      queryClient.invalidateQueries({ queryKey: ["rule-versions", orgID, active?.id] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not decide the publication"),
  });

  const publish = useMutation({
    mutationFn: (version: number) => publishRuleVersion(orgID, active!.id, version),
    onSuccess: () => {
      setError(null);
      queryClient.invalidateQueries({ queryKey: ["rule-approvals", orgID] });
      queryClient.invalidateQueries({ queryKey: ["rule-versions", orgID, active?.id] });
    },
    onError: (err) => setError(err instanceof ApiError ? err.message : "Could not request publication"),
  });

  return (
    <div className="grid gap-4 lg:grid-cols-[1fr_1.4fr]">
      <section className="rounded-lg border border-border bg-bg-surface">
        <header className="flex items-start justify-between gap-2 border-b border-border p-4">
          <div>
            <h2 className="text-sm font-semibold">Rules</h2>
            <p className="mt-1 text-xs text-text-secondary">
              Published rules are injected into every agent prompt.
            </p>
          </div>
          <Button variant="secondary" onClick={() => setShowForm(true)}>+ New rule</Button>
        </header>
        <ul className="divide-y divide-border">
          {rules.isSuccess && rules.data.data.length === 0 ? (
            <li className="p-4 text-sm text-text-secondary">No rules yet.</li>
          ) : null}
          {rules.isSuccess
            ? rules.data.data.map((rule) => (
                <li key={rule.id}>
                  <button
                    type="button"
                    onClick={() => setSelected(rule)}
                    className={`flex w-full items-center gap-2 px-4 py-2.5 text-left text-sm hover:bg-bg-hover ${
                      active?.id === rule.id ? "bg-bg-hover" : ""
                    }`}
                  >
                    <span className="flex-1">{rule.name}</span>
                    <span className="rounded bg-bg-hover px-1.5 py-0.5 text-[11px] uppercase text-text-muted">
                      {rule.level}
                    </span>
                  </button>
                </li>
              ))
            : null}
        </ul>

        {showForm ? (
        <SidePanel
          title="New rule"
          description="Rules are injected into agent prompts; publishing needs approval."
          onClose={() => setShowForm(false)}
          footer={
            <>
              <Button variant="secondary" onClick={() => setShowForm(false)}>Cancel</Button>
              <Button type="submit" form="rule-form" loading={create.isPending}>Create rule</Button>
            </>
          }
        >
        <form
          id="rule-form"
          className="space-y-3 p-5"
          onSubmit={(event) => {
            event.preventDefault();
            create.mutate();
          }}
        >
          <Field label="New rule name" htmlFor="rule-name">
            <Input id="rule-name" required value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
          </Field>
          <Field label="Level" htmlFor="rule-level">
            <select
              id="rule-level"
              value={form.level}
              onChange={(e) => setForm({ ...form, level: e.target.value })}
              className="h-9 w-full rounded-md border border-border bg-bg-surface px-3 text-sm"
            >
              {levels.map((level) => (
                <option key={level} value={level}>
                  {level}
                </option>
              ))}
            </select>
          </Field>
          <Field label="Body (markdown)" htmlFor="rule-body">
            <textarea
              id="rule-body"
              required
              rows={4}
              value={form.body}
              onChange={(e) => setForm({ ...form, body: e.target.value })}
              className="w-full rounded-md border border-border bg-bg-surface px-3 py-2 text-sm"
            />
          </Field>
        </form>
        </SidePanel>
        ) : null}
      </section>

      <section className="rounded-lg border border-border bg-bg-surface">
        <header className="border-b border-border p-4">
          <h2 className="text-sm font-semibold">{active ? active.name : "Select a rule"}</h2>
          <p className="mt-1 text-xs text-text-secondary">
            {active ? `${active.level} level` : "Create a rule to get started."}
          </p>
        </header>

        {error ? (
          <p role="alert" className="border-b border-border p-3 text-xs text-status-danger">
            {error}
          </p>
        ) : null}

        {active ? (
          <>
            {versions.isSuccess && versions.data.data.length >= 2 ? (
              <div className="flex flex-wrap items-end gap-2 border-b border-border p-3">
                <label className="text-xs text-text-secondary" htmlFor="diff-left">
                  Compare
                </label>
                <select
                  id="diff-left"
                  value={compare.left || versions.data.data[0].version}
                  onChange={(e) => setCompare({ ...compare, left: Number(e.target.value) })}
                  className="h-8 rounded-md border border-border bg-bg-surface px-2 text-xs"
                >
                  {versions.data.data.map((version) => (
                    <option key={version.version} value={version.version}>
                      v{version.version}
                    </option>
                  ))}
                </select>
                <span className="text-xs text-text-muted">→</span>
                <select
                  aria-label="Compare with version"
                  value={compare.right || versions.data.data[1].version}
                  onChange={(e) => setCompare({ ...compare, right: Number(e.target.value) })}
                  className="h-8 rounded-md border border-border bg-bg-surface px-2 text-xs"
                >
                  {versions.data.data.map((version) => (
                    <option key={version.version} value={version.version}>
                      v{version.version}
                    </option>
                  ))}
                </select>
              </div>
            ) : null}
            <ul className="divide-y divide-border">
              {versions.isSuccess
                ? versions.data.data.map((version) => (
                    <li key={version.version} className="flex items-start gap-3 p-3 text-sm">
                      <span className="w-10 font-mono text-xs text-text-muted">v{version.version}</span>
                      <pre className="max-h-32 flex-1 overflow-auto whitespace-pre-wrap font-sans text-xs text-text-secondary">
                        {version.body}
                      </pre>
                      {version.published ? (
                        <span className="rounded bg-status-success/10 px-1.5 py-0.5 text-[11px] text-status-success">
                          active
                        </span>
                      ) : (
                        <Button
                          variant="secondary"
                          loading={publish.isPending && publish.variables === version.version}
                          onClick={() => publish.mutate(version.version)}
                        >
                          Request publish
                        </Button>
                      )}
                    </li>
                  ))
                : null}
            </ul>

            {versions.isSuccess && versions.data.data.length >= 2 ? (
              <RuleDiff
                leftLabel={`v${compare.left || versions.data.data[0].version}`}
                rightLabel={`v${compare.right || versions.data.data[1].version}`}
                left={versions.data.data.find((v) => v.version === (compare.left || versions.data.data[0].version))?.body ?? ""}
                right={versions.data.data.find((v) => v.version === (compare.right || versions.data.data[1].version))?.body ?? ""}
              />
            ) : null}

            {pending.isSuccess && pending.data.data.length > 0 ? (
              <div className="border-t border-border p-4">
                <h3 className="text-xs font-semibold uppercase text-text-muted">Waiting for a second admin</h3>
                <ul className="mt-2 space-y-2">
                  {pending.data.data.map((item) => (
                    <li key={item.id} className="flex flex-wrap items-center gap-2 text-sm">
                      <span className="flex-1">
                        {item.rule} <span className="text-text-muted">v{item.version}</span>
                      </span>
                      <Button
                        variant="secondary"
                        className="h-7 px-2 text-xs"
                        loading={decide.isPending}
                        onClick={() => decide.mutate({ id: item.id, decision: "approved" })}
                      >
                        Approve
                      </Button>
                      <Button
                        variant="danger"
                        className="h-7 px-2 text-xs"
                        onClick={() => decide.mutate({ id: item.id, decision: "rejected" })}
                      >
                        Reject
                      </Button>
                    </li>
                  ))}
                </ul>
              </div>
            ) : null}

            <div className="space-y-2 border-t border-border p-4">
              <Field label="New draft version" htmlFor="rule-draft">
                <textarea
                  id="rule-draft"
                  rows={4}
                  value={draft}
                  onChange={(e) => setDraft(e.target.value)}
                  className="w-full rounded-md border border-border bg-bg-surface px-3 py-2 text-sm"
                />
              </Field>
              <Button loading={saveDraft.isPending} disabled={draft.trim() === ""} onClick={() => saveDraft.mutate()}>
                Save draft
              </Button>
            </div>
          </>
        ) : null}
      </section>
    </div>
  );
}
