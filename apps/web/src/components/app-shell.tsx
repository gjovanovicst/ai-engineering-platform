"use client";

import Link from "next/link";
import { useRouter, usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { decodeClaims, type Claims } from "@/lib/auth";
import { ensureSession, listNotifications, logout, logoutAll, markNotificationsRead } from "@/lib/api";
import { getAccessToken } from "@/lib/session";
import { BrandMark } from "@/components/brand-mark";
import { AssistantPanel } from "@/components/assistant-panel";
import { CommandPalette } from "@/components/command-palette";
import { HelpPanel } from "@/components/help-panel";

type NavItem = { label: string; href: string; icon: string; group: string; developerOnly?: boolean };

const nav: NavItem[] = [
  { label: "Overview", href: "/overview", icon: "grid", group: "WORKSPACE" },
  { label: "Projects", href: "/projects", icon: "layers", group: "WORKSPACE" },
  { label: "Search", href: "/search", icon: "search", group: "WORKSPACE" },
  { label: "AI Agents", href: "/agents", icon: "spark", group: "BUILD" },
  { label: "Repositories", href: "/repositories", icon: "git", group: "BUILD" },
  { label: "Blueprints", href: "/blueprints", icon: "blueprint", group: "BUILD" },
  { label: "Design standards", href: "/design-system", icon: "palette", group: "DEVELOPER TOOLS", developerOnly: true },
  { label: "Component registry", href: "/components", icon: "component", group: "DEVELOPER TOOLS", developerOnly: true },
  { label: "Playground", href: "/design-system/playground", icon: "component", group: "DEVELOPER TOOLS", developerOnly: true },
  { label: "Analytics", href: "/analytics", icon: "chart", group: "INSIGHTS" },
  { label: "Settings", href: "/settings", icon: "settings", group: "INSIGHTS" },
];

const groups = ["WORKSPACE", "BUILD", "DEVELOPER TOOLS", "INSIGHTS"];

function Icon({ name, size = 18 }: { name: string; size?: number }) {
  const paths: Record<string, React.ReactNode> = {
    grid: <><rect x="3" y="3" width="7" height="7" rx="1.5"/><rect x="14" y="3" width="7" height="7" rx="1.5"/><rect x="3" y="14" width="7" height="7" rx="1.5"/><rect x="14" y="14" width="7" height="7" rx="1.5"/></>,
    layers: <><path d="m12 3 9 5-9 5-9-5 9-5Z"/><path d="m3 12 9 5 9-5"/><path d="m3 16 9 5 9-5"/></>,
    search: <><circle cx="10.5" cy="10.5" r="6.5"/><path d="m16 16 5 5"/></>,
    spark: <><path d="m12 3 1.9 5.8L20 11l-6.1 2.2L12 19l-1.9-5.8L4 11l6.1-2.2L12 3Z"/><path d="m19 14 1.1 2.1L22 17l-1.9.9L19 20l-.9-2.1L16 17l2.1-.9L19 14Z"/></>,
    git: <><circle cx="6" cy="5" r="2"/><circle cx="18" cy="7" r="2"/><circle cx="6" cy="19" r="2"/><path d="M6 7v10"/><path d="M8 5h4a4 4 0 0 1 4 4v0"/></>,
    blueprint: <><path d="M4 3h12l4 4v14H4z"/><path d="M16 3v5h4"/><path d="M8 12h8M8 16h8"/></>,
    palette: <><path d="M12 3a9 9 0 1 0 0 18h1.2a2 2 0 0 0 1.4-3.4 1.8 1.8 0 0 1 1.3-3.1H18a3 3 0 0 0 3-3c0-4.7-4-8.5-9-8.5Z"/><circle cx="7.5" cy="11" r="1"/><circle cx="10" cy="7.5" r="1"/><circle cx="15" cy="8" r="1"/></>,
    component: <><rect x="3" y="3" width="8" height="8" rx="2"/><rect x="14" y="3" width="7" height="5" rx="1.5"/><rect x="14" y="11" width="7" height="10" rx="1.5"/><rect x="3" y="14" width="8" height="7" rx="1.5"/></>,
    chart: <><path d="M4 19V5M4 19h17"/><path d="m7 15 4-4 3 2 5-7"/></>,
    settings: <><circle cx="12" cy="12" r="3"/><path d="m19.4 15 .1.1 1.4 1.1-1.4 2.4-1.7-.6a8 8 0 0 1-1.8 1l-.3 1.8h-2.8l-.3-1.8a8 8 0 0 1-1.8-1l-1.7.6-1.4-2.4L7.1 15a8 8 0 0 1 0-2l-1.4-1.1 1.4-2.4 1.7.6a8 8 0 0 1 1.8-1l.3-1.8h2.8l.3 1.8a8 8 0 0 1 1.8 1l1.7-.6 1.4 2.4-1.4 1.1a8 8 0 0 1-.1 2Z"/></>,
    bell: <><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9"/><path d="M10 21h4"/></>,
    sun: <><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.93 4.93l1.42 1.42m11.3 11.3 1.42 1.42M2 12h2m16 0h2M4.93 19.07l1.42-1.42m11.3-11.3 1.42-1.42"/></>,
    moon: <path d="M20.5 14.2A8.5 8.5 0 0 1 9.8 3.5 8.5 8.5 0 1 0 20.5 14.2Z"/>,
    menu: <><path d="M4 6h16M4 12h16M4 18h16"/></>,
    chevron: <path d="m9 18 6-6-6-6"/>,
  };
  return <svg aria-hidden="true" width={size} height={size} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.7" strokeLinecap="round" strokeLinejoin="round">{paths[name] ?? paths.grid}</svg>;
}

export function AppShell({ orgName, children }: { orgName: string; children: React.ReactNode }) {
  const pathname = usePathname();
  const router = useRouter();
  const isActive = (href: string) => pathname === href || pathname.startsWith(`${href}/`);
  const [theme, setTheme] = useState<"light" | "dark">("light");
  const [claims, setClaims] = useState<Claims | null>(null);
  const [menuOpen, setMenuOpen] = useState(false);
  const [helpOpen, setHelpOpen] = useState(false);
  const [paletteOpen, setPaletteOpen] = useState(false);
  const [assistantOpen, setAssistantOpen] = useState(false);
  const [signingOut, setSigningOut] = useState(false);
  const [bellOpen, setBellOpen] = useState(false);
  const [mobileNavOpen, setMobileNavOpen] = useState(false);

  useEffect(() => {
    const openAssistant = () => setAssistantOpen(true);
    window.addEventListener("aip:assistant", openAssistant);
    return () => window.removeEventListener("aip:assistant", openAssistant);
  }, []);

  useEffect(() => {
    const onKeyDown = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key === "/") {
        event.preventDefault();
        setAssistantOpen(true);
        return;
      }
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "k") {
        event.preventDefault();
        setPaletteOpen((open) => !open);
      }
    };
    window.addEventListener("keydown", onKeyDown);
    return () => window.removeEventListener("keydown", onKeyDown);
  }, []);

  useEffect(() => {
    const stored = window.localStorage.getItem("aip-theme");
    const prefersDark = window.matchMedia?.("(prefers-color-scheme: dark)").matches ?? false;
    const initial = stored === "dark" || stored === "light" ? stored : prefersDark ? "dark" : "light";
    setTheme(initial);
    document.documentElement.classList.toggle("dark", initial === "dark");
    document.documentElement.style.colorScheme = initial;
  }, []);

  useEffect(() => {
    let cancelled = false;
    const token = getAccessToken();
    setClaims(token ? decodeClaims(token) : null);
    if (!token) {
      ensureSession().then((session) => {
        if (!cancelled && session) setClaims(session);
      });
    }
    setMobileNavOpen(false);
    return () => {
      cancelled = true;
    };
  }, [pathname]);

  const toggleTheme = () => {
    const next = theme === "dark" ? "light" : "dark";
    setTheme(next);
    window.localStorage.setItem("aip-theme", next);
    document.documentElement.classList.toggle("dark", next === "dark");
    document.documentElement.style.colorScheme = next;
  };

  const notifications = useQuery({
    queryKey: ["notifications", claims?.org], enabled: Boolean(claims?.org),
    refetchInterval: 30_000, queryFn: () => listNotifications(claims!.org),
  });
  const unread = useQuery({
    queryKey: ["notifications-unread", claims?.org], enabled: Boolean(claims?.org),
    refetchInterval: 30_000, queryFn: () => listNotifications(claims!.org, true),
  });
  const unreadCount = unread.data?.data.length ?? 0;

  const openNotification = async (id: string, target: string) => {
    setBellOpen(false);
    if (claims?.org) await markNotificationsRead(claims.org, [id]).catch(() => undefined);
    if (target) router.push(target);
  };

  const signOutEverywhere = async () => {
    setSigningOut(true);
    try { await logoutAll(); } catch { /* Session may already be revoked. */ }
    finally {
      await logout().catch(() => undefined);
      router.replace("/login");
    }
  };
  const signOut = async () => {
    setSigningOut(true);
    try { await logout(); } finally { router.replace("/login"); }
  };

  return (
    <div className="app-frame">
      <a href="#main-content" className="skip-link">Skip to content</a>
      {mobileNavOpen && <button aria-label="Close navigation" className="sidebar-backdrop" onClick={() => setMobileNavOpen(false)} />}
      <aside className={`app-sidebar ${mobileNavOpen ? "app-sidebar-open" : ""}`}>
        <Link href="/overview" className="brand-lockup">
          <BrandMark />
          <span className="brand-copy"><strong>AI Engineering</strong><small>PLATFORM</small></span>
        </Link>
        <button type="button" className="workspace-switcher" onClick={() => router.push("/settings")} aria-label="Open workspace settings">
          <span className="workspace-avatar">{orgName.slice(0, 1).toUpperCase()}</span>
          <span className="workspace-copy"><strong>{orgName}</strong><small>Workspace</small></span>
          <span className="workspace-chevron"><Icon name="chevron" size={15} /></span>
        </button>
        <nav aria-label="Main navigation" className="primary-nav">
          {groups.map((group) => {
            const items = nav.filter((item) => item.group === group);
            if (items.length === 0) return null;
            return (
            <section key={group} className="nav-group" aria-label={group}>
              <p className="nav-group-label">{group}</p>
              {items.map((item) => (
                <Link key={item.href} href={item.href} aria-current={isActive(item.href) ? "page" : undefined}
                  className={`nav-link ${isActive(item.href) ? "nav-link-active" : ""}`}>
                  <Icon name={item.icon} />
                  <span>{item.label}</span>
                  {item.label === "AI Agents" && <span className="nav-live-dot" />}
                </Link>
              ))}
            </section>
            );
          })}
        </nav>
        <div className="sidebar-footer">
          <div className="sidebar-status"><span className="status-dot" /><span>All systems operational</span></div>
          <div className="sidebar-footer-note">AI-assisted development, with human control.</div>
        </div>
      </aside>

      <div className="app-main">
        <header className="app-topbar">
          <button type="button" className="icon-button mobile-menu-button" aria-label="Open navigation" onClick={() => setMobileNavOpen(true)}><Icon name="menu" /></button>
          <div className="breadcrumbs"><span>Workspace</span><span className="breadcrumb-slash">/</span><strong>{nav.find((item) => isActive(item.href))?.label ?? "Overview"}</strong></div>
          <button type="button" onClick={() => setPaletteOpen(true)} className="global-search"><Icon name="search" size={16} /><span>Search anything...</span><kbd>⌘ K</kbd></button>
          <div className="topbar-actions">
            <div className="popover-anchor">
              <button type="button" onClick={() => setBellOpen((open) => !open)} aria-label={unreadCount > 0 ? `Notifications (${unreadCount} unread)` : "Notifications"} aria-expanded={bellOpen} className="icon-button notification-button">
                <Icon name="bell" size={18} />{unreadCount > 0 && <span className="notification-count">{unreadCount > 9 ? "9+" : unreadCount}</span>}
              </button>
              {bellOpen && <div className="popover notification-popover">
                <div className="popover-heading"><strong>Notifications</strong><button type="button" onClick={() => { if (claims?.org) markNotificationsRead(claims.org).then(() => { notifications.refetch(); unread.refetch(); }); }}>Mark all read</button></div>
                <ul className="notification-list">
                  {notifications.data?.data.length ? notifications.data.data.map((item) => <li key={item.id} className={item.read_at ? "notification-read" : ""}>
                    <button type="button" onClick={() => openNotification(item.id, item.target)}><strong>{item.title}</strong>{item.body && <span>{item.body}</span>}</button>
                  </li>) : <li className="notification-empty">You’re all caught up.</li>}
                </ul>
              </div>}
            </div>
            <button type="button" onClick={() => setAssistantOpen(true)} aria-label="Assistant (⌘/)" title="Assistant (⌘/)" className="icon-button"><span aria-hidden="true">✦</span></button>
            <button type="button" onClick={() => setHelpOpen(true)} aria-label="Help" className="icon-button"><span aria-hidden="true">?</span></button>
            <button type="button" onClick={toggleTheme} aria-label={theme === "dark" ? "Switch to light theme" : "Switch to dark theme"} title={theme === "dark" ? "Light theme" : "Dark theme"} className="icon-button"><Icon name={theme === "dark" ? "sun" : "moon"} size={18} /></button>
            <span className="topbar-divider" />
            <div className="popover-anchor">
              <button type="button" onClick={() => setMenuOpen((open) => !open)} aria-haspopup="menu" aria-expanded={menuOpen} aria-label="Account menu" className="account-trigger">
                <span className="account-avatar">{(claims?.role ?? "O").slice(0, 1).toUpperCase()}</span>
                <span className="account-label"><strong>{claims?.role ?? "Owner"}</strong><small>Account</small></span>
                <span className="account-chevron"><Icon name="chevron" size={14} /></span>
              </button>
              {menuOpen && <div role="menu" aria-label="Account" className="popover account-popover">
                <div className="account-popover-header"><strong>Signed in</strong><span>{claims?.role}</span></div>
                <Link href="/settings" role="menuitem" onClick={() => setMenuOpen(false)}>Settings</Link>
                <button type="button" role="menuitem" disabled={signingOut} onClick={signOut}>{signingOut ? "Signing out…" : "Sign out"}</button>
                <button type="button" role="menuitem" disabled={signingOut} onClick={signOutEverywhere}>Sign out everywhere</button>
              </div>}
            </div>
          </div>
          {helpOpen ? <HelpPanel onClose={() => setHelpOpen(false)} /> : null}
          {paletteOpen && claims?.org ? <CommandPalette orgID={claims.org} onClose={() => setPaletteOpen(false)} /> : null}
          {assistantOpen && claims?.org ? <AssistantPanel orgID={claims.org} onClose={() => setAssistantOpen(false)} /> : null}
        </header>
        <main id="main-content" tabIndex={-1} className="app-content">{children}</main>

      </div>
    </div>
  );
}
