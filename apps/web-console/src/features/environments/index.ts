import { lazy } from 'react';

export { environmentKeys } from './api';

/** Route components as their own chunk. See `@features/auth` for why. */
export const EnvironmentsRoute = lazy(() =>
  import('./EnvironmentsPage').then((module) => ({ default: module.EnvironmentsPage })),
);
export const EnvironmentRoute = lazy(() =>
  import('./EnvironmentPage').then((module) => ({ default: module.EnvironmentPage })),
);
