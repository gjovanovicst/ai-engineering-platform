"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

import { AuditPanel } from "@/components/audit-panel";
import { InvitesPanel } from "@/components/invites-panel";
import { MembersPanel } from "@/components/members-panel";
import { WorkspacePanel } from "@/components/workspace-panel";
import { ProvidersPanel } from "@/components/providers-panel";
import { APIKeysPanel } from "@/components/api-keys-panel";
import { AgentPluginsPanel } from "@/components/agent-plugins-panel";
import { BYOKPanel } from "@/components/byok-panel";
import { DRPanel } from "@/components/dr-panel";
import { ScimPanel } from "@/components/scim-panel";
import { SSOPanel } from "@/components/sso-panel";
import { GitHubAppPanel } from "@/components/github-app-panel";
import { RulesPanel } from "@/components/rules-panel";
import { Tabs } from "@/components/ui/tabs";
import { ensureSession } from "@/lib/api";
import type { Claims } from "@/lib/auth";

const tabs = [
  { id: "workspace", label: "Workspace" },
  { id: "members", label: "Members" },
  { id: "invites", label: "Invites" },
  { id: "rules", label: "Rules" },
  { id: "providers", label: "Providers" },
  { id: "github", label: "GitHub App" },
  { id: "api-keys", label: "API keys" },
  { id: "scim", label: "SCIM" },
  { id: "sso", label: "SSO" },
  { id: "crypto", label: "Encryption" },
  { id: "agents", label: "Agent plugins" },
  { id: "backups", label: "Backups" },
  { id: "audit", label: "Audit" },
] as const;
type Tab = (typeof tabs)[number]["id"];

export default function SettingsPage() {
  const router = useRouter();
  const [claims, setClaims] = useState<Claims | null>(null);
  const [tab, setTab] = useState<Tab>("members");

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
        <h1 className="text-[22px] font-semibold">Settings</h1>
        <p className="mt-1 text-[13px] text-text-secondary">Workspace members, access and audit.</p>
      </header>

      <Tabs ariaLabel="Settings sections" value={tab} onChange={(id) => setTab(id as Tab)} items={[...tabs]} />

      {claims && tab === "workspace" ? <WorkspacePanel orgID={claims.org} /> : null}
      {claims && tab === "members" ? <MembersPanel orgID={claims.org} /> : null}
      {claims && tab === "invites" ? <InvitesPanel orgID={claims.org} /> : null}
      {claims && tab === "rules" ? <RulesPanel orgID={claims.org} /> : null}
      {claims && tab === "providers" ? <ProvidersPanel orgID={claims.org} /> : null}
      {claims && tab === "github" ? <GitHubAppPanel orgID={claims.org} /> : null}
      {claims && tab === "api-keys" ? <APIKeysPanel orgID={claims.org} /> : null}
      {claims && tab === "scim" ? <ScimPanel orgID={claims.org} /> : null}
      {claims && tab === "sso" ? <SSOPanel orgID={claims.org} /> : null}
      {claims && tab === "crypto" ? <BYOKPanel orgID={claims.org} /> : null}
      {claims && tab === "agents" ? <AgentPluginsPanel orgID={claims.org} /> : null}
      {claims && tab === "backups" ? <DRPanel orgID={claims.org} /> : null}
      {claims && tab === "audit" ? <AuditPanel orgID={claims.org} /> : null}
    </div>
  );
}
