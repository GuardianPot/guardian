import { Navigate, Outlet, createBrowserRouter, type RouteObject } from 'react-router-dom';
import { useAuth } from '@features/auth';
import { AppLayout } from '@app/AppLayout';
import { Shell } from '@app/Shell';
import { LoadingState, RouteErrorBoundary } from '@shared/ui';
import { DevicePage } from '@features/devices';
import { EnvironmentPage, EnvironmentsPage } from '@features/environments';
import { LoginPage } from '@features/auth';
import { SHELL_TEXT } from '@app/text';
import { SCREEN } from '@app/screens';

export function RequireAuth() {
  const auth = useAuth();
  if (auth.loading) return <LoadingState activity={SHELL_TEXT.checkingSession} />;
  if (!auth.session) return <Navigate to="/login" replace />;
  return <Outlet />;
}

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
 * Sign-in is outside the shell and carries its own route boundary. Every
 * screen inside the shell is covered by the boundary the shell wraps its
 * outlet in, which keeps navigation and sign-out reachable during a failure.
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

export const routes: RouteObject[] = [
  {
    element: <AppLayout />,
    children: [
      ...workbenchRoutes,
      {
        path: '/login',
        element: <RouteErrorBoundary><LoginPage /></RouteErrorBoundary>,
        handle: { screen: SCREEN.signIn },
      },
      {
        element: <RequireAuth />,
        children: [{
          element: <Shell />,
          children: [
            { path: '/environments', element: <EnvironmentsPage />, handle: { screen: SCREEN.environments } },
            { path: '/environments/:environmentId', element: <EnvironmentPage />, handle: { screen: SCREEN.environment } },
            {
              path: '/environments/:environmentId/devices/:deviceId',
              element: <DevicePage />,
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
