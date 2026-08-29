import { Navigate, Outlet, createBrowserRouter } from 'react-router-dom';
import { useAuth } from './auth/AuthContext';
import { Shell } from './components/Shell';
import { LoadingState } from './components/Status';
import { DevicePage } from './routes/DevicePage';
import { EnvironmentPage } from './routes/EnvironmentPage';
import { EnvironmentsPage } from './routes/EnvironmentsPage';
import { LoginPage } from './routes/LoginPage';

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
