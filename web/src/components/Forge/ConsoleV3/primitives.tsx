// Copyright (c) 2025 Reliant Labs

/**
 * Console v3 PROTOTYPE primitives. These are the shapes proposed for
 * `components/forge-ui/` (see research/console-v3/PROPOSAL.md, "Visual
 * system"); they live here so the prototype does not restyle the shipping
 * console while the owner reacts to it.
 *
 * The rules they encode:
 *  - One surface level. A page is `bg-background`; a panel is `bg-card` with
 *    `border-border`; a well inside a panel is `bg-background` with
 *    `border-border/60`. Never a card in a card; `bg-muted` only for hover and
 *    selection (web-styling skill).
 *  - Sentence-case labels, never uppercase-tracked ones.
 *  - Monospace only for ids and digests, truncated, always with a copy button.
 *  - Status is a dot plus a word, never a colored pill wall.
 */

import { useState, type ReactNode } from "react";
import { Check, Copy } from "lucide-react";

import { cn } from "@/lib/utils";

import type { Tone } from "./fixtures";

const DOT: Record<Tone, string> = {
  success: "bg-success",
  warning: "bg-warning",
  danger: "bg-destructive",
  neutral: "bg-muted-foreground/50",
  accent: "bg-primary",
};

const INK: Record<Tone, string> = {
  success: "text-success-ink",
  warning: "text-warning-ink",
  danger: "text-destructive-ink",
  neutral: "text-muted-foreground",
  accent: "text-primary",
};

export function Dot({ tone, className }: { tone: Tone; className?: string }) {
  return <span aria-hidden="true" className={cn("inline-block h-2 w-2 shrink-0 rounded-full", DOT[tone], className)} />;
}

/** A status: a dot and a word. The word carries the tone only when it is not "fine". */
export function Status({ tone, children }: { tone: Tone; children: ReactNode }) {
  return (
    <span className="inline-flex items-center gap-2 whitespace-nowrap">
      <Dot tone={tone} />
      <span className={tone === "success" || tone === "neutral" ? "text-foreground" : INK[tone]}>{children}</span>
    </span>
  );
}

/** A small neutral tag — env names, kinds. Bordered, no fill. */
export function Tag({ children, tone = "neutral", className }: { children: ReactNode; tone?: Tone; className?: string }) {
  return (
    <span
      className={cn(
        "inline-flex h-5 items-center gap-1 whitespace-nowrap rounded-md border px-1.5 text-xs leading-none",
        tone === "neutral" ? "border-border text-muted-foreground" : "border-current/30",
        tone !== "neutral" && INK[tone],
        className,
      )}
    >
      {children}
    </span>
  );
}

/**
 * An id or digest: monospace, truncated to `keep` characters, full value on
 * hover and in the clipboard. The ONLY place monospace is allowed.
 */
export function Mono({ value, keep = 12, copy = true, className }: { value: string; keep?: number; copy?: boolean; className?: string }) {
  const [copied, setCopied] = useState(false);
  const shown = value.length > keep ? value.slice(0, keep) : value;
  return (
    <span className={cn("group inline-flex items-center gap-1 align-middle", className)}>
      <code title={value} className="font-mono text-xs text-muted-foreground">
        {shown}
      </code>
      {copy && (
        <button
          type="button"
          aria-label={`Copy ${value}`}
          onClick={(e) => {
            e.stopPropagation();
            void navigator.clipboard?.writeText(value);
            setCopied(true);
            setTimeout(() => setCopied(false), 1200);
          }}
          className="rounded p-0.5 text-muted-foreground opacity-0 transition hover:bg-muted hover:text-foreground focus-visible:opacity-100 group-hover:opacity-100"
        >
          {copied ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
        </button>
      )}
    </span>
  );
}

export function Button({
  children,
  variant = "secondary",
  icon,
  className,
}: {
  children?: ReactNode;
  variant?: "primary" | "secondary" | "ghost" | "danger";
  icon?: ReactNode;
  className?: string;
}) {
  return (
    <button
      type="button"
      className={cn(
        "inline-flex h-8 items-center gap-1.5 whitespace-nowrap rounded-md px-3 text-sm font-medium transition focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring",
        variant === "primary" && "bg-primary text-primary-foreground hover:bg-primary/90",
        variant === "secondary" && "border border-border bg-card text-foreground shadow-xs hover:bg-muted",
        variant === "ghost" && "text-muted-foreground hover:bg-muted hover:text-foreground",
        variant === "danger" && "border border-border bg-card text-destructive-ink hover:bg-muted",
        className,
      )}
    >
      {icon}
      {children}
    </button>
  );
}

/** The page header: breadcrumb, title row with actions, one meta line. No card. */
export function PageHead({
  crumbs,
  title,
  status,
  meta,
  actions,
}: {
  crumbs?: ReactNode[];
  title: ReactNode;
  status?: ReactNode;
  meta?: ReactNode;
  actions?: ReactNode;
}) {
  return (
    <header className="px-8 pt-6">
      {crumbs && crumbs.length > 0 && (
        <nav aria-label="Breadcrumb" className="mb-2 flex items-center gap-1.5 text-xs text-muted-foreground">
          {crumbs.map((c, i) => (
            <span key={i} className="flex items-center gap-1.5">
              {i > 0 && <span aria-hidden="true">/</span>}
              <span className={i === crumbs.length - 1 ? "text-foreground" : "hover:text-foreground"}>{c}</span>
            </span>
          ))}
        </nav>
      )}
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0">
          <div className="flex items-center gap-3">
            <h1 className="truncate text-xl font-semibold tracking-tight text-foreground">{title}</h1>
            {status}
          </div>
          {meta && <div className="mt-1.5 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm text-muted-foreground">{meta}</div>}
        </div>
        {actions && <div className="flex shrink-0 items-center gap-2">{actions}</div>}
      </div>
    </header>
  );
}

/** Underline tabs, flush with the page header. */
export function Tabs({ tabs, active, onSelect }: { tabs: { id: string; label: string; count?: number }[]; active: string; onSelect?: (id: string) => void }) {
  return (
    <div role="tablist" className="mt-4 flex gap-5 border-b border-border px-8">
      {tabs.map((t) => (
        <button
          key={t.id}
          role="tab"
          type="button"
          aria-selected={t.id === active}
          onClick={() => onSelect?.(t.id)}
          className={cn(
            "-mb-px flex items-center gap-1.5 border-b-2 pb-2.5 pt-1 text-sm transition",
            t.id === active ? "border-foreground font-medium text-foreground" : "border-transparent text-muted-foreground hover:text-foreground",
          )}
        >
          {t.label}
          {t.count !== undefined && <span className="rounded bg-muted px-1 text-xs tabular-nums text-muted-foreground">{t.count}</span>}
        </button>
      ))}
    </div>
  );
}

/** A panel: the one surface level. Title row is part of it, not a nested header card. */
export function Panel({
  title,
  description,
  actions,
  children,
  flush,
  className,
}: {
  title?: ReactNode;
  description?: ReactNode;
  actions?: ReactNode;
  children: ReactNode;
  flush?: boolean;
  className?: string;
}) {
  return (
    <section className={cn("rounded-lg border border-border bg-card", className)}>
      {(title || actions) && (
        <div className="flex items-center justify-between gap-3 px-4 pb-0 pt-3">
          <div className="min-w-0">
            {title && <h2 className="text-sm font-medium text-foreground">{title}</h2>}
            {description && <p className="mt-0.5 text-xs text-muted-foreground">{description}</p>}
          </div>
          {actions && <div className="flex items-center gap-2">{actions}</div>}
        </div>
      )}
      <div className={flush ? "mt-2" : "p-4"}>{children}</div>
    </section>
  );
}

/** Table primitives: 40px rows, hairline dividers, no zebra, sticky sentence-case head. */
export function Table({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <div className={cn("overflow-x-auto", className)}>
      <table className="w-full border-collapse text-sm">{children}</table>
    </div>
  );
}

export function Th({ children, className, align = "left" }: { children?: ReactNode; className?: string; align?: "left" | "right" }) {
  return (
    <th
      scope="col"
      className={cn(
        "h-9 whitespace-nowrap border-b border-border px-4 text-xs font-medium text-muted-foreground",
        align === "right" ? "text-right" : "text-left",
        className,
      )}
    >
      {children}
    </th>
  );
}

export function Td({ children, className, align = "left" }: { children?: ReactNode; className?: string; align?: "left" | "right" }) {
  return <td className={cn("h-10 whitespace-nowrap px-4 align-middle", align === "right" && "text-right tabular-nums", className)}>{children}</td>;
}

export function Tr({ children, onClick, selected }: { children: ReactNode; onClick?: () => void; selected?: boolean }) {
  return (
    <tr
      onClick={onClick}
      className={cn(
        "border-b border-border/70 last:border-0",
        onClick && "cursor-pointer hover:bg-muted/60",
        selected && "bg-muted/60",
      )}
    >
      {children}
    </tr>
  );
}

/** A usage bar: used vs limit, with the number beside it. Warns past 80%. */
export function Meter({ pct, label }: { pct: number; label: string }) {
  const tone = pct >= 85 ? "bg-destructive" : pct >= 75 ? "bg-warning" : "bg-primary/70";
  return (
    <div className="flex items-center gap-2">
      <div className="h-1.5 w-16 overflow-hidden rounded-full bg-muted" role="meter" aria-valuenow={pct} aria-valuemin={0} aria-valuemax={100} aria-label={label}>
        <div className={cn("h-full rounded-full", tone)} style={{ width: `${Math.min(100, pct)}%` }} />
      </div>
      <span className="text-xs tabular-nums text-muted-foreground">{label}</span>
    </div>
  );
}

/** A filter chip in a toolbar: `Label: value ▾`. */
export function FilterChip({ label, value, active }: { label: string; value: string; active?: boolean }) {
  return (
    <button
      type="button"
      className={cn(
        "inline-flex h-8 items-center gap-1.5 rounded-md border px-2.5 text-sm transition hover:bg-muted",
        active ? "border-primary/40 text-foreground" : "border-border text-muted-foreground",
      )}
    >
      <span className="text-muted-foreground">{label}</span>
      <span className="text-foreground">{value}</span>
      <svg aria-hidden="true" viewBox="0 0 12 12" className="h-3 w-3 text-muted-foreground">
        <path d="M3 4.5l3 3 3-3" fill="none" stroke="currentColor" strokeWidth="1.5" />
      </svg>
    </button>
  );
}

/**
 * A tiny SVG area chart. The prototype stays dependency-free; the product
 * keeps uPlot. Release markers are vertical hairlines with a label.
 */
export function Spark({
  data,
  height = 96,
  markers = [],
  tone = "primary",
}: {
  data: number[];
  height?: number;
  markers?: { at: number; label: string }[];
  tone?: "primary" | "warning" | "destructive";
}) {
  const w = 600;
  const max = Math.max(...data, 1) * 1.15;
  const step = w / (data.length - 1);
  const pts = data.map((v, i) => `${(i * step).toFixed(1)},${(height - (v / max) * height).toFixed(1)}`);
  const stroke = tone === "warning" ? "hsl(var(--warning))" : tone === "destructive" ? "hsl(var(--destructive))" : "hsl(var(--primary))";
  return (
    <svg viewBox={`0 0 ${w} ${height}`} preserveAspectRatio="none" className="block w-full" style={{ height }} role="img" aria-label="chart">
      {[0.25, 0.5, 0.75].map((f) => (
        <line key={f} x1={0} x2={w} y1={height * f} y2={height * f} stroke="hsl(var(--border))" strokeWidth={1} vectorEffect="non-scaling-stroke" />
      ))}
      <polygon points={`0,${height} ${pts.join(" ")} ${w},${height}`} fill={stroke} fillOpacity={0.1} />
      <polyline points={pts.join(" ")} fill="none" stroke={stroke} strokeWidth={1.5} vectorEffect="non-scaling-stroke" />
      {markers.map((m) => (
        <g key={m.at}>
          <line x1={m.at * step} x2={m.at * step} y1={0} y2={height} stroke="hsl(var(--muted-foreground))" strokeDasharray="3 3" strokeWidth={1} vectorEffect="non-scaling-stroke" />
        </g>
      ))}
    </svg>
  );
}

/** Label/value rows, two columns, no boxes. */
export function Facts({ rows }: { rows: [ReactNode, ReactNode][] }) {
  return (
    <dl className="grid grid-cols-[minmax(7rem,auto)_1fr] gap-x-6 gap-y-2.5 text-sm">
      {rows.map(([k, v], i) => (
        <div key={i} className="contents">
          <dt className="text-muted-foreground">{k}</dt>
          <dd className="min-w-0 truncate text-foreground">{v}</dd>
        </div>
      ))}
    </dl>
  );
}

export function Avatar({ name, system }: { name: string; system?: boolean }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "inline-flex h-5 w-5 shrink-0 items-center justify-center rounded-full text-2xs font-semibold",
        system ? "bg-primary/15 text-primary" : "bg-muted text-muted-foreground",
      )}
    >
      {name.charAt(0).toUpperCase()}
    </span>
  );
}
