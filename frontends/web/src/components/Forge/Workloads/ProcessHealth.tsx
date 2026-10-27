// Copyright (c) 2025 Reliant Labs

/** Landed process telemetry, retained on the single app Overview rather than a second home. */
import { useMemo } from "react";
import Link from "@/components/forge-ui/link";
import Status from "@/components/forge-ui/status";
import Tag from "@/components/forge-ui/tag";
import { useServiceOverview } from "@/hooks/observability-queries";
import type { AlertInstance } from "@/services/controlPlane/observability/alertingTypes";
import type { ObsScope, Workload } from "@/services/controlPlane/observability/types";
import { keepsLastResult } from "@/services/controlPlane/observability/quota";
import type { RangePreset } from "@/services/controlPlane/observability/window";
import { ENV_KEY } from "../Observability/attributes";
import { formatBytes, formatCores, formatDuration, formatPercent, formatRate } from "../Observability/format";
import { byAttention, serviceRows } from "../Observability/Home/homeModel";
import { ObsAsOf, ObsError, ObsLoadingRows } from "../Observability/ObsStates";
import { INK_LINK, Panel } from "../Releases/releaseParts";
import { StackedTable } from "../StackedTable";
import { worstUsage } from "./workloadModel";
import { UsageMeter } from "./workloadParts";

export function ProcessHealth({ scope, range, workloads, alerts, envNameOf, workloadHref, logsHref, navigateTo }: {
  scope: ObsScope | null;
  range: RangePreset;
  workloads: readonly Workload[];
  alerts: readonly AlertInstance[];
  envNameOf: (id: string) => string | undefined;
  workloadHref: (name: string, env: string | null) => string;
  logsHref: (name: string) => string;
  navigateTo: (href: string) => void;
}) {
  const showEnv = scope?.envIds.length === 0;
  const groupBy = useMemo(() => showEnv ? [ENV_KEY] : [], [showEnv]);
  const overview = useServiceOverview(scope, range, [], groupBy);
  const rows = useMemo(() => serviceRows(overview.data?.services ?? [], workloads, alerts, envNameOf).sort(byAttention), [overview.data, workloads, alerts, envNameOf]);
  return <Panel title="Process health" flush testId="observability-services">
    <p className="px-4 pt-3 text-sm text-ink-muted">Deployed processes (service.name), not RPC services. Open a process to investigate.</p>
    {overview.data && <div className="px-4"><ObsAsOf updatedAt={overview.dataUpdatedAt} /></div>}
    {overview.isLoading ? <ObsLoadingRows /> : overview.error && !keepsLastResult(overview.error, overview.data) ?
      <div className="p-4"><ObsError error={overview.error} subject={scope?.projectId ?? ""} onRetry={() => void overview.refetch()} /></div> : rows.length === 0 ?
      <p className="px-4 py-6 text-sm text-ink-muted" data-testid="observability-no-services">No process sent telemetry in this window.</p> :
      <StackedTable className="w-full text-sm">
        <caption className="sr-only">Services, sorted by what needs attention</caption>
        <thead><tr className="border-b border-border text-left text-xs text-ink-muted">
          {["Name", ...(showEnv ? ["Environment"] : []), "Status", "Req/s", "Errors", "p95", "CPU", "Memory", "Error logs", "Alerts"].map(label => <th key={label} scope="col" className="h-9 px-4 font-medium">{label}</th>)}
        </tr></thead>
        <tbody>{rows.map(row => <tr key={`${row.name}/${row.env ?? ""}`} data-testid={`observability-service-${row.name}${row.env ? `-${row.env}` : ""}`} data-tone={row.tone}
          className="cursor-pointer border-b border-border/70 last:border-b-0 hover:bg-ink/5"
          onClick={event => { if (!(event.target as HTMLElement).closest("a, button")) navigateTo(workloadHref(row.name, row.env)); }}>
          <td className="h-10 px-4"><Link href={workloadHref(row.name, row.env)} className="no-color font-medium text-ink">{row.name}</Link></td>
          {showEnv && <td className="h-10 px-4"><Tag>{row.env ?? "—"}</Tag></td>}
          <td className="h-10 px-4"><span title={row.reason ?? undefined}><Status tone={row.tone} label={row.status} /></span></td>
          <td className="h-10 px-4 tabular-nums">{formatRate(row.health.requestRate)}</td>
          <td className="h-10 px-4 tabular-nums">{row.health.requestCount ? formatPercent(row.health.errorRate) : "—"}</td>
          <td className="h-10 px-4 tabular-nums">{row.health.requestCount ? formatDuration(row.health.p95Ms) : "—"}</td>
          <td className="h-10 w-40 px-4">{row.workload ? <UsageMeter usage={worstUsage(row.workload, "cpu")} format={formatCores} what="CPU" /> : "—"}</td>
          <td className="h-10 w-40 px-4">{row.workload ? <UsageMeter usage={worstUsage(row.workload, "memory")} format={formatBytes} what="Memory" /> : "—"}</td>
          <td className="h-10 px-4 tabular-nums"><Link href={logsHref(row.name)} className={INK_LINK} title={`Error logs from ${row.name}`}>{row.health.errorLogCount.toLocaleString()}</Link></td>
          <td className="h-10 px-4 tabular-nums">{row.alertsFiring > 0 ? <span className="text-warning-ink">{row.alertsFiring} firing</span> : <span className="text-ink-muted">·</span>}</td>
        </tr>)}</tbody>
      </StackedTable>}
  </Panel>;
}
