// Copyright (c) 2025 Reliant Labs

/**
 * The org's query-quota state for the observability console, provided once
 * (ForgeLayout) and read by the hooks, the banner and the "as of" badges.
 *
 * The provider does three things and nothing else:
 *
 *   1. WATCHES the query cache. Any observability query that fails with a
 *      quota answer reports it to the store, so the first panel to hit the
 *      limit tells every other one — no panel needs to hit it itself.
 *   2. PAUSES polling by being the store the poll registry answers to.
 *   3. RESUMES at the reset: it clears the state a jittered 0–30s after
 *      `resets_at` (so N tabs don't all refetch in the same second), then
 *      refetches the queries that were refused, which brings every polled
 *      view back with it.
 */

import { createContext, useContext, useEffect, useMemo, useSyncExternalStore, type ReactNode } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { ObsPollRegistry } from "./pollRegistry";
import { quotaOf } from "./quota";
import { ObsQuotaStore, type ObsQuotaState } from "./quotaStore";

export interface ObsQuota {
  store: ObsQuotaStore;
  polls: ObsPollRegistry;
}

function createObsQuota(now?: () => number): ObsQuota {
  const store = new ObsQuotaStore(now);
  return { store, polls: new ObsPollRegistry(store) };
}

/** Without a provider nothing reports, so the state stays "ok" and reads and polls run as usual. */
const ObsQuotaContext = createContext<ObsQuota>(createObsQuota());

/** The longest a tab waits after the reset before it resumes. */
export const RESUME_JITTER_MS = 30_000;

export function ObsQuotaProvider({
  children,
  random = Math.random,
  now = Date.now,
}: {
  children: ReactNode;
  /** Jitter source in [0, 1); tests pin it. */
  random?: () => number;
  now?: () => number;
}) {
  const client = useQueryClient();
  const quota = useMemo(() => createObsQuota(now), [now]);
  const { store } = quota;

  useEffect(
    () =>
      client.getQueryCache().subscribe((event) => {
        if (event.type !== "updated" || event.action.type !== "error") return;
        if (event.query.queryKey[0] !== "observability") return;
        const exhaustion = quotaOf(event.action.error, now());
        if (exhaustion) store.report(exhaustion);
      }),
    [client, store, now]
  );

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | undefined;
    const schedule = () => {
      if (timer !== undefined) clearTimeout(timer);
      timer = undefined;
      const state = store.getState();
      if (state.kind === "ok") return;
      const wait = Math.max(0, state.resetsAtMs - now()) + Math.floor(random() * RESUME_JITTER_MS);
      timer = setTimeout(() => {
        store.clear();
        void client.invalidateQueries({
          queryKey: ["observability"],
          predicate: (query) => query.state.status === "error" && quotaOf(query.state.error) !== null,
        });
      }, wait);
    };
    schedule();
    const unsubscribe = store.subscribe(schedule);
    return () => {
      unsubscribe();
      if (timer !== undefined) clearTimeout(timer);
    };
  }, [client, store, random, now]);

  return <ObsQuotaContext.Provider value={quota}>{children}</ObsQuotaContext.Provider>;
}

export function useObsQuota(): ObsQuota {
  return useContext(ObsQuotaContext);
}

/** The quota state, re-rendering the caller when it changes. */
export function useObsQuotaState(): ObsQuotaState {
  const { store } = useObsQuota();
  return useSyncExternalStore(store.subscribe, store.getState, store.getState);
}
