import { queryOptions } from '@tanstack/react-query';
import { freshness } from '@shared/api/freshness';
import { request } from '@shared/api/transport';
import { taintDevice } from '@shared/api/taint';
import type { DeviceRaw } from '@shared/api/types';

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
