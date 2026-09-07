import { queryOptions, type QueryClient } from '@tanstack/react-query';
import { DEFAULT_PAGE_SIZE } from '@shared/api/query';
import { freshness } from '@shared/api/freshness';
import { request } from '@shared/api/transport';
import {
  taintEnrollmentSecret,
  taintEnrollmentToken,
  taintEnvironment,
  taintZone,
} from '@shared/api/taint';
import type {
  EnrollmentSecret,
  EnrollmentSecretRaw,
  EnrollmentToken,
  EnrollmentTokenRaw,
  Environment,
  EnvironmentRaw,
  Zone,
  ZoneRaw,
} from '@shared/api/types';
import { deviceKeys } from '@features/devices';

export const environmentKeys = {
  all: ['environments'] as const,
  list: () => [...environmentKeys.all, 'list'] as const,
  detail: (environmentID: string) => [...environmentKeys.all, 'detail', environmentID] as const,
  zones: (environmentID: string) => [...environmentKeys.all, 'zones', environmentID] as const,
  tokens: (environmentID: string) => [...environmentKeys.all, 'tokens', environmentID] as const,
};

export const environmentsQuery = () =>
  queryOptions({
    queryKey: environmentKeys.list(),
    queryFn: async ({ signal }) =>
      // The trust boundary (WCX-06 section 9.8): display names are marked
      // untrusted once, here, so no screen below can render one directly.
      (await request<{ environments: EnvironmentRaw[] }>(
        `/v1/environments?limit=${DEFAULT_PAGE_SIZE}`,
        { signal },
      )).environments.map(taintEnvironment),
    ...freshness('configuration'),
  });

export const environmentQuery = (environmentID: string) =>
  queryOptions({
    queryKey: environmentKeys.detail(environmentID),
    queryFn: async ({ signal }) =>
      taintEnvironment(
        (await request<{ environment: EnvironmentRaw }>(`/v1/environments/${environmentID}`, { signal }))
          .environment,
      ),
    ...freshness('configuration'),
  });

export const zonesQuery = (environmentID: string) =>
  queryOptions({
    queryKey: environmentKeys.zones(environmentID),
    queryFn: async ({ signal }) =>
      (await request<{ zones: ZoneRaw[] }>(
        `/v1/environments/${environmentID}/zones?limit=${DEFAULT_PAGE_SIZE}`,
        { signal },
      )).zones.map(taintZone),
    ...freshness('configuration'),
  });

export async function createEnvironment(displayName: string, csrf: string): Promise<Environment> {
  return taintEnvironment((await request<{ environment: EnvironmentRaw }>('/v1/environments', {
    method: 'POST',
    body: { display_name: displayName },
    csrf,
  })).environment);
}

export async function updateEnvironment(
  environment: Environment,
  displayName: string,
  csrf: string,
): Promise<Environment> {
  return taintEnvironment((await request<{ environment: EnvironmentRaw }>(
    `/v1/environments/${environment.environment_id}`,
    { method: 'PATCH', body: { display_name: displayName }, csrf, etag: environment.revision },
  )).environment);
}

export async function createZone(
  environmentID: string,
  input: { display_name: string; cidr: string },
  csrf: string,
): Promise<Zone> {
  return taintZone((await request<{ zone: ZoneRaw }>(`/v1/environments/${environmentID}/zones`, {
    method: 'POST',
    body: input,
    csrf,
  })).zone);
}

/**
 * Creates a one-time enrollment secret.
 *
 * The result deliberately bypasses the query cache: the secret lives only in
 * route-local state so dismissal, route exit, and unload destroy it.
 */
export async function createEnrollmentSecret(
  environmentID: string,
  deviceName: string,
  csrf: string,
): Promise<EnrollmentSecret> {
  return taintEnrollmentSecret(
    await request<EnrollmentSecretRaw>(`/v1/environments/${environmentID}/enrollment-tokens`, {
      method: 'POST',
      body: { device_name: deviceName },
      csrf,
    }),
  );
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
    Promise.all([
      client.invalidateQueries({ queryKey: deviceKeys.list(environmentID) }),
      client.invalidateQueries({ queryKey: environmentKeys.tokens(environmentID) }),
    ]),
  afterTokenRevoke: (client: QueryClient, environmentID: string) =>
    client.invalidateQueries({ queryKey: environmentKeys.tokens(environmentID) }),
};

/**
 * Enrollment tokens, without their values (WCX-09 sections 8.9 and 9.4).
 *
 * The contract's summary has no token field, so there is nothing to omit here
 * and nothing a screen could accidentally render. The one-time secret remains
 * visible exactly once, at creation, under the existing rules.
 *
 * `operational` freshness: an operator watching a handoff window close needs
 * the list to move without a manual refresh.
 */
export const enrollmentTokensQuery = (environmentID: string) =>
  queryOptions({
    queryKey: environmentKeys.tokens(environmentID),
    queryFn: async ({ signal }) =>
      (await request<{ tokens: EnrollmentTokenRaw[] }>(
        `/v1/environments/${environmentID}/enrollment-tokens`,
        { signal },
      )).tokens.map(taintEnrollmentToken),
    ...freshness('operational'),
  });

export function revokeEnrollmentToken(
  environmentID: string,
  tokenID: string,
  csrf: string,
): Promise<void> {
  return request<void>(
    `/v1/environments/${environmentID}/enrollment-tokens/${tokenID}`,
    { method: 'DELETE', csrf },
  );
}

/**
 * The state a token is in, derived rather than stored.
 *
 * The contract carries three optional timestamps and no state field, so the
 * console decides. Expiry is computed against the wall clock, which is why
 * `expired` is derived at render time and not cached: a token expires while
 * the operator is looking at it.
 *
 * An expired or consumed token stays in the list (section 9.5.4). Removing it
 * would hide that a handoff window closed, which is exactly what an operator
 * investigating a failed enrollment needs to see.
 */
export type EnrollmentTokenState = 'active' | 'consumed' | 'revoked' | 'expired';

export function enrollmentTokenState(
  token: EnrollmentToken,
  now: number = Date.now(),
): EnrollmentTokenState {
  // Revocation is a decision and outranks the clock: a token revoked before it
  // expired was revoked, and saying "expired" would lose that.
  if (token.revoked_at !== undefined) return 'revoked';
  if (token.consumed_at !== undefined) return 'consumed';
  const expiresAt = Date.parse(token.expires_at);
  // An unparseable expiry cannot be shown to be current, so it is not.
  if (Number.isNaN(expiresAt) || expiresAt <= now) return 'expired';
  return 'active';
}

/**
 * Updates one zone under optimistic concurrency.
 *
 * `If-Match` carries the revision the operator was looking at. A `412` means
 * something else changed it first, and section 9.8.1 forbids resolving that
 * silently: the caller renders the conflict and offers to reload.
 */
export async function updateZone(
  environmentID: string,
  zone: Zone,
  input: { display_name: string; cidr: string },
  csrf: string,
): Promise<Zone> {
  return taintZone((await request<{ zone: ZoneRaw }>(
    `/v1/environments/${environmentID}/zones/${zone.zone_id}`,
    { method: 'PATCH', body: input, csrf, etag: zone.revision },
  )).zone);
}

export function deleteZone(environmentID: string, zone: Zone, csrf: string): Promise<void> {
  return request<void>(
    `/v1/environments/${environmentID}/zones/${zone.zone_id}`,
    { method: 'DELETE', csrf, etag: zone.revision },
  );
}
