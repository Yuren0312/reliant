// Copyright (c) 2025 Reliant Labs

/**
 * Console v3 PROTOTYPE: releases list, release detail and the activity log,
 * against fabricated data. See research/console-v3/PROPOSAL.md.
 *
 * The releases list answers, per row: what is it (version, commit message,
 * author), is it good (checks), where is it (env columns: live / was), and
 * what can I do (promote, roll back) — the four things today's card of
 * image chips does not.
 */

import { ArrowRight, ChevronRight, Download, GitCommit, MoreHorizontal, Rocket, RotateCcw } from "lucide-react";

import { cn } from "@/lib/utils";

import {
  Avatar,
  Button,
  Dot,
  Facts,
  FilterChip,
  Mono,
  PageHead,
  Panel,
  Status,
  Table,
  Tabs,
  Tag,
  Td,
  Th,
  Tr,
} from "./primitives";
import {
  ACTIVITY,
  ENVS,
  RELEASES,
  RELEASE_COMMITS,
  RELEASE_IMAGES,
  RELEASE_OBJECTS,
  RELEASE_TIMELINE,
} from "./fixtures";

const PIPELINE = ["e2e", "dev-k8s", "prod"];

export function ReleaseListPage({ onOpen }: { onOpen?: (v: string) => void }) {
  return (
    <div className="pb-10">
      <PageHead
        title="Releases"
        meta={<span>Every release cut for control-plane, and where each one runs.</span>}
        actions={<Button variant="primary" icon={<Rocket className="h-3.5 w-3.5" />}>Promote…</Button>}
      />

      {/* Where each env is right now: the pipeline strip. */}
      <div className="mx-8 mt-5 grid grid-cols-4 gap-px overflow-hidden rounded-lg border border-border bg-border">
        {ENVS.map((e) => (
          <div key={e.name} className="bg-card px-4 py-3">
            <div className="flex items-center gap-2 text-sm">
              <Dot tone={e.tone} />
              <span className="font-medium text-foreground">{e.name}</span>
              <span className="text-xs text-muted-foreground">{e.kind === "local" ? "local" : ""}</span>
            </div>
            <div className="mt-1 flex items-baseline gap-2">
              <span className="text-base font-semibold tabular-nums text-foreground">{e.version}</span>
              {e.behindBy > 0 && <span className="text-xs text-warning-ink">{e.behindBy} behind</span>}
            </div>
            <div className="mt-0.5 text-xs text-muted-foreground">
              {e.promotedAgo} · {e.promotedBy}
            </div>
          </div>
        ))}
      </div>

      <div className="mt-5 flex items-center gap-2 px-8">
        <div className="flex h-8 w-64 items-center rounded-md border border-border bg-card px-2.5 text-sm text-muted-foreground">Search version, commit, author…</div>
        <FilterChip label="Environment" value="Any" />
        <FilterChip label="Author" value="Anyone" />
        <FilterChip label="Status" value="Any" />
      </div>

      <div className="mx-8 mt-4 rounded-lg border border-border bg-card">
        <Table>
          <thead>
            <tr>
              <Th className="w-[44%]">Release</Th>
              <Th>Checks</Th>
              {PIPELINE.map((p) => (
                <Th key={p} className="w-20 text-center">
                  {p}
                </Th>
              ))}
              <Th>Cut</Th>
              <Th className="w-10" />
            </tr>
          </thead>
          <tbody>
            {RELEASES.map((r) => (
              <Tr key={r.version} onClick={() => onOpen?.(r.version)}>
                <Td className="whitespace-normal py-2.5">
                  <div className="flex items-center gap-2">
                    <span className="font-medium text-foreground">{r.version}</span>
                    {r.rolledBack && <Tag tone="danger">rolled back</Tag>}
                    {r.scoped && <Tag>scoped: {r.scoped}</Tag>}
                  </div>
                  <div className="mt-0.5 flex min-w-0 items-center gap-2 text-xs text-muted-foreground">
                    <GitCommit className="h-3 w-3 shrink-0" />
                    <Mono value={r.sha} keep={8} copy={false} />
                    <span className="truncate">{r.message}</span>
                  </div>
                </Td>
                <Td>
                  <Status tone={r.checks.passed === r.checks.total ? "success" : "danger"}>
                    <span className="tabular-nums">
                      {r.checks.passed}/{r.checks.total}
                    </span>
                  </Status>
                </Td>
                {PIPELINE.map((p) => (
                  <Td key={p} className="text-center">
                    <EnvCell state={r.live.includes(p) ? "live" : r.was.includes(p) ? "was" : "never"} />
                  </Td>
                ))}
                <Td className="text-muted-foreground">
                  <span className="flex items-center gap-2">
                    <Avatar name={r.author} />
                    {r.author} · {r.ago}
                  </span>
                </Td>
                <Td>
                  <button type="button" aria-label={`Actions for ${r.version}`} className="rounded p-1 text-muted-foreground hover:bg-muted hover:text-foreground">
                    <MoreHorizontal className="h-4 w-4" />
                  </button>
                </Td>
              </Tr>
            ))}
          </tbody>
        </Table>
        <div className="flex items-center justify-between border-t border-border px-4 py-2.5 text-xs text-muted-foreground">
          <span>7 of 112 releases</span>
          <span className="flex items-center gap-3">
            <span className="flex items-center gap-1.5">
              <span className="h-2 w-2 rounded-full bg-success" /> running
            </span>
            <span className="flex items-center gap-1.5">
              <span className="h-2 w-2 rounded-full border border-muted-foreground/60" /> ran before
            </span>
            <span>Load more</span>
          </span>
        </div>
      </div>
    </div>
  );
}

function EnvCell({ state }: { state: "live" | "was" | "never" }) {
  if (state === "live")
    return (
      <span className="inline-flex items-center gap-1.5 text-xs font-medium text-success-ink">
        <span className="h-2 w-2 rounded-full bg-success" /> live
      </span>
    );
  if (state === "was") return <span className="mx-auto inline-block h-2 w-2 rounded-full border border-muted-foreground/60" aria-label="ran before" />;
  return <span className="text-muted-foreground/50">·</span>;
}

export function ReleaseDetailPage() {
  return (
    <div className="pb-10">
      <PageHead
        crumbs={["Releases", "v1.7.44"]}
        title="v1.7.44"
        status={<Status tone="success">Live on prod</Status>}
        meta={
          <>
            <span className="flex items-center gap-1.5">
              <Avatar name="sean" /> sean cut this 6h ago from <span className="text-foreground">main</span>
            </span>
            <Mono value="571f17d5e2a94c1b" keep={8} />
            <span>ci:full 25/25</span>
            <Tag>scoped deploy: admin-server</Tag>
          </>
        }
        actions={
          <>
            <Button icon={<RotateCcw className="h-3.5 w-3.5" />} variant="danger">
              Roll back prod…
            </Button>
            <Button variant="primary" icon={<Rocket className="h-3.5 w-3.5" />}>
              Promote…
            </Button>
          </>
        }
      />
      <Tabs
        tabs={[
          { id: "changes", label: "Changes" },
          { id: "rollout", label: "Rollout" },
          { id: "checks", label: "Checks", count: 25 },
          { id: "artifacts", label: "Artifacts" },
          { id: "observability", label: "Health after release" },
        ]}
        active="changes"
      />
      <div className="grid grid-cols-1 gap-4 px-8 pt-5 xl:grid-cols-[1fr_340px]">
        <div className="min-w-0 space-y-4">
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            Compared with
            <FilterChip label="" value="v1.7.43 (previous on prod)" />
            <span className="ml-auto">3 commits · 1 image · 3 objects</span>
          </div>

          <Panel title="Commits" flush>
            <ul>
              {RELEASE_COMMITS.map((c) => (
                <li key={c.sha} className="flex h-10 items-center gap-3 border-t border-border/70 px-4 text-sm first:border-0 hover:bg-muted/60">
                  <Mono value={c.sha} keep={8} />
                  <span className="truncate text-foreground">{c.message}</span>
                  <span className="ml-auto flex shrink-0 items-center gap-2 text-xs text-muted-foreground">
                    <Avatar name={c.author} /> {c.ago}
                  </span>
                </li>
              ))}
            </ul>
          </Panel>

          <Panel title="Images" description="Only changed images are expanded; unchanged ones collapse." flush>
            <Table>
              <tbody>
                {RELEASE_IMAGES.map((i) => (
                  <Tr key={i.name}>
                    <Td className="font-medium">{i.name}</Td>
                    <Td>
                      {i.changed ? (
                        <span className="flex items-center gap-2">
                          <Mono value={i.from} keep={19} copy={false} />
                          <ArrowRight className="h-3 w-3 text-muted-foreground" />
                          <Mono value={i.to} keep={19} />
                        </span>
                      ) : (
                        <span className="text-xs text-muted-foreground">unchanged</span>
                      )}
                    </Td>
                    <Td align="right">{i.changed ? <Tag tone="accent">changed</Tag> : null}</Td>
                  </Tr>
                ))}
              </tbody>
            </Table>
          </Panel>

          <Panel title="Kubernetes objects" flush>
            <Table>
              <tbody>
                {RELEASE_OBJECTS.map((o) => (
                  <Tr key={o.name} onClick={() => undefined}>
                    <Td className="w-24">
                      <span className={cn("text-xs font-medium", o.change === "added" ? "text-success-ink" : "text-warning-ink")}>{o.change}</span>
                    </Td>
                    <Td className="w-28 text-muted-foreground">{o.kind}</Td>
                    <Td className="font-medium">{o.name}</Td>
                    <Td className="text-muted-foreground">{o.detail}</Td>
                    <Td align="right">
                      <ChevronRight className="ml-auto h-4 w-4 text-muted-foreground" />
                    </Td>
                  </Tr>
                ))}
              </tbody>
            </Table>
          </Panel>

          <Panel title="Deploy note">
            <p className="text-sm text-muted-foreground">session G3: main 571f17d5 (ci:full 25/25). Hotfix for admin-server OOM; scoped to admin-server.</p>
          </Panel>
        </div>

        <div className="space-y-4">
          <Panel title="Promotion timeline">
            <ol className="relative space-y-4 border-l border-border pl-4">
              {RELEASE_TIMELINE.map((t, i) => (
                <li key={i} className="relative">
                  <Dot tone={t.tone} className="absolute -left-[21px] top-1.5 ring-4 ring-card" />
                  <div className="text-sm text-foreground">{t.what}</div>
                  <div className="mt-0.5 text-xs text-muted-foreground">
                    {t.who} · {t.when}
                  </div>
                </li>
              ))}
            </ol>
          </Panel>
          <Panel title="Details">
            <Facts
              rows={[
                ["Version", "v1.7.44"],
                ["Source", "github.com/reliant-labs/control-plane"],
                ["Branch", "main · clean"],
                ["Forge", "v0.1.52"],
                ["Bundle", <Mono key="b" value="sha256:3f0e19aa7c4d2b6e" keep={19} />],
                ["Lead time", "38m commit → prod"],
              ]}
            />
          </Panel>
          <Panel title="Artifacts">
            <Button icon={<Download className="h-3.5 w-3.5" />}>Download bundle</Button>
          </Panel>
        </div>
      </div>
    </div>
  );
}

const KIND_LINK: Record<string, string> = {
  release: "Release",
  workload: "Workload",
  secret: "Secret",
  alert: "Alert",
  domain: "Domain",
  object: "Object",
};

export function ActivityPage() {
  const days = Array.from(new Set(ACTIVITY.map((a) => a.day)));
  return (
    <div className="pb-10">
      <PageHead title="Activity" meta={<span>Who did what, to what, and when — people, CI and Reliant itself.</span>} actions={<Button icon={<Download className="h-3.5 w-3.5" />}>Export</Button>} />
      <div className="mt-5 flex flex-wrap items-center gap-2 px-8">
        <div className="flex h-8 w-72 items-center rounded-md border border-border bg-card px-2.5 text-sm text-muted-foreground">
          <span className="text-foreground">actor:sean</span>&nbsp;Search activity…
        </div>
        <FilterChip label="Actor" value="Anyone" />
        <FilterChip label="Action" value="Any" />
        <FilterChip label="Target" value="Any" />
        <FilterChip label="Via" value="Any" />
        <FilterChip label="Time" value="Last 7 days" active />
      </div>
      <div className="mx-8 mt-4 rounded-lg border border-border bg-card">
        {days.map((day) => (
          <div key={day}>
            <div className="sticky top-0 z-10 border-b border-border bg-card px-4 py-2 text-xs font-medium text-muted-foreground first:rounded-t-lg">{day}</div>
            <ul>
              {ACTIVITY.filter((a) => a.day === day).map((a, i) => (
                <li key={i} className="group flex h-11 cursor-pointer items-center gap-3 border-b border-border/70 px-4 text-sm last:border-0 hover:bg-muted/60">
                  <span className="w-11 shrink-0 tabular-nums text-xs text-muted-foreground">{a.time}</span>
                  <Dot tone={a.tone} />
                  <Avatar name={a.actor} system={a.system} />
                  <span className="min-w-0 truncate">
                    <span className="font-medium text-foreground">{a.actor}</span> <span className="text-muted-foreground">{a.verb}</span>{" "}
                    <span className="font-medium text-foreground underline-offset-2 group-hover:underline">{a.target}</span>
                  </span>
                  <span className="ml-auto flex shrink-0 items-center gap-2">
                    {a.env && <Tag>{a.env}</Tag>}
                    <span className="w-16 text-right text-xs text-muted-foreground">{KIND_LINK[a.targetKind]}</span>
                    <span className="w-14 text-right text-xs text-muted-foreground">{a.via}</span>
                    <ChevronRight className="h-4 w-4 text-muted-foreground opacity-0 group-hover:opacity-100" />
                  </span>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </div>
  );
}
