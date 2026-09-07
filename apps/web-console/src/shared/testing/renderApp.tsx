import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render } from '@testing-library/react';
import type { ReactNode } from 'react';
import { RouterProvider, createMemoryRouter } from 'react-router';
import { AuthProvider } from '@features/auth';
import { routes } from '@app/router';
import {
  SignedIn,
  device,
  deviceID,
  environment,
  environmentID,
  healthView,
  json,
  loginHandlers,
  mockApi,
  session,
  type MockResponder,
} from './harness';

/**
 * The whole application, mounted at one address (WCX-10 section 10.1).
 *
 * `renderRoute` in the harness mounts one screen against a two-route memory
 * router, which is right for a component test and wrong for anything about
 * the shell: the shell is what a screen is mounted *inside*, and a navigation
 * entry, a breadcrumb, or a disclosure only exists in the assembled tree.
 *
 * Extracted from `splitting.test.tsx`, which had built this for the router
 * upgrade. Three suites need it now, and three copies of an application
 * bootstrap is three places for a provider to go missing.
 */
export function appHandlers(overrides: Record<string, MockResponder> = {}): Record<string, MockResponder> {
  return {
    ...loginHandlers(),
    'GET /v1/environments?limit=200': () => json({ environments: [environment()] }),
    [`GET /v1/environments/${environmentID}`]: () => json({ environment: environment() }),
    [`GET /v1/environments/${environmentID}/zones?limit=200`]: () => json({ zones: [] }),
    [`GET /v1/environments/${environmentID}/devices`]: () => json({ devices: [device()] }),
    [`GET /v1/environments/${environmentID}/enrollment-tokens`]: () => json({ tokens: [] }),
    [`GET /v1/environments/${environmentID}/health`]: () => json(healthView()),
    [`GET /v1/environments/${environmentID}/devices/${deviceID}`]: () => json({ device: device() }),
    [`GET /v1/devices/${deviceID}/health`]: () => json(healthView()),
    'GET /v1/auth/sessions': () => json({ sessions: [session()] }),
    ...overrides,
  };
}

export type RenderAppOptions = {
  /**
   * `false` keeps the reload-restored, read-only session: a cookie the
   * Control Plane accepts with no CSRF proof in memory.
   */
  signedIn?: boolean;
  handlers?: Record<string, MockResponder>;
};

export function renderApp(entry: string, options: RenderAppOptions = {}) {
  const api = mockApi(appHandlers(options.handlers ?? {}));
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(routes, { initialEntries: [entry] });
  const tree = <RouterProvider router={router} />;
  return {
    api,
    router,
    queryClient,
    ...render(
      <QueryClientProvider client={queryClient}>
        <AuthTree signedIn={options.signedIn ?? true}>{tree}</AuthTree>
      </QueryClientProvider>,
    ),
  };
}

function AuthTree({ signedIn, children }: { signedIn: boolean; children: ReactNode }) {
  return <AuthProvider>{signedIn ? <SignedIn>{children}</SignedIn> : children}</AuthProvider>;
}
