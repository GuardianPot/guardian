import { queryOptions } from '@tanstack/react-query';
import { retryDelay, retryRead } from '@shared/api/query';
import { request } from '@shared/api/transport';
import type { Device } from '@shared/api/types';

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
      (await request<{ devices: Device[] }>(`/v1/environments/${environmentID}/devices`, { signal }))
        .devices,
    retry: retryRead,
    retryDelay,
    refetchInterval: 5_000,
  });

export const deviceQuery = (environmentID: string, deviceID: string) =>
  queryOptions({
    queryKey: deviceKeys.detail(environmentID, deviceID),
    queryFn: async ({ signal }) =>
      (await request<{ device: Device }>(
        `/v1/environments/${environmentID}/devices/${deviceID}`,
        { signal },
      )).device,
    retry: retryRead,
    retryDelay,
    refetchInterval: 5_000,
  });
