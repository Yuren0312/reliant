// Copyright (c) 2025 Reliant Labs

/**
 * The pure parts of the health strip (HealthStrip, on the Overview) and the
 * workload page's red panels: which service an alert names, the figures of
 * the four-cell strip. No React, so they are tested on their own.
 */

import type { AlertInstance } from "@/services/controlPlane/observability/alertingTypes";
import type { Panel } from "@/services/controlPlane/observability/types";

import type { ChartBand } from "../charts/UPlotChart";

/** The labels an alert names its service by, in the order the rule packs use them. */
export function alertService(alert: Pick<AlertInstance, "labels">): string | undefined {
  const l = alert.labels;
  return l.service ?? l.service_name ?? l["service.name"] ?? l.workload ?? l.job;
}

// ── The strip ────────────────────────────────────────────────────────────────

const SERVER_SPANS = [{ field: "span.kind", op: "eq", value: "server" }];

/**
 * The strip's figures, as one forge panel: requests, failed requests and p95
 * over the project's server spans. Drawn as a NUMBER panel (the window's
 * totals, and again for the previous window) and as a LINE panel (the
 * sparklines, one bucket grid for all three).
 */
export function redPanel(kind: "number" | "line"): Panel {
  return {
    id: `observability-home-${kind}`,
    title: "Requests, errors and p95",
    kind,
    signal: "traces",
    where: SERVER_SPANS,
    series: [
      { label: "requests", agg: "count" },
      { label: "errors", agg: "count", where: [{ field: "span.status", op: "eq", value: "error" }] },
      { label: "p95", agg: "quantile", quantile: 0.95, field: "span.duration" },
    ],
  };
}

/** A series' values by label from a panel answer. */
export function seriesValues(series: readonly { label: string; values: (number | null)[] }[], label: string): (number | null)[] {
  return series.find((s) => s.label === label)?.values ?? [];
}

/** errors / requests per bucket; null where a bucket had no requests. */
export function ratioSeries(errors: readonly (number | null)[], requests: readonly (number | null)[]): (number | null)[] {
  return requests.map((r, i) => (r ? (errors[i] ?? 0) / r : null));
}

/**
 * How many alerts were firing in each bucket, from their firing bands — the
 * Firing alerts cell's sparkline.
 */
export function firingCounts(timestamps: readonly number[], bands: readonly ChartBand[]): number[] {
  const step = timestamps.length > 1 ? timestamps[1] - timestamps[0] : 60;
  return timestamps.map((t) => bands.filter((b) => b.startSec < t + step && b.endSec > t).length);
}
