import { Navigate, Outlet, createBrowserRouter } from 'react-router-dom';
import { useAuth } from '@features/auth';
import { Shell } from '@app/Shell';
import { LoadingState, RouteErrorBoundary } from '@shared/ui';
import { DevicePage } from '@features/devices';
import { EnvironmentPage, EnvironmentsPage } from '@features/environments';
import { LoginPage } from '@features/auth';
import { SHELL_TEXT } from '@app/text';

export function RequireAuth() {
  const auth = useAuth();
  if (auth.loading) return <LoadingState activity={SHELL_TEXT.checkingSession} />;
  if (!auth.session) return <Navigate to="/login" replace />;
  return <Outlet />;
}

export const router = createBrowserRouter([
  // Sign-in is outside the shell, so it carries its own route boundary. Every
  // screen inside the shell is covered by the boundary the shell wraps its
  // outlet in, which keeps navigation and sign-out reachable during a failure.
  { path: '/login', element: <RouteErrorBoundary><LoginPage /></RouteErrorBoundary> },
  {
    element: <RequireAuth />,
    children: [{
      element: <Shell />,
      children: [
        { path: '/environments', element: <EnvironmentsPage /> },
        { path: '/environments/:environmentId', element: <EnvironmentPage /> },
        { path: '/environments/:environmentId/devices/:deviceId', element: <DevicePage /> },
      ],
    }],
  },
  { path: '*', element: <Navigate to="/environments" replace /> },
]);
