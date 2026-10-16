// Copyright (c) 2025 Reliant Labs

/**
 * FABRICATED data for the console v3 prototype. Not wired to any backend.
 *
 * Shapes follow what control-plane already serves where it can
 * (ObservabilityService.ListWorkloads / GetWorkload, DeployService
 * ListReleases / ListPromotions / GetRollout, AuditLogService) so the
 * prototype does not promise a page the data cannot fill. Fields marked
 * GAP in research/console-v3/PROPOSAL.md are invented here on purpose, to
 * show the owner what the page looks like once the backend serves them.
 */

export type Tone = "success" | "warning" | "danger" | "neutral" | "accent";

export interface EnvSummary {
  name: string;
  kind: "local" | "cluster" | "hosted";
  version: string;
  promotedAgo: string;
  promotedBy: string;
  tone: Tone;
  state: string;
  behindBy: number;
}

export const ENVS: EnvSummary[] = [
  { name: "dev", kind: "local", version: "working tree", promotedAgo: "running now", promotedBy: "sean", tone: "success", state: "Running", behindBy: 0 },
  { name: "e2e", kind: "cluster", version: "v1.7.45", promotedAgo: "38m ago", promotedBy: "ci", tone: "success", state: "Converged", behindBy: 0 },
  { name: "dev-k8s", kind: "cluster", version: "v1.7.41", promotedAgo: "3d ago", promotedBy: "sean", tone: "neutral", state: "Converged", behindBy: 4 },
  { name: "prod", kind: "cluster", version: "v1.7.44", promotedAgo: "5h ago", promotedBy: "sean", tone: "success", state: "Converged", behindBy: 1 },
];

// ── Workloads ──────────────────────────────────────────────────────────────

export interface WorkloadRow {
  name: string;
  env: string;
  kind: "Deployment" | "StatefulSet";
  tone: Tone;
  status: string;
  ready: string;
  version: string;
  digest: string;
  restarts: number;
  lastExit?: string;
  cpu: { used: string; limit: string; pct: number };
  mem: { used: string; limit: string; pct: number };
  updated: string;
}

export const WORKLOADS: WorkloadRow[] = [
  { name: "admin-server", env: "prod", kind: "Deployment", tone: "success", status: "Healthy", ready: "2/2", version: "v1.7.44", digest: "sha256:20edb404371b9a1f", restarts: 1, lastExit: "OOMKilled", cpu: { used: "210m", limit: "500m", pct: 42 }, mem: { used: "402Mi", limit: "512Mi", pct: 79 }, updated: "5h ago" },
  { name: "admin-api", env: "prod", kind: "Deployment", tone: "success", status: "Healthy", ready: "1/1", version: "v1.7.43", digest: "sha256:8a54465b8d71c0e2", restarts: 0, cpu: { used: "40m", limit: "250m", pct: 16 }, mem: { used: "96Mi", limit: "256Mi", pct: 38 }, updated: "1d ago" },
  { name: "reliant-api-server", env: "prod", kind: "Deployment", tone: "success", status: "Healthy", ready: "2/2", version: "v1.7.43", digest: "sha256:c3f19e07aa21b4d5", restarts: 2, cpu: { used: "380m", limit: "1 core", pct: 38 }, mem: { used: "610Mi", limit: "1Gi", pct: 60 }, updated: "1d ago" },
  { name: "reliant-temporal-worker", env: "prod", kind: "Deployment", tone: "warning", status: "Degraded", ready: "1/2", version: "v1.7.43", digest: "sha256:c3f19e07aa21b4d5", restarts: 7, lastExit: "Error", cpu: { used: "720m", limit: "1 core", pct: 72 }, mem: { used: "880Mi", limit: "1Gi", pct: 86 }, updated: "1d ago" },
  { name: "control-plane-workers", env: "prod", kind: "Deployment", tone: "success", status: "Healthy", ready: "1/1", version: "v1.7.43", digest: "sha256:8a54465b8d71c0e2", restarts: 0, cpu: { used: "60m", limit: "500m", pct: 12 }, mem: { used: "140Mi", limit: "384Mi", pct: 36 }, updated: "1d ago" },
  { name: "daemon-gateway", env: "prod", kind: "Deployment", tone: "success", status: "Healthy", ready: "1/1", version: "v1.7.43", digest: "sha256:8a54465b8d71c0e2", restarts: 1, cpu: { used: "90m", limit: "500m", pct: 18 }, mem: { used: "120Mi", limit: "256Mi", pct: 47 }, updated: "1d ago" },
  { name: "workspace-proxy", env: "prod", kind: "Deployment", tone: "success", status: "Healthy", ready: "2/2", version: "v1.7.43", digest: "sha256:8a54465b8d71c0e2", restarts: 0, cpu: { used: "30m", limit: "250m", pct: 12 }, mem: { used: "70Mi", limit: "256Mi", pct: 27 }, updated: "1d ago" },
  { name: "temporal", env: "prod", kind: "StatefulSet", tone: "success", status: "Healthy", ready: "1/1", version: "1.25.2", digest: "sha256:5d0c2a9e61f7b380", restarts: 0, cpu: { used: "150m", limit: "1 core", pct: 15 }, mem: { used: "300Mi", limit: "1Gi", pct: 29 }, updated: "12d ago" },
  { name: "nats", env: "prod", kind: "StatefulSet", tone: "success", status: "Healthy", ready: "3/3", version: "2.10.22", digest: "sha256:91be0c47d2a35e18", restarts: 0, cpu: { used: "20m", limit: "250m", pct: 8 }, mem: { used: "48Mi", limit: "256Mi", pct: 19 }, updated: "12d ago" },
  { name: "zitadel", env: "prod", kind: "Deployment", tone: "success", status: "Healthy", ready: "1/1", version: "2.66.0", digest: "sha256:e7a3018fbc4d2961", restarts: 0, cpu: { used: "70m", limit: "500m", pct: 14 }, mem: { used: "210Mi", limit: "512Mi", pct: 41 }, updated: "20d ago" },
];

export interface JobRow {
  name: string;
  tone: Tone;
  status: string;
  release: string;
  duration: string;
  finished: string;
}

export const JOBS: JobRow[] = [
  { name: "control-plane-migrate", tone: "success", status: "Succeeded", release: "v1.7.44", duration: "14s", finished: "5h ago" },
  { name: "control-plane-idp-provision", tone: "success", status: "Succeeded", release: "v1.7.44", duration: "6s", finished: "5h ago" },
  { name: "dbrole-provision-reliant", tone: "success", status: "Succeeded", release: "v1.7.43", duration: "3s", finished: "1d ago" },
  { name: "dbrole-provision-temporal", tone: "danger", status: "Failed · BackoffLimitExceeded", release: "v1.7.42", duration: "41s", finished: "3d ago" },
];

// ── One workload ───────────────────────────────────────────────────────────

/** Deterministic pseudo-random series, so screenshots are reproducible. */
export function series(seed: number, n: number, base: number, jitter: number, spikeAt?: number): number[] {
  let s = seed;
  const out: number[] = [];
  for (let i = 0; i < n; i++) {
    s = (s * 9301 + 49297) % 233280;
    const r = s / 233280;
    let v = base + (r - 0.5) * jitter + Math.sin(i / 6) * jitter * 0.3;
    if (spikeAt !== undefined && Math.abs(i - spikeAt) < 3) v += jitter * 2.2 * (1 - Math.abs(i - spikeAt) / 3);
    out.push(Math.max(0, v));
  }
  return out;
}

export const PODS = [
  { name: "admin-server-7d9f8c6b5-x1q2w", tone: "success" as Tone, status: "Running", restarts: 1, lastExit: "OOMKilled · 2h ago", age: "5h", node: "pool-std-4-9f1a" },
  { name: "admin-server-7d9f8c6b5-k8m3p", tone: "success" as Tone, status: "Running", restarts: 0, lastExit: "", age: "5h", node: "pool-std-4-2c7e" },
];

export const EVENTS = [
  { tone: "warning" as Tone, reason: "OOMKilled", object: "pod/admin-server-…-x1q2w", message: "Container admin-server exceeded its memory limit (512Mi)", count: 1, ago: "2h ago" },
  { tone: "neutral" as Tone, reason: "Pulled", object: "pod/admin-server-…-x1q2w", message: "Image control-plane@sha256:20edb404 already present on machine", count: 1, ago: "2h ago" },
  { tone: "neutral" as Tone, reason: "ScalingReplicaSet", object: "deployment/admin-server", message: "Scaled up replica set admin-server-7d9f8c6b5 to 2", count: 1, ago: "5h ago" },
  { tone: "neutral" as Tone, reason: "Scheduled", object: "pod/admin-server-…-k8m3p", message: "Assigned to pool-std-4-2c7e", count: 1, ago: "5h ago" },
];

export const DEPLOY_HISTORY = [
  { version: "v1.7.44", tone: "success" as Tone, what: "Scoped deploy · admin-server only", who: "sean", ago: "5h ago", current: true },
  { version: "v1.7.43", tone: "success" as Tone, what: "Promoted from e2e", who: "sean", ago: "1d ago", current: false },
  { version: "v1.7.42", tone: "danger" as Tone, what: "Rolled back after 22m · DeployFailed", who: "ci", ago: "3d ago", current: false },
  { version: "v1.7.41", tone: "success" as Tone, what: "Promoted from e2e", who: "ci", ago: "4d ago", current: false },
];

export const LOG_LINES = [
  { t: "14:02:11.204", level: "info", msg: "rpc ok procedure=/controlplane.v1.DeployService/GetLiveView duration_ms=38" },
  { t: "14:02:10.988", level: "info", msg: "rpc ok procedure=/controlplane.v1.ObservabilityService/QueryPanel duration_ms=212" },
  { t: "14:02:09.551", level: "warn", msg: "ledger read slow: 1.8s (threshold 1s) env=prod" },
  { t: "14:02:07.130", level: "info", msg: "rpc ok procedure=/controlplane.v1.BillingService/GetUsage duration_ms=19" },
  { t: "14:02:05.774", level: "error", msg: "promote: registry push 401 registry.reliantapi.com (bundle kept locally)" },
  { t: "14:02:04.002", level: "info", msg: "reconcile tick envs=4 converged=4 drifted=0" },
];

// ── Releases ───────────────────────────────────────────────────────────────

export interface ReleaseRow {
  version: string;
  sha: string;
  message: string;
  author: string;
  ago: string;
  commits: number;
  images: number;
  checks: { passed: number; total: number };
  live: string[];
  was: string[];
  rolledBack?: boolean;
  scoped?: string;
}

export const RELEASES: ReleaseRow[] = [
  { version: "v1.7.45", sha: "9c41e2ab", message: "feat(observability): workload detail reads GetWorkload", author: "sean", ago: "52m ago", commits: 6, images: 2, checks: { passed: 25, total: 25 }, live: ["e2e"], was: [] },
  { version: "v1.7.44", sha: "571f17d5", message: "fix(admin-server): raise memory limit, retry ledger reads", author: "sean", ago: "6h ago", commits: 3, images: 1, checks: { passed: 25, total: 25 }, live: ["prod"], was: ["e2e"], scoped: "admin-server" },
  { version: "v1.7.43", sha: "20edb404", message: "feat(deploy): scoped deploys record their target", author: "release-bot", ago: "1d ago", commits: 14, images: 4, checks: { passed: 25, total: 25 }, live: [], was: ["e2e", "prod"] },
  { version: "v1.7.42", sha: "d0a7c3f1", message: "chore(temporal): bump server to 1.25.2", author: "ci", ago: "3d ago", commits: 2, images: 1, checks: { passed: 23, total: 25 }, live: [], was: ["e2e", "prod"], rolledBack: true },
  { version: "v1.7.41", sha: "8a54465b", message: "feat(billing): coupons on the usage page", author: "sean", ago: "4d ago", commits: 9, images: 3, checks: { passed: 25, total: 25 }, live: ["dev-k8s"], was: ["e2e", "prod"] },
  { version: "v1.7.40", sha: "47d0d152", message: "fix(proxy): keep websocket upgrades on idle timeout", author: "mara", ago: "6d ago", commits: 4, images: 1, checks: { passed: 25, total: 25 }, live: [], was: ["e2e", "prod"] },
  { version: "v1.7.39", sha: "b13f9e60", message: "feat(secrets): OpenBao-backed secret store", author: "mara", ago: "8d ago", commits: 21, images: 5, checks: { passed: 25, total: 25 }, live: [], was: ["e2e", "prod"] },
];

export const RELEASE_COMMITS = [
  { sha: "571f17d5", message: "fix(admin-server): raise memory limit to 768Mi", author: "sean", ago: "6h ago" },
  { sha: "3be0a912", message: "fix(ledger): retry transient read errors with jitter", author: "sean", ago: "7h ago" },
  { sha: "e81c44d0", message: "test(ledger): cover the 1s slow-read threshold", author: "sean", ago: "7h ago" },
];

export const RELEASE_IMAGES = [
  { name: "control-plane", from: "sha256:8a54465b8d71", to: "sha256:20edb404371b", changed: true },
  { name: "reliant-api-server", from: "sha256:c3f19e07aa21", to: "sha256:c3f19e07aa21", changed: false },
  { name: "temporal", from: "sha256:5d0c2a9e61f7", to: "sha256:5d0c2a9e61f7", changed: false },
];

export const RELEASE_OBJECTS = [
  { change: "modified", kind: "Deployment", name: "admin-server", detail: "resources.limits.memory 512Mi → 768Mi; image digest" },
  { change: "modified", kind: "ConfigMap", name: "admin-server-config", detail: "LEDGER_READ_RETRIES 0 → 3" },
  { change: "added", kind: "Job", name: "control-plane-migrate-571f17d5", detail: "migration 20261015120000_ledger_retry_index" },
];

export const RELEASE_TIMELINE = [
  { tone: "neutral" as Tone, what: "Cut from main@571f17d5", who: "sean", when: "Oct 15, 09:02" },
  { tone: "success" as Tone, what: "Checks passed · ci:full 25/25", who: "ci", when: "Oct 15, 09:31" },
  { tone: "success" as Tone, what: "Promoted to e2e · converged in 2m 10s", who: "ci", when: "Oct 15, 09:34" },
  { tone: "success" as Tone, what: "Deployed to prod · target admin-server", who: "sean", when: "Oct 15, 09:40" },
  { tone: "success" as Tone, what: "prod converged · stable for 15m", who: "Reliant", when: "Oct 15, 09:57" },
];

// ── Activity ───────────────────────────────────────────────────────────────

export interface ActivityRow {
  day: string;
  time: string;
  actor: string;
  system?: boolean;
  verb: string;
  target: string;
  targetKind: string;
  env?: string;
  via: "Console" | "CLI" | "CI" | "Reliant";
  tone: Tone;
}

export const ACTIVITY: ActivityRow[] = [
  { day: "Today", time: "14:01", actor: "Reliant", system: true, verb: "observed restarts on", target: "reliant-temporal-worker", targetKind: "workload", env: "prod", via: "Reliant", tone: "warning" },
  { day: "Today", time: "13:10", actor: "ci", verb: "promoted", target: "v1.7.45", targetKind: "release", env: "e2e", via: "CI", tone: "success" },
  { day: "Today", time: "13:09", actor: "ci", verb: "cut release", target: "v1.7.45", targetKind: "release", via: "CI", tone: "neutral" },
  { day: "Today", time: "11:42", actor: "mara", verb: "rotated secret", target: "STRIPE_WEBHOOK_SECRET", targetKind: "secret", env: "prod", via: "Console", tone: "neutral" },
  { day: "Today", time: "09:40", actor: "sean", verb: "deployed admin-server from", target: "v1.7.44", targetKind: "release", env: "prod", via: "CLI", tone: "success" },
  { day: "Today", time: "09:12", actor: "sean", verb: "silenced alert", target: "HighMemory · admin-server", targetKind: "alert", env: "prod", via: "Console", tone: "neutral" },
  { day: "Yesterday", time: "18:20", actor: "sean", verb: "promoted", target: "v1.7.43", targetKind: "release", env: "prod", via: "Console", tone: "success" },
  { day: "Yesterday", time: "16:05", actor: "mara", verb: "added domain", target: "status.reliantlabs.io", targetKind: "domain", env: "prod", via: "Console", tone: "neutral" },
  { day: "Yesterday", time: "15:47", actor: "Reliant", system: true, verb: "drift detected on", target: "ConfigMap/admin-server-config", targetKind: "object", env: "prod", via: "Reliant", tone: "warning" },
  { day: "Oct 12", time: "22:31", actor: "ci", verb: "rolled back prod to", target: "v1.7.41", targetKind: "release", env: "prod", via: "CI", tone: "danger" },
  { day: "Oct 12", time: "22:09", actor: "ci", verb: "promoted", target: "v1.7.42", targetKind: "release", env: "prod", via: "CI", tone: "success" },
];
