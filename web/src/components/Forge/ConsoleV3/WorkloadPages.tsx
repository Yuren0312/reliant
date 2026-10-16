// Copyright (c) 2025 Reliant Labs

/**
 * Console v3 PROTOTYPE: workload list and workload detail, against
 * fabricated data. See research/console-v3/PROPOSAL.md, "Workloads".
 */

import { ArrowUpRight, ExternalLink, FileSearch, Flame, ListTree, MoreHorizontal, RotateCcw } from "lucide-react";

import {
  Button,
  Dot,
  Facts,
  FilterChip,
  Meter,
  Mono,
  PageHead,
  Panel,
  Spark,
  Status,
  Table,
  Tabs,
  Tag,
  Td,
  Th,
  Tr,
} from "./primitives";
import { DEPLOY_HISTORY, EVENTS, JOBS, LOG_LINES, PODS, WORKLOADS, series } from "./fixtures";

export function WorkloadListPage({ onOpen }: { onOpen?: (name: string) => void }) {
  const degraded = WORKLOADS.filter((w) => w.tone !== "success").length;
  return (
    <div className="pb-10">
      <PageHead
        title="Workloads"
        meta={
          <>
            <span>{WORKLOADS.length} services</span>
            <span className="flex items-center gap-1.5">
              <Dot tone="warning" /> {degraded} degraded
            </span>
            <span>{JOBS.length} jobs</span>
          </>
        }
        actions={<Button icon={<ExternalLink className="h-3.5 w-3.5" />}>Open in Observability</Button>}
      />
      <div className="mt-5 flex items-center gap-2 px-8">
        <div className="flex h-8 w-64 items-center rounded-md border border-border bg-card px-2.5 text-sm text-muted-foreground">Filter workloads…</div>
        <FilterChip label="Status" value="Any" />
        <FilterChip label="Kind" value="Services" active />
        <FilterChip label="Release" value="Any" />
      </div>

      <div className="mx-8 mt-4 rounded-lg border border-border bg-card">
        <Table>
          <thead>
            <tr>
              <Th>Name</Th>
              <Th>Status</Th>
              <Th>Ready</Th>
              <Th>Version</Th>
              <Th align="right">Restarts 24h</Th>
              <Th>CPU</Th>
              <Th>Memory</Th>
              <Th>Updated</Th>
            </tr>
          </thead>
          <tbody>
            {WORKLOADS.map((w) => (
              <Tr key={w.name} onClick={() => onOpen?.(w.name)}>
                <Td>
                  <span className="font-medium text-foreground">{w.name}</span>
                  <span className="ml-2 text-xs text-muted-foreground">{w.kind}</span>
                </Td>
                <Td>
                  <Status tone={w.tone}>{w.status}</Status>
                </Td>
                <Td className="tabular-nums text-muted-foreground">{w.ready}</Td>
                <Td>
                  <span className="text-foreground">{w.version}</span>
                </Td>
                <Td align="right">
                  <span className={w.restarts > 3 ? "text-warning-ink" : "text-muted-foreground"}>{w.restarts}</span>
                  {w.lastExit && <span className="ml-1.5 text-xs text-muted-foreground">{w.lastExit}</span>}
                </Td>
                <Td>
                  <Meter pct={w.cpu.pct} label={`${w.cpu.pct}%`} />
                </Td>
                <Td>
                  <Meter pct={w.mem.pct} label={`${w.mem.pct}%`} />
                </Td>
                <Td className="text-muted-foreground">{w.updated}</Td>
              </Tr>
            ))}
          </tbody>
        </Table>
      </div>

      <h2 className="mx-8 mt-8 text-sm font-medium text-foreground">Jobs</h2>
      <p className="mx-8 mt-0.5 text-xs text-muted-foreground">One-shot runs a release starts: migrations, provisioning.</p>
      <div className="mx-8 mt-3 rounded-lg border border-border bg-card">
        <Table>
          <thead>
            <tr>
              <Th>Name</Th>
              <Th>Result</Th>
              <Th>Release</Th>
              <Th align="right">Duration</Th>
              <Th>Finished</Th>
            </tr>
          </thead>
          <tbody>
            {JOBS.map((j) => (
              <Tr key={j.name} onClick={() => undefined}>
                <Td className="font-medium">{j.name}</Td>
                <Td>
                  <Status tone={j.tone}>{j.status}</Status>
                </Td>
                <Td>{j.release}</Td>
                <Td align="right" className="text-muted-foreground">
                  {j.duration}
                </Td>
                <Td className="text-muted-foreground">{j.finished}</Td>
              </Tr>
            ))}
          </tbody>
        </Table>
      </div>
    </div>
  );
}

const WORKLOAD_TABS = [
  { id: "overview", label: "Overview" },
  { id: "logs", label: "Logs" },
  { id: "metrics", label: "Metrics" },
  { id: "traces", label: "Traces" },
  { id: "profiles", label: "Profiles" },
  { id: "events", label: "Events", count: 4 },
  { id: "history", label: "History" },
  { id: "config", label: "Config" },
];

export function WorkloadDetailPage() {
  const w = WORKLOADS[0];
  const cpu = series(7, 60, 210, 90, 41);
  const mem = series(11, 60, 380, 60, 41);
  const rps = series(3, 60, 38, 14);
  const err = series(5, 60, 0.6, 0.8, 41);
  const markers = [{ at: 12, label: "v1.7.44" }];
  return (
    <div className="pb-10">
      <PageHead
        crumbs={["Workloads", "admin-server"]}
        title="admin-server"
        status={<Status tone="success">Healthy</Status>}
        meta={
          <>
            <Tag>prod</Tag>
            <span>Deployment · 2/2 ready</span>
            <span>
              Release <a className="text-foreground underline-offset-2 hover:underline">v1.7.44</a>
            </span>
            <Mono value={w.digest} keep={19} />
            <span>Updated 5h ago by sean</span>
          </>
        }
        actions={
          <>
            <Button icon={<FileSearch className="h-3.5 w-3.5" />}>Logs</Button>
            <Button icon={<RotateCcw className="h-3.5 w-3.5" />}>Restart</Button>
            <Button variant="ghost" icon={<MoreHorizontal className="h-4 w-4" />} className="px-2" />
          </>
        }
      />
      <Tabs tabs={WORKLOAD_TABS} active="overview" />

      <div className="grid grid-cols-1 gap-4 px-8 pt-5 xl:grid-cols-[1fr_340px]">
        <div className="min-w-0 space-y-4">
          {/* Golden signals, one row, release markers on every chart. */}
          <div className="grid grid-cols-2 gap-4">
            <ChartPanel title="Requests" value="38.2/s" sub="p95 86ms" data={rps} markers={markers} />
            <ChartPanel title="Error rate" value="0.4%" sub="12 errors in 1h" data={err} markers={markers} tone="destructive" />
            <ChartPanel title="CPU" value="210m" sub="of 500m limit · 42%" data={cpu} markers={markers} />
            <ChartPanel title="Memory" value="402Mi" sub="of 512Mi limit · 79%" data={mem} markers={markers} tone="warning" />
          </div>

          <Panel title="Pods" description="Restarts and the last reason each container exited." flush>
            <Table>
              <thead>
                <tr>
                  <Th>Pod</Th>
                  <Th>Status</Th>
                  <Th align="right">Restarts</Th>
                  <Th>Last exit</Th>
                  <Th>Node</Th>
                  <Th>Age</Th>
                </tr>
              </thead>
              <tbody>
                {PODS.map((p) => (
                  <Tr key={p.name} onClick={() => undefined}>
                    <Td>
                      <Mono value={p.name} keep={30} />
                    </Td>
                    <Td>
                      <Status tone={p.tone}>{p.status}</Status>
                    </Td>
                    <Td align="right">{p.restarts}</Td>
                    <Td className={p.lastExit ? "text-warning-ink" : "text-muted-foreground"}>{p.lastExit || "—"}</Td>
                    <Td className="text-muted-foreground">{p.node}</Td>
                    <Td className="text-muted-foreground">{p.age}</Td>
                  </Tr>
                ))}
              </tbody>
            </Table>
          </Panel>

          <Panel
            title="Live logs"
            actions={
              <a className="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground">
                Open in Logs <ArrowUpRight className="h-3 w-3" />
              </a>
            }
            flush
          >
            <div className="mx-4 mb-4 rounded-md border border-border/60 bg-background py-1.5 font-mono text-xs leading-6">
              {LOG_LINES.map((l, i) => (
                <div key={i} className="flex gap-3 px-3 hover:bg-muted/60">
                  <span className="shrink-0 text-muted-foreground">{l.t}</span>
                  <span
                    className={
                      l.level === "error" ? "w-10 shrink-0 text-destructive-ink" : l.level === "warn" ? "w-10 shrink-0 text-warning-ink" : "w-10 shrink-0 text-muted-foreground"
                    }
                  >
                    {l.level}
                  </span>
                  <span className="truncate text-foreground">{l.msg}</span>
                </div>
              ))}
            </div>
          </Panel>
        </div>

        {/* The right rail: provenance and history — "where did this come from". */}
        <div className="space-y-4">
          <Panel title="Provenance">
            <Facts
              rows={[
                ["Release", <a key="r" className="hover:underline">v1.7.44</a>],
                ["Commit", <span key="c" className="flex items-center gap-2"><Mono value="571f17d5e2a9" keep={8} /> <span className="truncate text-muted-foreground">raise memory limit…</span></span>],
                ["Image", <Mono key="i" value="control-plane@sha256:20edb404371b" keep={22} />],
                ["Deployed by", "sean · CLI · scoped"],
                ["Declared in", <span key="d" className="text-muted-foreground">deploy/kcl/prod/main.k</span>],
              ]}
            />
          </Panel>

          <Panel title="Deploy history" actions={<a className="text-xs text-muted-foreground hover:text-foreground">All</a>}>
            <ol className="relative space-y-4 border-l border-border pl-4">
              {DEPLOY_HISTORY.map((d) => (
                <li key={d.version} className="relative">
                  <Dot tone={d.tone} className="absolute -left-[21px] top-1.5 ring-4 ring-card" />
                  <div className="flex items-center gap-2 text-sm">
                    <span className="font-medium text-foreground">{d.version}</span>
                    {d.current && <Tag tone="success">running</Tag>}
                  </div>
                  <div className="mt-0.5 text-xs text-muted-foreground">
                    {d.what} · {d.who} · {d.ago}
                  </div>
                </li>
              ))}
            </ol>
          </Panel>

          <Panel title="Recent events" actions={<a className="text-xs text-muted-foreground hover:text-foreground">All 4</a>}>
            <ul className="space-y-3">
              {EVENTS.slice(0, 3).map((e, i) => (
                <li key={i} className="flex gap-2.5 text-sm">
                  <Dot tone={e.tone} className="mt-1.5" />
                  <div className="min-w-0">
                    <div className="flex items-baseline gap-2">
                      <span className="font-medium text-foreground">{e.reason}</span>
                      <span className="text-xs text-muted-foreground">{e.ago}</span>
                    </div>
                    <p className="truncate text-xs text-muted-foreground">{e.message}</p>
                  </div>
                </li>
              ))}
            </ul>
          </Panel>

          <Panel title="Jump to">
            <div className="grid grid-cols-2 gap-2">
              <Button icon={<ListTree className="h-3.5 w-3.5" />}>Traces</Button>
              <Button icon={<Flame className="h-3.5 w-3.5" />}>Profiles</Button>
            </div>
          </Panel>
        </div>
      </div>
    </div>
  );
}

function ChartPanel({
  title,
  value,
  sub,
  data,
  markers,
  tone,
}: {
  title: string;
  value: string;
  sub: string;
  data: number[];
  markers: { at: number; label: string }[];
  tone?: "primary" | "warning" | "destructive";
}) {
  return (
    <section className="rounded-lg border border-border bg-card px-4 pb-3 pt-3">
      <div className="flex items-baseline justify-between">
        <h3 className="text-sm text-muted-foreground">{title}</h3>
        <span className="text-xs text-muted-foreground">1h</span>
      </div>
      <div className="mt-1 flex items-baseline gap-2">
        <span className="text-lg font-semibold tabular-nums text-foreground">{value}</span>
        <span className="text-xs text-muted-foreground">{sub}</span>
      </div>
      <div className="mt-3">
        <Spark data={data} height={72} markers={markers} tone={tone} />
      </div>
      <div className="mt-1 flex justify-between text-2xs text-muted-foreground">
        <span>13:05</span>
        <span className="flex items-center gap-1">
          <span className="inline-block h-2 border-l border-dashed border-muted-foreground" /> v1.7.44
        </span>
        <span>14:05</span>
      </div>
    </section>
  );
}
