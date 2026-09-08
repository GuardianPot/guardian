import { lazy } from 'react';

/**
 * The environment list is read by the shell's scope selector as well as by
 * the list screen, so it is public API rather than a screen-local query.
 * `WCX-10` section 9.9: one `queryOptions` helper, so however many consumers
 * ask, the list is fetched once per freshness interval.
 */
export { environmentKeys, environmentQuery, environmentsQuery, zonesQuery } from './api';

/** Route components as their own chunk. See `@features/auth` for why. */
export const EnvironmentsRoute = lazy(() =>
  import('./EnvironmentsPage').then((module) => ({ default: module.EnvironmentsPage })),
);
export const EnvironmentRoute = lazy(() =>
  import('./EnvironmentPage').then((module) => ({ default: module.EnvironmentPage })),
);
