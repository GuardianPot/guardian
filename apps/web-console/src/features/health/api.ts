import { queryOptions } from '@tanstack/react-query';
import { freshness } from '@shared/api/freshness';
import { request } from '@shared/api/transport';
import { taintHealthView } from '@shared/api/taint';
import type { HealthViewRaw } from '@shared/api/types';

export const healthKeys = {
  all: ['health'] as const,
  environment: (environmentID: string) => [...healthKeys.all, 'environment', environmentID] as const,
  device: (deviceID: string) => [...healthKeys.all, 'device', deviceID] as const,
};

export const environmentHealthQuery = (environmentID: string) =>
  queryOptions({
    queryKey: healthKeys.environment(environmentID),
    // Condition reasons and messages come from the device, so they cross the
    // trust boundary here (WCX-06 section 9.8).
    queryFn: async ({ signal }) =>
      taintHealthView(await request<HealthViewRaw>(`/v1/environments/${environmentID}/health`, { signal })),
    ...freshness('operational'),
    // A missing projection is an answer, not a transient fault; do not retry.
    retry: false,
  });

export const deviceHealthQuery = (deviceID: string) =>
  queryOptions({
    queryKey: healthKeys.device(deviceID),
    queryFn: async ({ signal }) =>
      taintHealthView(await request<HealthViewRaw>(`/v1/devices/${deviceID}/health`, { signal })),
    ...freshness('operational'),
    retry: false,
  });
