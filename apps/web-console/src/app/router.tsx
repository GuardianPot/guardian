import { Navigate, Outlet, createBrowserRouter } from 'react-router-dom';
import { useAuth } from '@features/auth';
import { Shell } from '@app/Shell';
import { LoadingState } from '@shared/ui/Status';
import { DevicePage } from '@features/devices';
import { EnvironmentPage, EnvironmentsPage } from '@features/environments';
import { LoginPage } from '@features/auth';

export function RequireAuth() {
  const auth = useAuth();
  if (auth.loading) return <LoadingState label="Checking your session…" />;
  if (!auth.session) return <Navigate to="/login" replace />;
  return <Outlet />;
}

export const router = createBrowserRouter([
  { path: '/login', element: <LoginPage /> },
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
