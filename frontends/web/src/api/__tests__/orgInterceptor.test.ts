/**
 * The active org rides every request whose message has an `org_id` field,
 * through the REAL transport chain, and nothing else.
 *
 * What this pins is the product rule: the org a person selected is the org a
 * control-plane call acts in, with no call site having to remember it — and a
 * request that names none (no selection yet, or no field to put it in) goes out
 * untouched, so the server resolves the person's default. No "first org".
 */
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup } from "@testing-library/react";
import { BillingService } from "@/gen/controlplane/services/billing/v1/billing_pb";
import { DeployService } from "@/gen/controlplane/services/deploy/v1/deploy_pb";
import { OrganizationService } from "@/gen/controlplane/services/organization/v1/organization_pb";
import { ForgeService } from "@/gen/reliant/v1/forge_pb";
import { SettingsService } from "@/gen/reliant/v1/settings_pb";

const state = vi.hoisted(() => ({ controlPlaneUrl: "https://admin.example.test" }));

vi.mock("@/services/controlPlane/config", () => ({
  get CONTROL_PLANE_API_URL() {
    return state.controlPlaneUrl;
  },
  get hasControlPlane() {
    return Boolean(state.controlPlaneUrl);
  },
}));
vi.mock("@sentry/react", () => ({ captureException: vi.fn(), captureMessage: vi.fn() }));
vi.mock("@/lib/logger", () => ({
  logger: { info: vi.fn(), warn: vi.fn(), error: vi.fn(), debug: vi.fn() },
}));

let bodies: Array<{ url: string; body: Record<string, unknown> }>;

beforeEach(() => {
  vi.resetModules();
  bodies = [];
  vi.stubGlobal(
    "fetch",
    vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const raw = init?.body ? new TextDecoder().decode(init.body as Uint8Array) : "{}";
      bodies.push({ url: String(input), body: JSON.parse(raw || "{}") as Record<string, unknown> });
      return new Response("{}", { status: 200, headers: { "content-type": "application/json" } });
    }),
  );
});

afterEach(() => {
  cleanup();
  vi.unstubAllGlobals();
  localStorage.clear();
});

async function load(activeOrg: string | null) {
  const connect = await import("@/lib/connect");
  const { setAuthTokenProvider } = await import("@/api/authProvider");
  setAuthTokenProvider({ getToken: async () => "t", hasSession: async () => true, refresh: async () => null });
  const store = await import("@/store/activeOrgStore");
  store.bindActiveOrgUser("user-1");
  if (activeOrg) store.chooseActiveOrg(activeOrg);
  return { ...connect, ...store };
}

describe("orgInterceptor", () => {
  it("names the active org on a control-plane request that carries org_id", async () => {
    const { connectClient } = await load("org-b");
    await connectClient(DeployService).listEnvironments({});
    await connectClient(BillingService).getCurrentUserWalletOverview({ ledgerPage: 1, ledgerPageSize: 10 });
    expect(bodies.map((b) => b.body.orgId)).toEqual(["org-b", "org-b"]);
  });

  it("names the org on ForgeService requests, which hand it to forge's credential helper", async () => {
    const { connectClient } = await load("org-b");
    await connectClient(ForgeService).planDeploy({ projectId: "p", env: "prod" });
    expect(bodies[0]?.body.orgId).toBe("org-b");
  });

  it("names nothing when no org is selected, so the server uses the default", async () => {
    const { connectClient } = await load(null);
    await connectClient(DeployService).listEnvironments({});
    expect(bodies[0]?.body).not.toHaveProperty("orgId");
  });

  it("leaves a request with no org_id field alone", async () => {
    const { connectClient } = await load("org-b");
    await connectClient(OrganizationService).listMyOrganizations({});
    await connectClient(SettingsService).listSettings({});
    for (const b of bodies) expect(b.body, b.url).not.toHaveProperty("orgId");
  });

  it("keeps an org the caller named itself", async () => {
    const { connectClient } = await load("org-b");
    await connectClient(DeployService).listEnvironments({ orgId: "org-explicit" });
    expect(bodies[0]?.body.orgId).toBe("org-explicit");
  });

  it("follows a switch on the very next request", async () => {
    const { connectClient, chooseActiveOrg } = await load("org-a");
    await connectClient(DeployService).listEnvironments({});
    chooseActiveOrg("org-b");
    await connectClient(DeployService).listEnvironments({});
    expect(bodies.map((b) => b.body.orgId)).toEqual(["org-a", "org-b"]);
  });
});
