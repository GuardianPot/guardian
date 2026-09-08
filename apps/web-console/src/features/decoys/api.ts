import { queryOptions, type QueryClient } from '@tanstack/react-query';
import { DEFAULT_PAGE_SIZE } from '@shared/api/query';
import { freshness } from '@shared/api/freshness';
import { request } from '@shared/api/transport';
import { taintDecoyView } from '@shared/api/taint';
import type { DecoyView, DecoyViewRaw, DecoyWriteRequest } from '@shared/api/types';

export const decoyKeys = {
  all: ['decoys'] as const,
  list: (environmentID: string) => [...decoyKeys.all, 'list', environmentID] as const,
  detail: (environmentID: string, decoyID: string) =>
    [...decoyKeys.all, 'detail', environmentID, decoyID] as const,
};

/**
 * `operational` freshness (section 9.3.3).
 *
 * A decoy's observed state changes without the operator doing anything — an
 * Edge converges, a process dies, a device is revoked — so the list has to move
 * on its own. Configuration freshness would leave someone watching a
 * deployment they had already been told to expect within sixty seconds.
 */
export const decoysQuery = (environmentID: string) =>
  queryOptions({
    queryKey: decoyKeys.list(environmentID),
    queryFn: async ({ signal }) =>
      (await request<{ decoys: DecoyViewRaw[] }>(
        `/v1/environments/${environmentID}/decoys?limit=${DEFAULT_PAGE_SIZE}`,
        { signal },
      )).decoys.map(taintDecoyView),
    ...freshness('operational'),
  });

export const decoyQuery = (environmentID: string, decoyID: string) =>
  queryOptions({
    queryKey: decoyKeys.detail(environmentID, decoyID),
    queryFn: async ({ signal }) =>
      taintDecoyView(
        await request<DecoyViewRaw>(
          `/v1/environments/${environmentID}/decoys/${decoyID}`,
          { signal },
        ),
      ),
    ...freshness('operational'),
  });

export async function createDecoy(
  environmentID: string,
  input: DecoyWriteRequest,
  csrf: string,
): Promise<DecoyView> {
  return taintDecoyView(
    await request<DecoyViewRaw>(`/v1/environments/${environmentID}/decoys`, {
      method: 'POST',
      body: input,
      csrf,
    }),
  );
}

/**
 * Every decoy write carries the revision the operator was looking at.
 *
 * A `412` means something changed first. Section 9.9.1 forbids resolving that
 * silently: the caller renders the conflict and offers the current value, and
 * nothing is overwritten on the strength of a stale form.
 */
export async function updateDecoy(
  environmentID: string,
  decoy: { decoy_id: string; revision: number },
  input: DecoyWriteRequest,
  csrf: string,
): Promise<DecoyView> {
  return taintDecoyView(
    await request<DecoyViewRaw>(`/v1/environments/${environmentID}/decoys/${decoy.decoy_id}`, {
      method: 'PATCH',
      body: input,
      csrf,
      etag: decoy.revision,
    }),
  );
}

/**
 * Enable and disable are separate operations rather than a PATCH field, because
 * WC-D16 assigns confirmation levels to operations. Both return the decoy, and
 * both say only that the desired state changed: what the network is doing is
 * the observed record's business, and it arrives later.
 */
export async function setDecoyEnabled(
  environmentID: string,
  decoy: { decoy_id: string; revision: number },
  enabled: boolean,
  csrf: string,
): Promise<DecoyView> {
  return taintDecoyView(
    await request<DecoyViewRaw>(
      `/v1/environments/${environmentID}/decoys/${decoy.decoy_id}/${enabled ? 'enable' : 'disable'}`,
      { method: 'POST', csrf, etag: decoy.revision },
    ),
  );
}

export function removeDecoy(
  environmentID: string,
  decoy: { decoy_id: string; revision: number },
  csrf: string,
): Promise<void> {
  return request<void>(`/v1/environments/${environmentID}/decoys/${decoy.decoy_id}`, {
    method: 'DELETE',
    csrf,
    etag: decoy.revision,
  });
}

/** Invalidation lives beside the mutations that cause it, never in a component. */
export const decoyInvalidation = {
  afterWrite: (client: QueryClient, environmentID: string) =>
    client.invalidateQueries({ queryKey: decoyKeys.list(environmentID) }),
  afterDetailWrite: (client: QueryClient, environmentID: string, decoyID: string) =>
    Promise.all([
      client.invalidateQueries({ queryKey: decoyKeys.detail(environmentID, decoyID) }),
      client.invalidateQueries({ queryKey: decoyKeys.list(environmentID) }),
    ]),
};
