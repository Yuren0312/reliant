// Copyright (c) 2025 Reliant Labs

import { useMemo } from "react";
import { useMutation, useQuery, useQueryClient, type QueryClient } from "@tanstack/react-query";

import { connectionGrpc, type Connection } from "../api/connection-grpc";
import { unavailableIntegrationIds } from "../lib/integrationAvailability";
import { catalogServiceKeys } from "./catalog-service-hooks_gen";

export const connectionKeys = {
  all: ["connections"] as const,
  list: (integrationId: string) => [...connectionKeys.all, "list", integrationId] as const,
};

/** The caller's connections to one integration. */
export function useConnections(integrationId: string | undefined) {
  return useQuery({
    queryKey: connectionKeys.list(integrationId ?? ""),
    queryFn: () => connectionGrpc.list(integrationId),
    enabled: !!integrationId,
    staleTime: 30_000,
  });
}

/**
 * The integrations nobody can connect on this deployment (every connection
 * method switched off here), from ListIntegrations: one request for the whole
 * catalog, so the palette can say so on every row before the user clicks.
 * Empty until it loads, and if it fails — rows then read "Not connected",
 * and the connect dialog still explains.
 */
export function useUnavailableIntegrations(): ReadonlySet<string> {
  const query = useQuery({
    queryKey: [...connectionKeys.all, "integrations"],
    queryFn: () => connectionGrpc.listIntegrations(),
    staleTime: 5 * 60_000,
  });
  return useMemo(() => unavailableIntegrationIds(query.data ?? []), [query.data]);
}

/** Refetch everything a new or changed connection shows up in. */
export function invalidateConnectionQueries(queryClient: QueryClient) {
  void queryClient.invalidateQueries({ queryKey: connectionKeys.all });
  // A new connection changes `connected` on every catalog search result,
  // listing and entry for its integration, and what its input pickers list.
  void queryClient.invalidateQueries({ queryKey: catalogServiceKeys.all });
}

export function useCreateApiKeyConnection() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: connectionGrpc.createApiKey,
    onSuccess: (connection: Connection) => {
      invalidateConnectionQueries(queryClient);
      return connection;
    },
  });
}
