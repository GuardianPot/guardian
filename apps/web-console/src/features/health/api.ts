import { queryOptions } from '@tanstack/react-query';
import { retryDelay, retryRead } from '@shared/api/query';
import { request } from '@shared/api/transport';
import type { HealthView } from '@shared/api/types';

export const healthKeys = {
  all: ['health'] as const,
  environment: (environmentID: string) => [...healthKeys.all, 'environment', environmentID] as const,
  device: (deviceID: string) => [...healthKeys.all, 'device', deviceID] as const,
};

export const environmentHealthQuery = (environmentID: string) =>
  queryOptions({
    queryKey: healthKeys.environment(environmentID),
    queryFn: ({ signal }) =>
      request<HealthView>(`/v1/environments/${environmentID}/health`, { signal }),
    // A missing projection is an answer, not a transient fault; do not retry.
    retry: false,
    refetchInterval: 5_000,
  });

export const deviceHealthQuery = (deviceID: string) =>
  queryOptions({
    queryKey: healthKeys.device(deviceID),
    queryFn: ({ signal }) => request<HealthView>(`/v1/devices/${deviceID}/health`, { signal }),
    retry: false,
    refetchInterval: 5_000,
  });

export { retryDelay, retryRead };
