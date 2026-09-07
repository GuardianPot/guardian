import { queryOptions, type QueryClient } from '@tanstack/react-query';
import { freshness } from '@shared/api/freshness';
import { request } from '@shared/api/transport';
import { taintDevice, taintEnrollmentSecret } from '@shared/api/taint';
import type { DeviceRaw, DeviceState, EnrollmentSecret, EnrollmentSecretRaw } from '@shared/api/types';

export const deviceKeys = {
  all: ['devices'] as const,
  list: (environmentID: string) => [...deviceKeys.all, 'list', environmentID] as const,
  detail: (environmentID: string, deviceID: string) =>
    [...deviceKeys.all, 'detail', environmentID, deviceID] as const,
};

export const devicesQuery = (environmentID: string) =>
  queryOptions({
    queryKey: deviceKeys.list(environmentID),
    queryFn: async ({ signal }) =>
      // The trust boundary: display names are marked untrusted here, once, so
      // no screen below can render one without the contract (WCX-06 9.8).
      (await request<{ devices: DeviceRaw[] }>(`/v1/environments/${environmentID}/devices`, { signal }))
        .devices.map(taintDevice),
    ...freshness('operational'),
  });

export const deviceQuery = (environmentID: string, deviceID: string) =>
  queryOptions({
    queryKey: deviceKeys.detail(environmentID, deviceID),
    queryFn: async ({ signal }) =>
      taintDevice((await request<{ device: DeviceRaw }>(
        `/v1/environments/${environmentID}/devices/${deviceID}`,
        { signal },
      )).device),
    ...freshness('operational'),
  });

/**
 * The device lifecycle transitions (WCX-09 sections 9.4 and 9.7).
 *
 * Each returns nothing. That is the contract — every one of these is a `204`
 * — and it is also the rule: `WCX-09` section 9.8.4 forbids an optimistic
 * update, so the displayed state changes only when a refetch brings back what
 * the Control Plane now says. A mutation that returned an optimistic device
 * would invite exactly the write these screens must not perform.
 */
const lifecycle = (environmentID: string, deviceID: string, transition: string, csrf: string): Promise<void> =>
  request<void>(
    `/v1/environments/${environmentID}/devices/${deviceID}/${transition}`,
    { method: 'POST', csrf },
  );

export const disableDevice = (environmentID: string, deviceID: string, csrf: string): Promise<void> =>
  lifecycle(environmentID, deviceID, 'disable', csrf);

export const revokeDevice = (environmentID: string, deviceID: string, csrf: string): Promise<void> =>
  lifecycle(environmentID, deviceID, 'revoke', csrf);

/**
 * Issues a one-time re-enrollment token for a stable device record.
 *
 * The result bypasses the query cache exactly as the first enrollment secret
 * does (`W11-C3-A`, `WCX-09` section 8.11): it lives only in route-local state
 * so dismissal, route exit, and unload destroy it. There is deliberately no
 * query, no key, and no second read path — the token cannot be fetched again.
 *
 * The backend moves the record to `pending` and permanently revokes prior
 * certificates and unconsumed tokens. That is *not* a restoration: the device
 * becomes active only once it completes enrollment and the Control Plane says
 * so, which is why the caller refetches rather than assuming (section 8.12).
 */
export async function createReenrollmentToken(
  environmentID: string,
  deviceID: string,
  csrf: string,
): Promise<EnrollmentSecret> {
  return taintEnrollmentSecret(
    await request<EnrollmentSecretRaw>(
      `/v1/environments/${environmentID}/devices/${deviceID}/re-enrollment-token`,
      { method: 'POST', csrf },
    ),
  );
}

/**
 * The transitions the contract offers from each inventory state.
 *
 * A table rather than a chain of conditions in the screen, for the same reason
 * the confirmation levels are a table: a control that appears or disappears by
 * accident is indistinguishable from one that was designed to.
 *
 * `WC-D07` forbids hiding a control, so a transition absent from a state's row
 * still renders — disabled, with the reason. `SEC-06` names re-enrollment as a
 * resolution state for a revoked device, which is why `revoked` is not a dead
 * end here.
 */
export type DeviceTransition = 'disable' | 'revoke' | 'reenroll';

export const DEVICE_TRANSITIONS: Readonly<Record<DeviceState, readonly DeviceTransition[]>> = {
  // Enrolment has not completed, so there is no trust to withdraw yet. Issuing
  // a fresh token replaces the outstanding one.
  pending: ['reenroll'],
  active: ['disable', 'revoke', 'reenroll'],
  // Re-enable is not in the contract: there is no endpoint that returns a
  // disabled device to active. Re-enrollment is the path the API actually
  // offers, and claiming otherwise would be a control that cannot work.
  disabled: ['revoke', 'reenroll'],
  revoked: ['reenroll'],
};

export const deviceInvalidation = {
  afterLifecycle: (client: QueryClient, environmentID: string, deviceID: string) =>
    Promise.all([
      client.invalidateQueries({ queryKey: deviceKeys.detail(environmentID, deviceID) }),
      client.invalidateQueries({ queryKey: deviceKeys.list(environmentID) }),
    ]),
};
