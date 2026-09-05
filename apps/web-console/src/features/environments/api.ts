import { queryOptions, type QueryClient } from '@tanstack/react-query';
import { DEFAULT_PAGE_SIZE, retryDelay, retryRead } from '@shared/api/query';
import { request } from '@shared/api/transport';
import type { EnrollmentSecret, Environment, Zone } from '@shared/api/types';
import { deviceKeys } from '@features/devices';

export const environmentKeys = {
  all: ['environments'] as const,
  list: () => [...environmentKeys.all, 'list'] as const,
  detail: (environmentID: string) => [...environmentKeys.all, 'detail', environmentID] as const,
  zones: (environmentID: string) => [...environmentKeys.all, 'zones', environmentID] as const,
};

export const environmentsQuery = () =>
  queryOptions({
    queryKey: environmentKeys.list(),
    queryFn: async ({ signal }) =>
      (await request<{ environments: Environment[] }>(
        `/v1/environments?limit=${DEFAULT_PAGE_SIZE}`,
        { signal },
      )).environments,
    retry: retryRead,
    retryDelay,
  });

export const environmentQuery = (environmentID: string) =>
  queryOptions({
    queryKey: environmentKeys.detail(environmentID),
    queryFn: async ({ signal }) =>
      (await request<{ environment: Environment }>(`/v1/environments/${environmentID}`, { signal }))
        .environment,
    retry: retryRead,
    retryDelay,
  });

export const zonesQuery = (environmentID: string) =>
  queryOptions({
    queryKey: environmentKeys.zones(environmentID),
    queryFn: async ({ signal }) =>
      (await request<{ zones: Zone[] }>(
        `/v1/environments/${environmentID}/zones?limit=${DEFAULT_PAGE_SIZE}`,
        { signal },
      )).zones,
    retry: retryRead,
    retryDelay,
  });

export async function createEnvironment(displayName: string, csrf: string): Promise<Environment> {
  return (await request<{ environment: Environment }>('/v1/environments', {
    method: 'POST',
    body: { display_name: displayName },
    csrf,
  })).environment;
}

export async function updateEnvironment(
  environment: Environment,
  displayName: string,
  csrf: string,
): Promise<Environment> {
  return (await request<{ environment: Environment }>(
    `/v1/environments/${environment.environment_id}`,
    { method: 'PATCH', body: { display_name: displayName }, csrf, etag: environment.revision },
  )).environment;
}

export async function createZone(
  environmentID: string,
  input: { display_name: string; cidr: string },
  csrf: string,
): Promise<Zone> {
  return (await request<{ zone: Zone }>(`/v1/environments/${environmentID}/zones`, {
    method: 'POST',
    body: input,
    csrf,
  })).zone;
}

/**
 * Creates a one-time enrollment secret.
 *
 * The result deliberately bypasses the query cache: the secret lives only in
 * route-local state so dismissal, route exit, and unload destroy it.
 */
export function createEnrollmentSecret(
  environmentID: string,
  deviceName: string,
  csrf: string,
): Promise<EnrollmentSecret> {
  return request<EnrollmentSecret>(`/v1/environments/${environmentID}/enrollment-tokens`, {
    method: 'POST',
    body: { device_name: deviceName },
    csrf,
  });
}

/** Invalidation lives beside the mutations that cause it, never in a component. */
export const environmentInvalidation = {
  afterEnvironmentWrite: (client: QueryClient) =>
    client.invalidateQueries({ queryKey: environmentKeys.list() }),
  afterEnvironmentUpdate: (client: QueryClient, environmentID: string) =>
    Promise.all([
      client.invalidateQueries({ queryKey: environmentKeys.detail(environmentID) }),
      client.invalidateQueries({ queryKey: environmentKeys.list() }),
    ]),
  afterZoneWrite: (client: QueryClient, environmentID: string) =>
    Promise.all([
      client.invalidateQueries({ queryKey: environmentKeys.zones(environmentID) }),
      client.invalidateQueries({ queryKey: environmentKeys.detail(environmentID) }),
      client.invalidateQueries({ queryKey: environmentKeys.list() }),
    ]),
  afterEnrollmentSecret: (client: QueryClient, environmentID: string) =>
    client.invalidateQueries({ queryKey: deviceKeys.list(environmentID) }),
};
