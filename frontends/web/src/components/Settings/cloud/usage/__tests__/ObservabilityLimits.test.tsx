import { create } from "@bufbuild/protobuf";
import { Code, ConnectError } from "@connectrpc/connect";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { GetObservabilityLimitsResponseSchema, UpdateObservabilityLimitsResponseSchema, GetObservabilityStorageResponseSchema } from "@/gen/controlplane/services/observability_billing/v1/observability_billing_pb";
import { ObservabilityLimits, ObservabilityFairUse } from "../ObservabilityLimits";
import { StorageSnapshot, DailyUsageTable } from "../ObservabilityUsageDetails";

const limits = (canUpdate = true) => create(GetObservabilityLimitsResponseSchema, { canUpdate, spendCapCents: 5000n, version: 0n, alertThresholdsPercent: [50, 80, 100], alertOnCap: true, bounds: { maxCents: 100000n, defaultCents: 5000n, chargedThisPeriodCents: 4200n }, queryLanes: [{ lane: "interactive", queriesPerHour: 3000n, usageKnown: false }], maxQuerySeconds: 30, maxQueryMemoryBytes: 1073741824n, maxQueryRange: { seconds: 2592000n } });
describe("observability limits", () => {
  it("members can view but cannot edit", () => {
    render(<ObservabilityLimits orgId="org" limits={limits(false)} save={vi.fn()} reload={vi.fn()} />);
    expect(screen.queryByRole("button", { name: /edit observability/i })).not.toBeInTheDocument();
    expect(screen.getByText(/Cap at \$50.00/)).toBeInTheDocument();
    expect(screen.getByText(/delivery and recipient routing are not yet connected/)).toBeInTheDocument();
  });
  it("sends the read version including zero, and explicit zero for no overage", async () => {
    const save = vi.fn().mockResolvedValue(create(UpdateObservabilityLimitsResponseSchema, { limits: limits(), spendCapRaisedToCharged: true }));
    render(<ObservabilityLimits orgId="org" limits={limits()} save={save} reload={vi.fn()} />);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: /edit observability/i }));
    await user.click(screen.getByRole("radio", { name: /no overage/i }));
    await user.click(screen.getByRole("button", { name: /save observability/i }));
    expect(save).toHaveBeenCalledWith(expect.objectContaining({ orgId: "org", expectedVersion: 0n, spendCapCents: 0n, setAlertThresholds: true }));
    expect(await screen.findByRole("status")).toHaveTextContent("cannot undo charges");
  });
  it("does not retry unseen edits after a version conflict; reload requires review", async () => {
    const save = vi.fn().mockRejectedValue(new ConnectError("stale", Code.Aborted));
    const reload = vi.fn().mockResolvedValue(undefined);
    render(<ObservabilityLimits orgId="org" limits={limits()} save={save} reload={reload} />);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: /edit observability/i }));
    fireEvent.change(screen.getByLabelText("Monthly cap (USD)"), { target: { value: "75" } });
    await user.click(screen.getByRole("button", { name: /save observability/i }));
    expect(await screen.findByRole("status")).toHaveTextContent("Your edits were not saved");
    expect(screen.getByRole("button", { name: /save observability/i })).toBeDisabled();
    expect(save).toHaveBeenCalledTimes(1);
    await user.click(screen.getByRole("button", { name: /reload latest/i }));
    await waitFor(() => expect(reload).toHaveBeenCalledTimes(1));
    expect(await screen.findByText(/Review them before editing again/)).toBeInTheDocument();
    expect(save).toHaveBeenCalledTimes(1);
  });
  it("keeps backend validation errors visible without reporting success", async () => {
    render(<ObservabilityLimits orgId="org" limits={limits()} save={vi.fn().mockRejectedValue(new ConnectError("Cap must be at most $1000", Code.InvalidArgument))} reload={vi.fn()} />);
    const user = userEvent.setup(); await user.click(screen.getByRole("button", { name: /edit observability/i })); await user.click(screen.getByRole("button", { name: /save observability/i }));
    expect(await screen.findByRole("status")).toHaveTextContent("at most $1000");
  });
  it.each(["success", "conflict"])("ignores a stale A %s after switching to B and back to A", async (outcome) => {
    let resolve!: (value: ReturnType<typeof create<typeof UpdateObservabilityLimitsResponseSchema>>) => void;
    let reject!: (reason: unknown) => void;
    const pending = new Promise<ReturnType<typeof create<typeof UpdateObservabilityLimitsResponseSchema>>>((yes, no) => { resolve = yes; reject = no; });
    const save = vi.fn(() => pending);
    const props = { limits: limits(), save, reload: vi.fn() };
    const view = render(<ObservabilityLimits {...props} orgId="A" />);
    const user = userEvent.setup();
    await user.click(screen.getByRole("button", { name: /edit observability/i }));
    await user.click(screen.getByRole("button", { name: /save observability/i }));
    view.rerender(<ObservabilityLimits {...props} orgId="B" />);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    view.rerender(<ObservabilityLimits {...props} orgId="A" />);
    await user.click(screen.getByRole("button", { name: /edit observability/i }));
    fireEvent.change(screen.getByLabelText("Monthly cap (USD)"), { target: { value: "88" } });
    await act(async () => {
      if (outcome === "success") resolve(create(UpdateObservabilityLimitsResponseSchema));
      else reject(new ConnectError("old conflict", Code.Aborted));
      await pending.catch(() => undefined);
    });
    expect(screen.getByLabelText("Monthly cap (USD)")).toHaveValue(88);
    expect(screen.getByRole("button", { name: /save observability/i })).toBeEnabled();
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    expect(save).toHaveBeenCalledTimes(1);
    expect(save).toHaveBeenCalledWith(expect.objectContaining({ orgId: "A" }));
  });
  it("does not show unknown query usage as zero", () => {
    render(<ObservabilityFairUse limits={limits()} />);
    expect(screen.getByText(/Current usage unavailable/)).toBeInTheDocument();
    expect(screen.queryByText(/0 queries;/)).not.toBeInTheDocument();
  });
  it("missing snapshot is unavailable rather than zero GB stored", () => {
    render(<StorageSnapshot storage={create(GetObservabilityStorageResponseSchema)} />);
    expect(screen.getByText(/No stored-now snapshot available/)).toBeInTheDocument();
    expect(screen.queryByText(/0 GB stored/)).not.toBeInTheDocument();
  });
  it("has a useful empty monthly history state", () => {
    render(<DailyUsageTable buckets={[]} />);
    expect(screen.getByText(/No ingestion recorded in this month/)).toBeInTheDocument();
  });
});
