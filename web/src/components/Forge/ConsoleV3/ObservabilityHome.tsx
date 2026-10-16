// Copyright (c) 2025 Reliant Labs

/**
 * Console v3 PROTOTYPE: the observability home. One screen that answers
 * "is anything wrong, and where" — then every row links into the resource
 * (workload detail) or the explorer (logs/traces) pre-filtered.
 */

import { ArrowUpRight } from "lucide-react";

import { Button, Dot, FilterChip, PageHead, Panel, Spark, Status, Table, Td, Th, Tr } from "./primitives";
import { RELEASES, series } from "./fixtures";

const SERVICES = [
  { name: "admin-server", rps: "38.2/s", err: "0.4%", p95: "86ms", tone: "success" as const, d: series(3, 40, 38, 14) },
  { name: "reliant-api-server", rps: "112/s", err: "0.2%", p95: "64ms", tone: "success" as const, d: series(9, 40, 110, 30) },
  { name: "reliant-temporal-worker", rps: "6.1/s", err: "4.8%", p95: "1.9s", tone: "danger" as const, d: series(13, 40, 6, 4, 30) },
  { name: "daemon-gateway", rps: "22/s", err: "0.1%", p95: "12ms", tone: "success" as const, d: series(17, 40, 22, 6) },
  { name: "workspace-proxy", rps: "51/s", err: "0.0%", p95: "8ms", tone: "success" as const, d: series(21, 40, 50, 10) },
];

export function ObservabilityHomePage() {
  return (
    <div className="pb-10">
      <PageHead
        title="Observability"
        meta={<span>All environments · last 1 hour</span>}
        actions={
          <>
            <FilterChip label="Time" value="Last 1h" />
            <FilterChip label="Group by" value="Environment" />
            <Button>Save as code</Button>
          </>
        }
      />

      <div className="mx-8 mt-5 grid grid-cols-4 gap-px overflow-hidden rounded-lg border border-border bg-border">
        {[
          { k: "Requests", v: "229/s", s: "+4% vs prev hour", d: series(1, 40, 229, 40) },
          { k: "Error rate", v: "0.9%", s: "1 service above 2%", d: series(2, 40, 0.9, 0.8, 30), tone: "destructive" as const },
          { k: "p95 latency", v: "92ms", s: "steady", d: series(4, 40, 92, 20) },
          { k: "Firing alerts", v: "1", s: "WorkerErrorRate · prod", d: series(6, 40, 0.3, 0.6, 30), tone: "warning" as const },
        ].map((t) => (
          <div key={t.k} className="bg-card px-4 pb-2 pt-3">
            <div className="text-sm text-muted-foreground">{t.k}</div>
            <div className="mt-0.5 text-xl font-semibold tabular-nums text-foreground">{t.v}</div>
            <div className="text-xs text-muted-foreground">{t.s}</div>
            <div className="mt-2">
              <Spark data={t.d} height={36} tone={t.tone} markers={[{ at: 22, label: "v1.7.45" }]} />
            </div>
          </div>
        ))}
      </div>

      <div className="grid grid-cols-1 gap-4 px-8 pt-4 xl:grid-cols-[1fr_340px]">
        <Panel title="Services" description="Sorted by what needs attention. Click a row for the workload." flush>
          <Table>
            <thead>
              <tr>
                <Th>Service</Th>
                <Th>Health</Th>
                <Th align="right">Requests</Th>
                <Th align="right">Errors</Th>
                <Th align="right">p95</Th>
                <Th className="w-48">Last hour</Th>
              </tr>
            </thead>
            <tbody>
              {[...SERVICES].sort((a, b) => (a.tone === "danger" ? -1 : b.tone === "danger" ? 1 : 0)).map((s) => (
                <Tr key={s.name} onClick={() => undefined}>
                  <Td className="font-medium">{s.name}</Td>
                  <Td>
                    <Status tone={s.tone}>{s.tone === "danger" ? "Error rate high" : "Healthy"}</Status>
                  </Td>
                  <Td align="right">{s.rps}</Td>
                  <Td align="right" className={s.tone === "danger" ? "text-destructive-ink" : ""}>
                    {s.err}
                  </Td>
                  <Td align="right">{s.p95}</Td>
                  <Td>
                    <Spark data={s.d} height={24} tone={s.tone === "danger" ? "destructive" : "primary"} />
                  </Td>
                </Tr>
              ))}
            </tbody>
          </Table>
        </Panel>
        <div className="space-y-4">
          <Panel title="Recent releases" actions={<a className="text-xs text-muted-foreground hover:text-foreground">Delivery</a>}>
            <ul className="space-y-3">
              {RELEASES.slice(0, 3).map((r) => (
                <li key={r.version} className="flex items-center gap-2 text-sm">
                  <Dot tone={r.rolledBack ? "danger" : "success"} />
                  <span className="font-medium">{r.version}</span>
                  <span className="truncate text-xs text-muted-foreground">{r.live.concat(r.was).join(", ") || "not promoted"}</span>
                  <span className="ml-auto text-xs text-muted-foreground">{r.ago}</span>
                </li>
              ))}
            </ul>
          </Panel>
          <Panel title="Explore">
            <div className="space-y-1">
              {["Search logs", "Find slow traces", "Compare CPU profiles", "Your dashboards (3)"].map((l) => (
                <a key={l} className="flex h-8 items-center justify-between rounded-md px-2 text-sm text-foreground hover:bg-muted">
                  {l} <ArrowUpRight className="h-3.5 w-3.5 text-muted-foreground" />
                </a>
              ))}
            </div>
          </Panel>
        </div>
      </div>
    </div>
  );
}
