// Copyright (c) 2025 Reliant Labs

/**
 * DEV-ONLY proving harness for the console v3 PROTOTYPE. Not a product
 * surface; no backend, fabricated data only.
 *
 *   /forge-console-v3-preview?page=workloads|workload|releases|release|activity|observability&dark=1&scheme=<scheme>
 *
 * Every state is a URL so a screenshot is reproducible. See
 * research/console-v3/PROPOSAL.md for what the prototype proposes.
 */

import { useEffect, useState } from "react";

import { ConsoleShell, type NavId } from "../ConsoleShell";
import { ObservabilityHomePage } from "../ObservabilityHome";
import { ActivityPage, ReleaseDetailPage, ReleaseListPage } from "../ReleasePages";
import { WorkloadDetailPage, WorkloadListPage } from "../WorkloadPages";

type Page = "workloads" | "workload" | "releases" | "release" | "activity" | "observability";

const NAV_OF: Record<Page, NavId> = {
  workloads: "workloads",
  workload: "workloads",
  releases: "releases",
  release: "releases",
  activity: "activity",
  observability: "obs-overview",
};

export default function ConsoleV3Preview() {
  const params = new URLSearchParams(window.location.search);
  const [page, setPage] = useState<Page>((params.get("page") as Page) || "workload");
  const dark = params.get("dark") === "1";
  const scheme = params.get("scheme");

  useEffect(() => {
    document.documentElement.classList.toggle("dark", dark);
    if (scheme) document.documentElement.setAttribute("data-color-scheme", scheme);
  }, [dark, scheme]);

  const env = page === "workload" || page === "workloads" ? "prod" : "all";
  const go = (id: NavId) => {
    const map: Partial<Record<NavId, Page>> = { workloads: "workloads", releases: "releases", activity: "activity", "obs-overview": "observability" };
    if (map[id]) setPage(map[id]!);
  };

  return (
    <ConsoleShell active={NAV_OF[page]} env={env} onNavigate={go}>
      {page === "workloads" && <WorkloadListPage onOpen={() => setPage("workload")} />}
      {page === "workload" && <WorkloadDetailPage />}
      {page === "releases" && <ReleaseListPage onOpen={() => setPage("release")} />}
      {page === "release" && <ReleaseDetailPage />}
      {page === "activity" && <ActivityPage />}
      {page === "observability" && <ObservabilityHomePage />}
    </ConsoleShell>
  );
}
