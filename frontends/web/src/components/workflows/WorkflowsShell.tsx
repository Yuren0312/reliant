// Copyright (c) 2025 Reliant Labs

/**
 * The Workflows area's chrome (WORKFLOW_UI.md §1.2): a left sidebar —
 * Library · Automations · Runs, Automations badged with how many need you —
 * a sidebar-brand exit, a header bar with the section and project scope,
 * and project resolution. It is the layout route every `/workflows/*`
 * page renders inside, so a cross-link between sections (workflow → its runs,
 * run → its automation, automation → its workflow) stays inside one shell.
 *
 * WHY A SIDEBAR. The sections used to be tab links squeezed into the title
 * bar beside "Close", where they read as window chrome rather than as the
 * area's navigation. A labelled vertical nav is the app's standard for a
 * full-page area — Settings and Deployments (Forge/Console/ConsoleShell) both use one — so
 * the three full-page areas now share a shape. Like the console, the brand
 * row owns the way out; the title bar owns the breadcrumb and scope.
 *
 * PROJECT RESOLUTION is the same ladder as ForgeLayout, for the same reason:
 * these routes sit under the bare `_authenticated` layout, which never mounts
 * ModernApp, and ModernApp is the only thing that selects a project on a page
 * load. Without this a hard refresh on /workflows/library had no project and
 * the Library — whose definitions are per project — could not load.
 *
 *   a. currentProject is set       → reflect it into `?project=`.
 *   b. `?project=` is set          → load projects, select that one.
 *   c. neither                     → restoreLastProject(), as the app shell does.
 *   d. still nothing               → the content renders its own empty state
 *                                    (Runs and Automations span projects and
 *                                    work without one; the Library offers the
 *                                    project switcher in the header).
 *
 * The URL is reflected with `replace`, so it never adds a history entry. A
 * project is background context here — Runs can widen to every project and
 * Automations always lists every one (decision 10) — so a missing project is
 * never a dead end.
 */

import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import { Outlet, useLocation, useNavigate, useSearch } from "@tanstack/react-router";
import { Activity, ArrowLeft, CalendarClock, Check, ChevronDown, Library, X } from "lucide-react";

import SidebarLayout from "@/components/forge-ui/sidebar_layout";
import { useNarrowViewport } from "@/hooks/useNarrowViewport";
import { useRouteProjectResolution } from "@/hooks/useRouteProjectResolution";
import { useTitleBarChrome } from "@/hooks/useTitleBarChrome";
import { getParentRouteNavigateOptions } from "@/lib/routeParent";
import { cn } from "@/lib/utils";
import {
  WORKFLOWS_AREA_PATH,
  WORKFLOWS_TABS,
  workflowsTabForPath,
  type WorkflowsTab,
} from "@/lib/workflowsArea";
import { useProjectStore } from "@/store/projectStore";
import { useTriggers } from "@/hooks/trigger-queries";
import { automationUnits, unitsNeedingYou } from "@/lib/automationUnits";
import { Dropdown } from "../ui/Dropdown";
import { Tooltip } from "../ui/Tooltip";

const TAB_ICONS: Record<WorkflowsTab, typeof Library> = {
  library: Library,
  runs: Activity,
  automations: CalendarClock,
};

/** Pages that host a transcript own their height and scrolling. */
function isFillLayout(pathname: string): boolean {
  return pathname.startsWith("/workflows/runs/");
}

export function WorkflowsLayout() {
  const { pathname } = useLocation();
  useWorkflowsProjectResolution();
  return (
    <WorkflowsShell pathname={pathname}>
      <Outlet />
    </WorkflowsShell>
  );
}

/**
 * The chrome alone, with no router-driven project resolution: what the
 * layout renders, and what a test can mount around a single page.
 */
export function WorkflowsShell({ pathname, children }: { pathname: string; children: ReactNode }) {
  const navigate = useNavigate();
  const activeTab = workflowsTabForPath(pathname);
  // The project rides along between sections, so switching keeps context.
  const search = useSearch({ strict: false }) as { project?: string };
  const { trafficLightPadding, dragRegionStyle } = useTitleBarChrome({ collapsedPadding: "0px" });
  const [collapsed, setCollapsed] = useState(false);
  const narrow = useNarrowViewport();
  // A 56px rail cannot contain the native macOS traffic-light inset. Keep
  // collapse available, but put its exit in the main header in that case.
  const railExitInHeader = !narrow && collapsed && trafficLightPadding !== "0px";

  // The button always leaves the area. Escape steps to the logical parent: a
  // detail page's list, or out of the area from a list.
  const leave = useCallback(() => void navigate({ to: "/", search: {} }), [navigate]);
  const stepBack = useCallback(() => void navigate(getParentRouteNavigateOptions(pathname)), [navigate, pathname]);
  useEscapeToLeave(stepBack);

  // Automations carries how many automations need the user, in every project:
  // the same set its "Needs you" lists.
  const triggers = useTriggers();
  const needYou = useMemo(() => unitsNeedingYou(automationUnits(triggers.data ?? [])).length, [triggers.data]);

  const navItems = useMemo(() => {
    const query = search.project ? `?${new URLSearchParams({ project: search.project }).toString()}` : "";
    return WORKFLOWS_TABS.map((tab) => {
      const Icon = TAB_ICONS[tab.key];
      return {
        label: tab.label,
        href: `${tab.path}${query}`,
        active: tab.key === activeTab,
        icon: <Icon className="h-4 w-4" aria-hidden="true" />,
        badge:
          tab.key === "automations" && needYou > 0
            ? ({ type: "count", value: needYou, tone: "danger", label: `${needYou} ${needYou === 1 ? "automation needs" : "automations need"} you` } as const)
            : undefined,
      };
    });
  }, [activeTab, search.project, needYou]);

  return (
    <div className="h-screen w-full" data-testid="workflows-shell">
      <SidebarLayout
        navLabel="Workflows"
        // `forge-ui` on the sidebar only: inside that scope `accent` is the
        // primary action colour (the active nav item). Scoping the whole page
        // would also repaint every reliant Button's accent hover.
        sidebarClassName="forge-ui"
        brand={
          <div
            className="flex min-w-0 items-center gap-1 transition-[padding] duration-200 ease-in-out"
            style={{ paddingLeft: trafficLightPadding, ...dragRegionStyle }}
            data-testid="workflows-brand"
          >
            <WorkflowsCloseButton onClose={leave} />
            <span className="truncate text-sm font-semibold tracking-tight text-foreground">Workflows</span>
          </div>
        }
        brandCollapsed={
          <div data-testid="workflows-brand-collapsed">
            {!railExitInHeader && <WorkflowsCloseButton onClose={leave} />}
          </div>
        }
        navItems={navItems}
        collapsed={collapsed}
        onToggle={() => setCollapsed((c) => !c)}
        // Automations always span every project, so a project scope there
        // would claim a filter that is not applied.
        headerContent={
          <WorkflowsHeader
            onClose={leave}
            activeTab={activeTab}
            showExit={narrow || railExitInHeader}
          />
        }
        contentLayout={isFillLayout(pathname) ? "fill" : "padded"}
      >
        {isFillLayout(pathname) ? children : <div className="mx-auto w-full max-w-6xl">{children}</div>}
      </SidebarLayout>
    </div>
  );
}

/**
 * Escape leaves, matching SettingsPage — except in a field, or while a dialog
 * or menu is open: that Escape is theirs.
 *
 * The open-overlay check runs in the CAPTURE phase, before anything handles
 * the key. ui/Dropdown closes its menu on a document-level keydown, which
 * runs before a window bubble listener — so a bubble-phase check saw the menu
 * already gone, and one Escape closed the filter menu AND left the area.
 */
function useEscapeToLeave(onLeave: () => void) {
  useEffect(() => {
    let overlayOwnsEscape = false;
    const onCapture = (event: KeyboardEvent) => {
      if (event.key !== "Escape") return;
      overlayOwnsEscape = Boolean(
        document.querySelector('[aria-modal="true"], [role="menu"], [role="listbox"], [role="dialog"]'),
      );
    };
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || event.defaultPrevented || overlayOwnsEscape) return;
      const target = event.target as HTMLElement | null;
      if (
        target?.tagName === "INPUT" ||
        target?.tagName === "TEXTAREA" ||
        target?.tagName === "SELECT" ||
        target?.isContentEditable
      ) {
        return;
      }
      event.preventDefault();
      onLeave();
    };
    window.addEventListener("keydown", onCapture, true);
    window.addEventListener("keydown", onKeyDown);
    return () => {
      window.removeEventListener("keydown", onCapture, true);
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [onLeave]);
}

/** The console pattern: a quiet exit in the brand row, also on the rail. */
function WorkflowsCloseButton({ onClose }: { onClose: () => void }) {
  const { isElectron, noDragRegionStyle } = useTitleBarChrome({ alignedToWindowEdge: false });
  const exitLabel = isElectron ? "Close workflows" : "Back to app";
  return (
    <Tooltip content={`${exitLabel} (Esc)`} placement="bottom" delay={300}>
      <button
        type="button"
        onClick={onClose}
        style={noDragRegionStyle}
        aria-label={exitLabel}
        data-testid="workflows-close"
        className="inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-muted-foreground transition-colors hover:bg-muted/70 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
      >
        {isElectron ? <X className="h-4 w-4" aria-hidden="true" /> : <ArrowLeft className="h-4 w-4" aria-hidden="true" />}
      </button>
    </Tooltip>
  );
}

/** Breadcrumb and scope, leaving the way out on the left, never the right. */
function WorkflowsHeader({ onClose, activeTab, showExit }: {
  onClose: () => void;
  activeTab: WorkflowsTab | null;
  showExit: boolean;
}) {
  const { isElectron, trafficLightPadding, dragRegionStyle, noDragRegionStyle } = useTitleBarChrome({ collapsedPadding: "0px" });
  // The shell can remain mounted briefly while navigating out of the area.
  const section = WORKFLOWS_TABS.find((tab) => tab.key === activeTab);
  return (
    <div
      className={cn("flex min-w-0 flex-1 items-center gap-3", isElectron && "cursor-move")}
      // With no brand exit visible (mobile drawer or native macOS rail),
      // leave room for traffic lights before the leading header exit.
      style={{ ...dragRegionStyle, paddingLeft: showExit ? trafficLightPadding : undefined }}
    >
      {showExit && <WorkflowsCloseButton onClose={onClose} />}
      <nav aria-label="Breadcrumb" className="flex min-w-0 flex-1 items-center gap-1.5 text-sm text-muted-foreground">
        <span className="hidden md:inline">Workflows</span>
        <span aria-hidden="true" className="hidden md:inline">›</span>
        <span aria-current="page" className="truncate font-medium text-foreground">{section?.label ?? "Workflows"}</span>
      </nav>
      <div className="flex min-w-0 max-w-xs cursor-default items-center max-md:max-w-48" style={noDragRegionStyle}>
        {activeTab !== "automations" ? <ProjectSwitcher /> : <span className="whitespace-nowrap text-sm text-muted-foreground">All projects</span>}
      </div>
    </div>
  );
}

/** Which project the Library and Runs read. Selecting one updates `?project=` (the layout's sync effect). */
function ProjectSwitcher() {
  const currentProject = useProjectStore((state) => state.currentProject);
  const projects = useProjectStore((state) => state.projects);
  const loadProjects = useProjectStore((state) => state.loadProjects);
  const selectProject = useProjectStore((state) => state.selectProject);
  const [open, setOpen] = useState(false);

  const onOpenChange = (next: boolean) => {
    setOpen(next);
    if (next && projects.length === 0) void loadProjects().catch(() => undefined);
  };

  return (
    <Dropdown
      isOpen={open}
      onOpenChange={onOpenChange}
      variant="form"
      contentClassName="max-h-80 w-72 overflow-y-auto border p-1"
      trigger={
        <button
          type="button"
          onClick={() => onOpenChange(!open)}
          aria-haspopup="menu"
          aria-expanded={open}
          data-testid="workflows-project-switcher"
          className="inline-flex h-8 max-w-[18rem] items-center gap-2 rounded-md border border-border px-2.5 text-sm text-foreground transition-colors hover:border-muted-foreground/50 focus:outline-none focus-visible:ring-2 focus-visible:ring-ring/40"
        >
          <span className="text-muted-foreground">Project</span>
          <span className="truncate font-medium">{currentProject?.name ?? "Choose…"}</span>
          <ChevronDown className="h-3.5 w-3.5 shrink-0 text-muted-foreground" aria-hidden="true" />
        </button>
      }
    >
      <div role="menu" aria-label="Projects">
        {projects.length === 0 ? (
          <p className="px-3 py-2 text-sm text-muted-foreground">No projects yet.</p>
        ) : (
          projects.map((project) => {
            const active = project.id === currentProject?.id;
            return (
              <button
                key={project.id}
                type="button"
                role="menuitemradio"
                aria-checked={active}
                onClick={() => {
                  setOpen(false);
                  if (!active) void selectProject(project);
                }}
                className="flex w-full items-center gap-2 rounded-md px-2.5 py-1.5 text-left text-sm text-foreground transition-colors hover:bg-muted/70 focus:bg-muted/70 focus:outline-none"
              >
                <Check className={cn("h-3.5 w-3.5 shrink-0", !active && "invisible")} aria-hidden="true" />
                <span className="truncate">{project.name}</span>
              </button>
            );
          })
        )}
      </div>
    </Dropdown>
  );
}

/** The resolution ladder in the header comment. Runs once per mount of the area. */
function useWorkflowsProjectResolution() {
  const { pathname } = useLocation();
  useRouteProjectResolution({ reflectInUrl: pathname.startsWith(WORKFLOWS_AREA_PATH) });
}
