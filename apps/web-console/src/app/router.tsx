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
export const routes: RouteObject[] = [
  {
    element: <AppLayout />,
    children: [
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
