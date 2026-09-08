import { lazy } from 'react';

/**
 * Decoy management (WCX-11).
 *
 * The queries are public API because the environment screen counts decoys
 * alongside zones and devices; one `queryOptions` helper means however many
 * consumers ask, the list is fetched once per freshness interval.
 */
export { decoyKeys, decoyQuery, decoysQuery } from './api';
export { convergenceOf, hasBeenObserved, observationAgeMs, CONVERGENCE_WINDOW_MS } from './convergence';

/** Its own chunk, per section 9.11. See `@features/auth` for why. */
export const DecoysRoute = lazy(() =>
  import('./DecoysPage').then((module) => ({ default: module.DecoysPage })),
);
export const DecoyRoute = lazy(() =>
  import('./DecoyDetailPage').then((module) => ({ default: module.DecoyDetailPage })),
);
