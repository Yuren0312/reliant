// Copyright (c) 2025 Reliant Labs

/**
 * Console v3 PROTOTYPE shell. The proposed replacement for ForgeShell +
 * forge-ui/sidebar_layout (research/console-v3/PROPOSAL.md, "Information
 * architecture").
 *
 * What changes against today's shell:
 *  - The nav is the PRODUCT, not the environment roster. Resources
 *    (Workloads, Releases, Activity, Secrets, Domains) are top-level and
 *    project-scoped; environment is a scope control in the top bar that
 *    filters every page (owner decision 1, kept).
 *  - Sections collapse (Observability, Delivery, Configure). Collapsed state
 *    is per-user, remembered.
 *  - A 52px top bar carries scope (project / env) and search, the way
 *    Vercel's and Cloudflare's do, so the sidebar never has to.
 *  - Density: 32px nav rows, 13px labels, 240px wide, collapsible to icons.
 */

import { useState, type ReactNode } from "react";
import {
  Activity,
  BarChart3,
  Bell,
  Boxes,
  ChevronDown,
  ChevronsUpDown,
  Cpu,
  FileSearch,
  Flame,
  Gauge,
  GitBranch,
  Globe,
  KeyRound,
  LayoutDashboard,
  ListTree,
  PanelLeft,
  Rocket,
  ScrollText,
  Search,
  Settings,
  SlidersHorizontal,
} from "lucide-react";

import { cn } from "@/lib/utils";

import { Dot } from "./primitives";
import { ENVS } from "./fixtures";

export type NavId =
  | "overview"
  | "workloads"
  | "releases"
  | "activity"
  | "obs-overview"
  | "logs"
  | "traces"
  | "metrics"
  | "profiles"
  | "dashboards"
  | "alerts"
  | "secrets"
  | "domains"
  | "environments"
  | "monitoring"
  | "settings";

interface Item {
  id: NavId;
  label: string;
  icon: typeof Activity;
  badge?: ReactNode;
}

interface Section {
  id: string;
  label?: string;
  items: Item[];
}

const SECTIONS: Section[] = [
  {
    id: "top",
    items: [
      { id: "overview", label: "Overview", icon: LayoutDashboard },
      { id: "workloads", label: "Workloads", icon: Boxes, badge: <Dot tone="warning" /> },
      { id: "releases", label: "Releases", icon: Rocket },
      { id: "activity", label: "Activity", icon: ScrollText },
    ],
  },
  {
    id: "observability",
    label: "Observability",
    items: [
      { id: "obs-overview", label: "Overview", icon: Gauge },
      { id: "logs", label: "Logs", icon: FileSearch },
      { id: "traces", label: "Traces", icon: ListTree },
      { id: "metrics", label: "Metrics", icon: BarChart3 },
      { id: "profiles", label: "Profiles", icon: Flame },
      { id: "dashboards", label: "Dashboards", icon: Activity },
      { id: "alerts", label: "Alerts", icon: Bell, badge: <span className="text-xs tabular-nums text-warning-ink">1</span> },
    ],
  },
  {
    id: "configure",
    label: "Configure",
    items: [
      { id: "environments", label: "Environments", icon: GitBranch },
      { id: "secrets", label: "Secrets", icon: KeyRound },
      { id: "domains", label: "Domains", icon: Globe },
      { id: "monitoring", label: "Monitoring settings", icon: SlidersHorizontal },
      { id: "settings", label: "Project settings", icon: Settings },
    ],
  },
];

export function ConsoleShell({
  active,
  env,
  onNavigate,
  children,
}: {
  active: NavId;
  /** The env scope; "all" = every environment. */
  env: string;
  onNavigate?: (id: NavId) => void;
  children: ReactNode;
}) {
  const [collapsed, setCollapsed] = useState(false);
  const [closed, setClosed] = useState<Record<string, boolean>>({});

  return (
    <div className="flex h-screen w-full overflow-hidden bg-background text-foreground" data-testid="console-v3-shell">
      <aside
        className={cn(
          "flex shrink-0 flex-col border-r border-border bg-background transition-[width] duration-150",
          collapsed ? "w-14" : "w-60",
        )}
      >
        {/* Org / project switcher — the one scope that changes what the nav lists. */}
        <div className="flex h-13 shrink-0 items-center gap-2 px-3">
          <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md bg-foreground text-2xs font-bold text-background">R</span>
          {!collapsed && (
            <button type="button" className="flex min-w-0 flex-1 items-center gap-1.5 rounded-md px-1.5 py-1 text-left hover:bg-muted">
              <span className="truncate text-sm font-medium">control-plane</span>
              <ChevronsUpDown className="ml-auto h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            </button>
          )}
        </div>

        <nav aria-label="Console" className="flex-1 overflow-y-auto px-2 pb-3">
          {SECTIONS.map((section) => {
            const isClosed = !!closed[section.id];
            return (
              <div key={section.id} className={section.label ? "mt-4" : "mt-1"}>
                {section.label && !collapsed && (
                  <button
                    type="button"
                    aria-expanded={!isClosed}
                    onClick={() => setClosed((c) => ({ ...c, [section.id]: !c[section.id] }))}
                    className="group flex h-7 w-full items-center gap-1 rounded-md px-2 text-xs font-medium text-muted-foreground hover:text-foreground"
                  >
                    {section.label}
                    <ChevronDown className={cn("h-3 w-3 transition-transform", isClosed && "-rotate-90")} />
                  </button>
                )}
                {section.label && collapsed && <div className="mx-auto my-2 h-px w-6 bg-border" />}
                {!isClosed && (
                  <ul className="space-y-px">
                    {section.items.map((item) => {
                      const Icon = item.icon;
                      const on = item.id === active;
                      return (
                        <li key={item.id}>
                          <button
                            type="button"
                            onClick={() => onNavigate?.(item.id)}
                            aria-current={on ? "page" : undefined}
                            title={collapsed ? item.label : undefined}
                            className={cn(
                              "flex h-8 w-full items-center gap-2.5 rounded-md px-2 text-sm transition",
                              on ? "bg-muted font-medium text-foreground" : "text-muted-foreground hover:bg-muted/60 hover:text-foreground",
                              collapsed && "justify-center",
                            )}
                          >
                            <Icon className={cn("h-4 w-4 shrink-0", on ? "text-foreground" : "text-muted-foreground")} aria-hidden="true" />
                            {!collapsed && <span className="truncate">{item.label}</span>}
                            {!collapsed && item.badge && <span className="ml-auto flex items-center">{item.badge}</span>}
                          </button>
                        </li>
                      );
                    })}
                  </ul>
                )}
              </div>
            );
          })}
        </nav>

        <div className="flex shrink-0 items-center gap-2 border-t border-border p-2">
          <span className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-muted text-xs font-semibold text-muted-foreground">S</span>
          {!collapsed && <span className="truncate text-sm text-muted-foreground">sean@reliantlabs.io</span>}
          <button
            type="button"
            onClick={() => setCollapsed((c) => !c)}
            aria-label={collapsed ? "Expand sidebar" : "Collapse sidebar"}
            className="ml-auto rounded-md p-1.5 text-muted-foreground hover:bg-muted hover:text-foreground"
          >
            <PanelLeft className="h-4 w-4" />
          </button>
        </div>
      </aside>

      <div className="flex min-w-0 flex-1 flex-col">
        {/* Top bar: env scope + search + time. Scope lives here, not in the nav. */}
        <div className="flex h-13 shrink-0 items-center gap-3 border-b border-border bg-background px-4">
          <EnvScope env={env} />
          <div className="mx-auto flex h-8 w-full max-w-md items-center gap-2 rounded-md border border-border bg-card px-2.5 text-sm text-muted-foreground">
            <Search className="h-3.5 w-3.5" />
            <span>Search workloads, releases, traces…</span>
            <kbd className="ml-auto rounded border border-border px-1 font-sans text-2xs">⌘K</kbd>
          </div>
          <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
            <Cpu className="h-3.5 w-3.5" /> 4 environments
          </span>
        </div>
        <main className="relative min-h-0 flex-1 overflow-y-auto">{children}</main>
      </div>
    </div>
  );
}

/**
 * The environment scope: a segmented control for the common case (≤5 envs),
 * a menu beyond that. "All" is the default; it is a FILTER, and every page
 * honours it — the URL carries `?env=prod`.
 */
function EnvScope({ env }: { env: string }) {
  const opts = ["all", ...ENVS.map((e) => e.name)];
  return (
    <div className="flex items-center gap-2">
      <span className="text-xs text-muted-foreground">Environment</span>
      <div role="radiogroup" aria-label="Environment" className="flex h-8 items-center rounded-md border border-border bg-card p-0.5">
        {opts.map((o) => {
          const meta = ENVS.find((e) => e.name === o);
          const on = o === env;
          return (
            <button
              key={o}
              type="button"
              role="radio"
              aria-checked={on}
              className={cn(
                "flex h-6 items-center gap-1.5 rounded px-2 text-xs transition",
                on ? "bg-muted font-medium text-foreground" : "text-muted-foreground hover:text-foreground",
              )}
            >
              {meta && <Dot tone={meta.tone} className="h-1.5 w-1.5" />}
              {o === "all" ? "All" : o}
            </button>
          );
        })}
      </div>
    </div>
  );
}
