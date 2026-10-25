# Dashboard viewer controls

Viewer state is URL-only. Changing filters, breakdown, section/metric visibility,
release markers or alert bands does not save a dashboard version. Layout/panel
editing remains explicit; editing a layout shows all panels, including those
hidden in the viewer. Share, refresh, and browser history preserve the view.

## Optional spec metadata

```json
{
  "sections": [{ "id": "activity", "title": "Activity" }],
  "filters": [{
    "id": "rpc-service", "label": "RPC service",
    "field": "span.attr.rpc.service", "signal": "traces",
    "panelIds": ["rpc-traces"]
  }],
  "panels": [{ "id": "rpc-traces", "section": "activity" }]
}
```

`panelIds` is canonical JSON (Go `PanelIDs`, proto `panel_ids`). Empty or omitted
means all panels. A producer of a mixed activity/financial dashboard must target
process and RPC filters explicitly away from financial facts. The viewer does not
infer policy from title, prefix, SQL, units, or amount. Undeclared URL filters do
nothing. Panel IDs and section IDs stay stable across releases.

`service.name` is the deployed process, not an RPC service. Trace RPC filters
use `span.attr.rpc.service` / `span.attr.rpc.method`; RPC metrics use the supplied
`metric.attr.procedure` field (full `/package.Service/Method`), targeted to their
metric panels. Runtime panels must not receive these predicates. Generic declared
filter IDs use URL `variables`; known RPC selections have dedicated URL keys.

Breakdown offers only fields declared in dashboard filters or saved panel
`groupBy`, using their actual semantic field spellings. A filter-declared field
uses its `panelIds` targets; a field found only in saved grouping affects panels
already using it. `Saved grouping` restores the original query; `None` explicitly
clears grouping for supported aggregate panels. SQL, funnel, issues, sessions,
search and heatmap panels are not regrouped by guesswork. No client dimension is
offered unless the spec declares it.

URL keys: `service`, `rpcService`, `rpcMethod`, `rpcProcedure`, `variables`,
`breakdown`, `hiddenSections`, `hiddenPanels`, `releases`, `alerts`, plus existing
range, compare, refresh, TV and history-version keys. Hidden sections and hidden
panels are independent: re-enabling a section retains its metric selections.

## Organization Cost integration

The approved CP contract is `BillingService.GetOrganizationCostSummary({org_id})`.
Discovery uses `DashboardSource.BUILTIN = 5`, key `org-costs`, and
`DashboardViewKind.ORGANIZATION_COSTS = 1` (`summary.view_kind`, tag 13).
This is a dedicated organization view, not a telemetry panel or SQL dashboard.
It requires live explicit `billing:read`, not billing-management permission.

Implement its renderer after generated CP bindings are integrated. Use the typed
period/source/allowance records; distinguish actual consumption, list-equivalent,
credit grants, pending estimates and unmeasured amounts. No grand total across
periods; absent amounts are not zero. Show source coverage, limitations,
completeness and measured-through. Environment/process/RPC/time viewer controls
must not filter the canonical organization accounting request.

CP's table NULL addition is `missing_value_indexes`, tag 3: preserve a flagged
value as `null` even when its wire placeholder is zero. The existing table
formatter renders null as a dash. Update `observability/api.ts` when generated
bindings provide `missingValueIndexes`; do not hand-edit generated code.

## Verification and integration checkpoint

Worktree `dashboard-consolidation`, Reliant only. Chart agent owns
`UPlotChart.tsx`; this work does not touch that file or `PanelBody.tsx`.

Before targeted metadata changes: scoped DashboardView/Home/URL tests, frontend
typecheck, touched-file ESLint, and Playwright viewer reload/back/forward passed.
Responsive fixture screenshots: `/tmp/dashboard-controls-wide.png` (1440px),
`/tmp/dashboard-controls-narrow.png` (540px). These screenshots precede the final
metadata controls and must be refreshed after integration.

The machine filled to 100% while other work ran. Heavy starts were refused;
parent instructed a pause. New targeting/section tests and final typecheck,
ESLint, build and affected gate still need to run after disk recovery. No merge
or deploy is authorized in this implementation wave.
