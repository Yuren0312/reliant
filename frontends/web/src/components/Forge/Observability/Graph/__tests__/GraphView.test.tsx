// Copyright (c) 2026 Reliant Labs
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { Code, ConnectError } from "@connectrpc/connect";
import { describe, expect, it, vi } from "vitest";
import { fixtureCatalog, fixtureGraph } from "../../__fixtures__/fixtureGraph";
import { GraphView } from "../GraphView";
const api = { getServiceGraph: vi.fn(), getServiceGraphCatalog: vi.fn() };
vi.mock("@/hooks/observability-queries", () => ({ obsKeys: { all: (s: unknown) => ["observability", s] }, useObsReadApi: () => api }));
vi.mock("../GraphPanel", () => ({ GraphPanel: () => <div>Graph populated</div> }));
const scope = { orgId: "org", projectId: "shop", envIds: ["env-prod"], filters: [] };
function show(search = {}) {
  const navigate = vi.fn();
  const client = new QueryClient({ defaultOptions: { queries: { retry: false, gcTime: 0 } } });
  render(<QueryClientProvider client={client}><GraphView scope={scope} range="15m" search={{ view: "graph", ...search }} navigate={navigate} subject="shop" /></QueryClientProvider>);
  return navigate;
}
describe("scoped graph boundary", () => {
  it("sends full service filter without enumerating/truncating methods", async () => {
    api.getServiceGraph.mockResolvedValue(fixtureGraph()); api.getServiceGraphCatalog.mockResolvedValue(fixtureCatalog());
    const navigate = show({ rpcService: "shop.v1.OrdersService" });
    await waitFor(() => expect(api.getServiceGraph).toHaveBeenCalledWith(expect.objectContaining({ ...scope, rpcIds: [], rpcService: "shop.v1.OrdersService" })));
    expect(await screen.findByText("Graph populated")).toBeInTheDocument();
    fireEvent.click(screen.getByLabelText("Declared overlay"));
    expect(navigate).toHaveBeenCalledWith({ declared: true });
  });
  it("renders backend failure with retry, not empty graph success", async () => {
    api.getServiceGraph.mockRejectedValue(new ConnectError("denied", Code.PermissionDenied));
    api.getServiceGraphCatalog.mockResolvedValue(fixtureCatalog());
    show();
    expect(await screen.findByTestId("obs-error-denied")).toBeInTheDocument();
    expect(screen.queryByText("Graph populated")).not.toBeInTheDocument();
  });
});
