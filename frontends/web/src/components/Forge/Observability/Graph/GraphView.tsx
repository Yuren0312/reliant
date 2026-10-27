// Copyright (c) 2026 Reliant Labs
import { useEffect, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { obsKeys, useObsReadApi } from "@/hooks/observability-queries";
import { obsRetry } from "@/services/controlPlane/observability/errors";
import type { ObsScope } from "@/services/controlPlane/observability/types";
import { resolveWindow, type RangePreset } from "@/services/controlPlane/observability/window";
import type { ObsNavigate, ObsSearch } from "../obsSearch";
import { ObsError, ObsLoadingRows } from "../ObsStates";
import { GraphPanel } from "./GraphPanel";
import { RPCExplorer } from "./RPCExplorer";

const INPUT = "h-8 max-w-full rounded-md border border-border bg-background px-2 text-sm text-foreground";
export function GraphView({ scope, range, search, navigate, subject }: {
  scope: ObsScope; range: RangePreset; search: ObsSearch; navigate: ObsNavigate; subject: string;
}) {
  const api = useObsReadApi();
  // A window stays frozen while selecting nodes/methods. Refresh explicitly reruns
  // queries for this window; changing the time preset remounts this boundary.
  const [window] = useState(() => resolveWindow(range));
  const explorer = search.view === "rpc";
  const rpcId = search.rpcId ?? "";
  const service = search.rpcService ?? "";
  const processService = search.service ?? "";
  const processServices = processService ? [processService] : [];
  const catalog = useQuery({ queryKey: [...obsKeys.all(scope), "graph-catalog", window, service, processService],
    queryFn: () => api.getServiceGraphCatalog({ ...scope, window, rpcService: service, services: processServices }), staleTime: Infinity, retry: obsRetry });
  const graph = useQuery({ queryKey: [...obsKeys.all(scope), "graph", window, rpcId, service, processService, !!search.internal, !!search.declared],
    queryFn: () => api.getServiceGraph({ ...scope, window, rpcIds: rpcId ? [rpcId] : [], rpcService: service, services: processServices, includeInternal: !!search.internal, includeDeclared: !!search.declared }),
    enabled: !explorer, staleTime: Infinity, retry: obsRetry });
  const services = [...new Set(catalog.data?.methods.map((m) => m.service) ?? [])].sort();
  const methods = [...new Set(catalog.data?.methods.filter((m) => !service || m.service === service).map((m) => m.id) ?? [])].sort();
  const [draft, setDraft] = useState(rpcId);
  useEffect(() => setDraft(rpcId), [rpcId]);
  const onRPC = (id: string) => { setDraft(id); navigate({ rpcId: id || undefined, rpcService: id.split("/")[0] || undefined }); };
  return <div className="space-y-3" data-testid="graph-view">
    <div className="flex flex-wrap items-end gap-3">
      <label className="space-y-1 text-xs text-muted-foreground"><span className="block">Full gRPC / Connect service</span><select aria-label="RPC service" className={INPUT} value={service} onChange={(e) => { setDraft(""); navigate({ rpcService: e.target.value || undefined, rpcId: undefined }); }}><option value="">All services</option>{service && !services.includes(service) && <option>{service}</option>}{services.map((s) => <option key={s}>{s}</option>)}</select></label>
      <form className="flex items-end gap-2" onSubmit={(e) => { e.preventDefault(); onRPC(draft.trim()); }}><label className="space-y-1 text-xs text-muted-foreground"><span className="block">Exact full service/method</span><input aria-label="Full RPC identity" list="graph-rpc-methods" className={`${INPUT} w-80`} value={draft} onChange={(e) => setDraft(e.target.value)} placeholder="acme.v1.OrdersService/GetOrder" maxLength={1024} /><datalist id="graph-rpc-methods">{methods.map((m) => <option key={m} value={m} />)}</datalist></label><button className={`${INPUT} hover:bg-muted`} type="submit">Apply</button></form>
      {!explorer && <><label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={!!search.errors} onChange={(e) => navigate({ errors: e.target.checked || undefined })} />Server faults only</label><label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={!!search.declared} onChange={(e) => navigate({ declared: e.target.checked || undefined })} />Declared overlay</label><label className="flex items-center gap-2 text-sm"><input type="checkbox" checked={!!search.internal} onChange={(e) => navigate({ internal: e.target.checked || undefined })} />RPC / method detail</label></>}
      <button className="text-sm text-primary underline" onClick={() => navigate({ view: explorer ? "graph" : "rpc" })}>{explorer ? "Service graph" : "Browse RPC catalog"}</button>
    </div>
    {processService && <p className="text-sm">Process service filter: {processService} <button className="text-primary underline" onClick={() => navigate({ service: undefined })}>Clear</button></p>}
    <p className="text-xs text-muted-foreground">Bounded minute aggregates · maximum one hour. Investigation window is frozen until the interval or scope changes. Missing minutes are not zero traffic.</p>
    {scope.filters.length > 0 && <p role="status" className="text-sm">Attribute filters remain in scope. Unsupported aggregate dimensions are rejected by the API, never silently ignored. <button className="text-primary underline" onClick={() => navigate({ f: undefined })}>Clear unsupported attribute filters</button></p>}
    {!!catalog.error && <ObsError error={catalog.error} subject={`${subject} RPC catalog`} onRetry={() => void catalog.refetch()} />}
    {explorer ? catalog.isPending ? <ObsLoadingRows /> : catalog.data && <RPCExplorer catalog={catalog.data} service={service} rpcId={rpcId} onSelect={onRPC} onGraph={(id) => navigate({ view: "graph", rpcId: id })} /> : graph.isPending ? <ObsLoadingRows /> : graph.error ? <ObsError error={graph.error} subject={subject} onRetry={() => void graph.refetch()} /> : graph.data && <GraphPanel graph={graph.data} catalog={catalog.data} scope={scope} filters={{ rpcId, errorsOnly: search.errors, declared: search.declared, internal: search.internal }} navigate={navigate} onRPC={onRPC} />}
  </div>;
}
