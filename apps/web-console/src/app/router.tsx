import { Suspense } from 'react';
import { Navigate, Outlet, createBrowserRouter, type RouteObject } from 'react-router';
import { LoginRoute, useAuth } from '@features/auth';
import { DeviceRoute } from '@features/devices';
import { EnvironmentRoute, EnvironmentsRoute } from '@features/environments';
import { AppLayout } from '@app/AppLayout';
import { Shell } from '@app/Shell';
import { LoadingState, RouteErrorBoundary } from '@shared/ui';
import { SHELL_TEXT } from '@app/text';
import { SCREEN } from '@app/screens';

export function RequireAuth() {
  const auth = useAuth();
  if (auth.loading) return <LoadingState activity={SHELL_TEXT.checkingSession} />;
  if (!auth.session) return <Navigate to="/login" replace />;
  return <Outlet />;
}

/**
 * Route-level code splitting (WCX-07 section 9.2, decision WC-D06).
 *
 * `React.lazy` rather than the router's own `lazy`, because `WC-D06` keeps the
 * explicit route tree without router loaders or actions: the route stays a
 * plain element and the loading behaviour stays React's.
 *
 * Each feature declares its own split point and exports the lazy component
 * through its public API. Doing it there rather than here is what actually
 * works: these barrels are statically imported by the shell and by each
 * other, so a dynamic `import('@features/...')` from this file would move
 * nothing. Which chunk each module lands in is decided in `vite.config.ts`.
 */

/**
 * The development-only component workbench (WCX-06 section 9.5).
 *
 * `import.meta.env.DEV` is replaced with the literal `false` by Vite in a
 * production build, so Rollup evaluates this to an empty array and drops the
 * dynamic import, the workbench module, and the hostile corpus it pulls in.
 * That is the first of the three exclusion proofs; `check-bundle.mjs` and a
 * browser scenario are the other two.
 */
const workbenchRoutes: RouteObject[] = import.meta.env.DEV
  ? [{
    path: '/__components',
    lazy: async () => ({ Component: (await import('@app/workbench/Workbench')).Workbench }),
  }]
  : [];

/**
 * The route tree.
 *
 * `AppLayout` is the persistent root: it owns the live region and the
 * route-change focus and title behaviour, so every screen below inherits them
 * without wiring anything (`WCX-05` section 9.2).
 *
 * Each screen carries its name in `handle`. That is what the document title
 * and the announcement read, so neither can ever contain backend data.
 *
 * Sign-in is outside the shell and carries its own route boundary and its own
 * `Suspense`. Every screen inside the shell is covered by the boundary and the
 * `Suspense` the shell wraps its outlet in, which keeps navigation and
 * sign-out reachable while a chunk loads and if one fails to.
 */
export const routes: RouteObject[] = [
  {
    element: <AppLayout />,
    children: [
      ...workbenchRoutes,
      {
        path: '/login',
        element: (
          <RouteErrorBoundary>
            <Suspense fallback={<LoadingState activity={SHELL_TEXT.loadingScreen} />}>
              <LoginRoute />
            </Suspense>
          </RouteErrorBoundary>
        ),
        handle: { screen: SCREEN.signIn },
      },
      {
        element: <RequireAuth />,
        children: [{
          element: <Shell />,
          children: [
            { path: '/environments', element: <EnvironmentsRoute />, handle: { screen: SCREEN.environments } },
            { path: '/environments/:environmentId', element: <EnvironmentRoute />, handle: { screen: SCREEN.environment } },
            {
              path: '/environments/:environmentId/devices/:deviceId',
              element: <DeviceRoute />,
              handle: { screen: SCREEN.device },
            },
          ],
        }],
      },
      { path: '*', element: <Navigate to="/environments" replace /> },
    ],
  },
];

export const router = createBrowserRouter(routes);
