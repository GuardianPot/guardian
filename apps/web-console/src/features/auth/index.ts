import { lazy } from 'react';

export { AuthProvider, useAuth } from './AuthContext';
export { useCapability } from './useCapability';
export { useStepUp, type StepUp } from './useStepUp';
export { authKeys } from './api';

/**
 * The sign-in screen, as its own chunk (WCX-07 section 9.2).
 *
 * The split point is here rather than in the router because the barrel is
 * statically imported by the shell, `main.tsx`, and two other features — a
 * dynamic `import('@features/auth')` from the router therefore moves nothing.
 * Declaring the boundary inside the feature keeps `WCX-01`'s rule that a
 * feature is only ever entered through its public API, and still gives Rollup
 * a real split point.
 */
export const LoginRoute = lazy(() => import('./LoginPage').then((module) => ({ default: module.LoginPage })));
