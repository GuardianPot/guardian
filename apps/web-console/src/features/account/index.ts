import { lazy } from 'react';

export { accountKeys, sessionsQuery } from './api';

/**
 * The account area as its own chunk (WCX-09 section 9.10).
 *
 * Nothing in the rest of the console imports it, so it costs an operator
 * nothing until they open it. See `@features/auth` for why the split point
 * lives inside the feature rather than in the router.
 */
export const AccountRoute = lazy(() => import('./AccountPage').then((module) => ({ default: module.AccountPage })));
