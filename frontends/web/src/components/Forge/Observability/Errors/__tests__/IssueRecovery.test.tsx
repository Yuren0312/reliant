// Copyright (c) 2025 Reliant Labs

import { act, render, screen, waitFor, within } from "@testing-library/react";
import { QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";

import { issueKeys } from "@/hooks/observability-queries";
import { createTestQueryClient } from "@/test/renderWithQuery";
import { ObsQuotaProvider } from "@/services/controlPlane/observability/ObsQuotaContext";
import { ObservabilityApiProvider, type ObservabilityApi } from "@/services/controlPlane/observability/ObservabilityApiContext";
import { quotaPausedError } from "@/services/controlPlane/observability/quota";
import { createFixtureApi } from "../../__fixtures__/fixtureApi";
import { ObsQuotaBanner } from "../../ObsStates";
import { IssueDetail } from "../IssueDetail";

vi.mock("uplot", async () => (await import("../../__tests__/obsMocks")).obsMocks.uplot());

function rig(api: ObservabilityApi) {
  const client = createTestQueryClient();
  render(
    <QueryClientProvider client={client}>
      <ObservabilityApiProvider api={api}>
        <ObsQuotaProvider>
          <ObsQuotaBanner />
          <IssueDetail issueId="iss-orders-deadline" scope={{ orgId: "org-fixture", envIds: ["env-prod"], filters: [] }} subject="production" range="1h" navigate={vi.fn()} onBack={vi.fn()} />
        </ObsQuotaProvider>
      </ObservabilityApiProvider>
    </QueryClientProvider>
  );
  return client;
}

async function ready() {
  await waitFor(() => expect(screen.getByTestId("obs-issue")).toHaveAttribute("data-state", "ready"));
  await screen.findByTestId("obs-issue-timeline-list");
}

describe("issue timeline recovery preserves main's quota and retention behavior", () => {
  it("keeps the loaded detail and discussion, marked as of, after a quota refusal", async () => {
    const api = createFixtureApi();
    const getIssue = api.getIssue;
    let exhausted = false;
    api.getIssue = (req) => exhausted
      ? Promise.reject(quotaPausedError({ limit: "queries", lane: "interactive", resetsAtMs: Date.now() + 60_000, soft: false }))
      : getIssue(req);
    const client = rig(api);
    await ready();
    const timeline = screen.getByTestId("obs-issue-timeline-list");
    exhausted = true;
    await act(async () => { await client.invalidateQueries({ queryKey: issueKeys.detail("iss-orders-deadline", "1h") }); });
    await waitFor(() => expect(client.getQueryState(issueKeys.detail("iss-orders-deadline", "1h"))?.error).toBeTruthy());
    expect(screen.getByTestId("obs-issue")).toHaveAttribute("data-state", "ready");
    expect(screen.getByTestId("obs-issue-timeline-list")).toBe(timeline);
    expect(screen.getByTestId("obs-as-of")).toHaveTextContent("as of");
  });

  it("still replaces cached detail on a non-quota refusal", async () => {
    const api = createFixtureApi();
    const client = rig(api);
    await ready();
    api.getIssue = async () => { throw new Error("permission refused"); };
    await act(async () => { await client.invalidateQueries({ queryKey: issueKeys.detail("iss-orders-deadline", "1h") }); });
    await waitFor(() => expect(screen.getByTestId("obs-issue")).toHaveAttribute("data-state", "error"));
    expect(screen.queryByTestId("obs-issue-timeline-list")).not.toBeInTheDocument();
  });

  it("keeps the timeline when the latest event has aged out, without inventing an event", async () => {
    const api = createFixtureApi();
    const getIssue = api.getIssue;
    api.getIssue = async (req) => ({ ...await getIssue(req), latestEvent: null, frames: [] });
    rig(api);
    await ready();
    expect(within(screen.getByTestId("obs-issue-latest")).getByText(/aged out of retention/)).toBeInTheDocument();
    expect(screen.getByTestId("obs-issue-counts")).toBeInTheDocument();
    expect(screen.queryByTestId("obs-issue-latest-message")).not.toBeInTheDocument();
  });
});
