// Copyright (c) 2025 Reliant Labs

/**
 * The Workflows area's routes (WORKFLOW_UI.md §1.3), mounted from the SAME
 * definitions routes.tsx uses (createWorkflowsAreaRoutes) with stub pages:
 *
 *   - every area route renders inside the shell, with the right tab active;
 *   - /workflows lands on the Library;
 *   - every retired path (/workflow, /runs*, /automations*) redirects to its
 *     new home and keeps its search params;
 *   - the builder's /workflow/$workflowName is NOT swallowed by the hub
 *     redirect.
 */

import type { CSSProperties } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import {
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
  useLocation,
} from "@tanstack/react-router";

const chrome = vi.hoisted(() => ({ isElectron: false, trafficLightPadding: "0px" }));
vi.mock("@/hooks/useTitleBarChrome", () => ({
  useTitleBarChrome: () => ({
    ...chrome,
    dragRegionStyle: chrome.isElectron ? { WebkitAppRegion: "drag" } : {},
    noDragRegionStyle: chrome.isElectron ? { WebkitAppRegion: "no-drag" } : {},
  }),
}));

beforeEach(() => {
  chrome.isElectron = false;
  chrome.trafficLightPadding = "0px";
});
afterEach(() => vi.unstubAllGlobals());

// The Automations badge counts what needs the user; by default nothing does.
const badge = vi.hoisted(() => ({ triggers: [] as unknown[] }));
vi.mock("@/hooks/trigger-queries", () => ({ useTriggers: () => ({ data: badge.triggers }) }));

// The shell resolves a project on mount; here one is already current.
vi.mock("@/store/projectStore", () => {
  // One stable state object, as a real store returns between updates.
  const state = {
    currentProject: { id: "proj-1", name: "Reliant" },
    projects: [{ id: "proj-1", name: "Reliant" }],
    loadProjects: vi.fn(async () => undefined),
    selectProject: vi.fn(async () => undefined),
    restoreLastProject: vi.fn(async () => false),
  };
  const snapshot = () => state;
  const useProjectStore = Object.assign(
    (selector?: (s: ReturnType<typeof snapshot>) => unknown) => (selector ? selector(snapshot()) : snapshot()),
    { getState: snapshot },
  );
  return { useProjectStore };
});

import { createWorkflowsAreaRoutes } from "@/workflowsAreaRoutes";
import { WorkflowsLayout } from "../WorkflowsShell";

function stub(name: string) {
  return function Stub() {
    const location = useLocation();
    return <div data-testid={`page-${name}`}>{location.pathname}</div>;
  };
}

function renderAt(path: string) {
  const rootRoute = createRootRoute({ component: () => <Outlet /> });
  const authed = createRoute({ getParentRoute: () => rootRoute, id: "_authenticated", component: () => <Outlet /> });
  const builder = createRoute({
    getParentRoute: () => authed,
    path: "/workflow/$workflowName",
    component: stub("builder"),
  });
  const home = createRoute({ getParentRoute: () => rootRoute, path: "/", component: stub("home") });
  const areaRoutes = createWorkflowsAreaRoutes(() => authed, {
    layout: WorkflowsLayout,
    library: stub("library"),
    workflowDetail: stub("workflow-detail"),
    runs: stub("runs"),
    runDetail: stub("run-detail"),
    automations: stub("automations"),
    automationDetail: stub("automation-detail"),
  });
  const router = createRouter({
    routeTree: rootRoute.addChildren([home, authed.addChildren([builder, ...areaRoutes])]),
    history: createMemoryHistory({ initialEntries: [path] }),
  });
  render(
    <QueryClientProvider client={new QueryClient()}>
      <RouterProvider router={router} />
    </QueryClientProvider>,
  );
  return router;
}

function activeTab(): string | null {
  const nav = screen.getByRole("navigation", { name: "Workflows" });
  const current = within(nav)
    .getAllByRole("link")
    .find((link) => link.getAttribute("aria-current") === "page");
  return current?.textContent ?? null;
}

describe("Workflows area routes render inside the shell with the right tab", () => {
  it.each([
    ["/workflows/library", "library", "Library"],
    ["/workflows/library/builtin%3A%2F%2Fagent", "workflow-detail", "Library"],
    ["/workflows/runs", "runs", "Runs"],
    ["/workflows/runs/chat-1", "run-detail", "Runs"],
    ["/workflows/automations", "automations", "Automations"],
    ["/workflows/automations/trig-1", "automation-detail", "Automations"],
  ])("%s → %s, %s tab active", async (path, page, tab) => {
    renderAt(path);
    expect(await screen.findByTestId(`page-${page}`)).toBeInTheDocument();
    expect(activeTab()).toBe(tab);
    // The three tabs, in the order of the jobs: find it, turn it on, watch it (O9).
    const nav = screen.getByRole("navigation", { name: "Workflows" });
    expect(within(nav).getAllByRole("link").map((link) => link.textContent)).toEqual([
      "Library",
      "Automations",
      "Runs",
    ]);
  });

  it("badges Automations with how many automations need you", async () => {
    const health = (status: string) => ({ status, consecutiveFailures: 0, consecutiveSkips: 0, lastFailureDetail: "" });
    const base = { name: "x", projectId: "proj-1", enabled: true, workflow: "triage", presets: {}, params: {}, message: "", daemonId: "", noMachine: true, notifyOnComplete: false, actingIdentity: "owner", createdAt: "", updatedAt: "", source: { kind: "unknown" } };
    badge.triggers = [
      { ...base, id: "a", workflowTrigger: "nightly", health: health("needs_setup") },
      { ...base, id: "b", workflowTrigger: "evening", health: health("healthy") },
      { ...base, id: "c", workflow: "other", health: health("broken") },
    ];
    renderAt("/workflows/library");
    const nav = await screen.findByRole("navigation", { name: "Workflows" });
    const automations = within(nav).getByRole("link", { name: /Automations/ });
    // Two automations: the triage one (its worst trigger needs setup) and the broken custom one.
    expect(automations).toHaveTextContent("2");
    badge.triggers = [];
  });

  it("/workflows lands on the Library and keeps ?project", async () => {
    const router = renderAt("/workflows?project=%22proj-1%22");
    expect(await screen.findByTestId("page-library")).toBeInTheDocument();
    expect(router.state.location.pathname).toBe("/workflows/library");
    expect(router.state.location.search).toMatchObject({ project: "proj-1" });
  });

  it("the exit always leaves the area; the sidebar is the way back to a list", async () => {
    const router = renderAt("/workflows/runs/chat-1?project=%22proj-1%22");
    await screen.findByTestId("page-run-detail");
    const runs = within(screen.getByRole("navigation", { name: "Workflows" })).getByRole("link", { name: "Runs" });
    expect(runs).toHaveAttribute("aria-current", "page");
    expect(runs).toHaveAttribute("href", "/workflows/runs?project=proj-1");
    fireEvent.click(screen.getByRole("button", { name: "Back to app" }));
    await waitFor(() => expect(router.state.location.pathname).toBe("/"));
  });

  it("Escape on a detail page steps back to its list", async () => {
    const router = renderAt("/workflows/automations/trig-1");
    await screen.findByTestId("page-automation-detail");
    fireEvent.keyDown(window, { key: "Escape" });
    await waitFor(() => expect(router.state.location.pathname).toBe("/workflows/automations"));
  });

  // A regression guard, NOT a reproduction: the original bug needs a real
  // keypress, where the browser runs a microtask checkpoint between listeners
  // and React removes the menu before the window listener looks for it. A
  // scripted dispatchEvent (jsdom, user-event) never checkpoints mid-dispatch,
  // so this passes against the bubble-phase check too. The fix — the capture
  // phase check in useEscapeToLeave — was verified in Chrome.
  it("Escape with a menu open closes only the menu, not the area", async () => {
    const router = renderAt("/workflows/library");
    await screen.findByTestId("page-library");
    fireEvent.click(screen.getByTestId("workflows-project-switcher"));
    expect(await screen.findByRole("menu", { name: "Projects" })).toBeInTheDocument();
    // As a real keypress does: dispatched at the focused element, bubbling to document and window.
    fireEvent.keyDown(document.body, { key: "Escape" });
    await waitFor(() => expect(screen.queryByRole("menu", { name: "Projects" })).toBeNull());
    expect(router.state.location.pathname).toBe("/workflows/library");
  });

  it.each([
    ["library", "Library"],
    ["runs", "Runs"],
    ["automations", "Automations"],
  ])("the %s header shows its section and honest project scope", async (path, label) => {
    renderAt(`/workflows/${path}`);
    await screen.findByTestId(`page-${path}`);
    const header = screen.getByRole("banner");
    expect(within(header).getByRole("navigation", { name: "Breadcrumb" })).toHaveTextContent(`Workflows›${label}`);
    expect(within(header).queryByTestId("workflows-close")).not.toBeInTheDocument();
    if (path === "automations") {
      expect(screen.queryByTestId("workflows-project-switcher")).not.toBeInTheDocument();
      expect(within(header).getByText("All projects")).toBeInTheDocument();
    } else {
      expect(within(header).getByTestId("workflows-project-switcher")).toHaveTextContent("Reliant");
    }
  });

  it.each([false, true])("keeps the exit in the brand row (collapsed: %s) and leaves the area", async (collapsed) => {
    const router = renderAt("/workflows/runs/chat-1?project=%22proj-1%22");
    await screen.findByTestId("page-run-detail");
    if (collapsed) fireEvent.click(screen.getByRole("button", { name: "Collapse sidebar" }));
    const brand = screen.getByTestId(collapsed ? "workflows-brand-collapsed" : "workflows-brand");
    const exit = within(brand).getByRole("button", { name: "Back to app" });
    expect(exit).toHaveAttribute("data-testid", "workflows-close");
    expect(exit.textContent).toBe("");
    expect(brand.closest("aside")).not.toBeNull();
    expect(screen.getAllByTestId("workflows-close")).toHaveLength(1);
    const nav = screen.getByRole("navigation", { name: "Workflows" });
    expect(within(nav).getByRole("link", { name: "Library" })).toHaveAttribute("href", "/workflows/library?project=proj-1");
    fireEvent.click(exit);
    await waitFor(() => expect(router.state.location.pathname).toBe("/"));
    expect(router.state.location.search).toEqual({});
  });

  it("preserves native traffic-light spacing and non-draggable Electron controls, including on the rail", async () => {
    chrome.isElectron = true;
    chrome.trafficLightPadding = "80px";
    const router = renderAt("/workflows/library");
    await screen.findByTestId("page-library");
    const brand = screen.getByTestId("workflows-brand");
    expect(brand).toHaveStyle({ paddingLeft: "80px" });
    expect(brand.style.getPropertyValue("-webkit-app-region") || (brand.style as CSSProperties).WebkitAppRegion).toBe("drag");
    const exit = within(brand).getByRole("button", { name: "Close workflows" });
    expect((exit.style as CSSProperties).WebkitAppRegion).toBe("no-drag");
    fireEvent.click(screen.getByRole("button", { name: "Collapse sidebar" }));
    expect(screen.getByRole("button", { name: "Expand sidebar" })).toBeInTheDocument();
    expect(within(screen.getByTestId("workflows-brand-collapsed")).queryByTestId("workflows-close")).toBeNull();
    const headerExit = within(screen.getByRole("banner")).getByRole("button", { name: "Close workflows" });
    expect((headerExit.style as CSSProperties).WebkitAppRegion).toBe("no-drag");
    fireEvent.click(headerExit);
    await waitFor(() => expect(router.state.location.pathname).toBe("/"));
  });

  it("offers a leading mobile exit with the drawer closed; Escape dismisses an open drawer only", async () => {
    vi.stubGlobal("matchMedia", vi.fn(() => ({
      matches: true,
      addEventListener: vi.fn(),
      removeEventListener: vi.fn(),
    })));
    const router = renderAt("/workflows/library");
    await screen.findByTestId("page-library");
    const header = screen.getByRole("banner");
    const exit = within(header).getByRole("button", { name: "Back to app" });
    const breadcrumb = within(header).getByRole("navigation", { name: "Breadcrumb" });
    expect(exit.compareDocumentPosition(breadcrumb) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
    fireEvent.click(screen.getByRole("button", { name: "Open navigation" }));
    expect(screen.getByRole("dialog", { name: "Workflows" })).toBeInTheDocument();
    fireEvent.keyDown(document.body, { key: "Escape" });
    expect(screen.queryByRole("dialog", { name: "Workflows" })).toBeNull();
    expect(router.state.location.pathname).toBe("/workflows/library");
    fireEvent.click(exit);
    await waitFor(() => expect(router.state.location.pathname).toBe("/"));
  });
});

describe("Retired paths redirect and keep their search params", () => {
  it.each([
    ["/workflow", "/workflows/library", "library"],
    ["/runs", "/workflows/runs", "runs"],
    ["/runs/chat-7", "/workflows/runs/chat-7", "run-detail"],
    ["/automations", "/workflows/automations", "automations"],
    ["/automations/trig-3", "/workflows/automations/trig-3", "automation-detail"],
  ])("%s → %s", async (from, to, page) => {
    const router = renderAt(from);
    expect(await screen.findByTestId(`page-${page}`)).toBeInTheDocument();
    expect(router.state.location.pathname).toBe(to);
  });

  it("carries the Runs filters across", async () => {
    const router = renderAt('/runs?state=%5B%22needs_you%22%5D&allProjects=true&parent=%22chat-1%22');
    await screen.findByTestId("page-runs");
    expect(router.state.location.pathname).toBe("/workflows/runs");
    expect(router.state.location.search).toMatchObject({
      state: ["needs_you"],
      allProjects: true,
      parent: "chat-1",
    });
  });

  it("carries ?tour across the hub redirect", async () => {
    const router = renderAt("/workflow?tour=%22workflow-hub%22");
    await screen.findByTestId("page-library");
    expect(router.state.location.search).toMatchObject({ tour: "workflow-hub" });
  });

  it("replaces history, so Back does not bounce through the old path", async () => {
    const router = renderAt("/automations/trig-3");
    await screen.findByTestId("page-automation-detail");
    expect(router.history.length).toBe(1);
  });

  it("leaves the builder's /workflow/$workflowName alone", async () => {
    const router = renderAt("/workflow/builtin%3A%2F%2Fagent");
    expect(await screen.findByTestId("page-builder")).toBeInTheDocument();
    await waitFor(() => expect(router.state.location.pathname).toBe("/workflow/builtin%3A%2F%2Fagent"));
  });
});
