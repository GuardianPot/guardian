import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { Suspense, lazy } from 'react';
import { MemoryRouter, RouterProvider, createMemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AuthProvider } from '@features/auth';
import { pollingIsPermitted } from '@shared/api/freshness';
import { UNAUTHORIZED_EVENT } from '@shared/api/transport';
import { LoadingState, RouteErrorBoundary } from '@shared/ui';
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
  type MockResponder,
} from '@shared/testing/harness';
import { routes } from './router';

/**
 * Code splitting, the router upgrade, and the polling gate as they behave in
 * the assembled application (WCX-07 sections 9.2, 9.3, 9.9.4, and 8.1).
 */
afterEach(() => vi.unstubAllGlobals());

function handlers(overrides: Record<string, MockResponder> = {}): Record<string, MockResponder> {
  return {
    ...loginHandlers(),
    'GET /v1/environments?limit=200': () => json({ environments: [environment()] }),
    [`GET /v1/environments/${environmentID}`]: () => json({ environment: environment() }),
    [`GET /v1/environments/${environmentID}/zones?limit=200`]: () => json({ zones: [] }),
    [`GET /v1/environments/${environmentID}/devices`]: () => json({ devices: [device()] }),
    [`GET /v1/environments/${environmentID}/health`]: () => json(healthView()),
    [`GET /v1/environments/${environmentID}/devices/${deviceID}`]: () => json({ device: device() }),
    [`GET /v1/devices/${deviceID}/health`]: () => json(healthView()),
    ...overrides,
  };
}

function renderApp(entry: string, overrides: Record<string, MockResponder> = {}) {
  const api = mockApi(handlers(overrides));
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(routes, { initialEntries: [entry] });
  return {
    api,
    router,
    ...render(
      <QueryClientProvider client={queryClient}>
        <AuthProvider><SignedIn><RouterProvider router={router} /></SignedIn></AuthProvider>
      </QueryClientProvider>,
    ),
  };
}

describe('the route tree after the React Router 8 upgrade', () => {
  it('preserves every path, guard, and redirect', () => {
    // Section 9.3. The upgrade is maintenance: the tree is asserted rather
    // than assumed, so a behavioural change in the router surfaces here.
    const root = routes[0];
    const children = root?.children ?? [];
    const shell = children.find((route) => route.children?.[0]?.children);
    const screens = shell?.children?.[0]?.children ?? [];

    // WCX-10 section 9.1: the incident-first root, then everything that was
    // already reachable, then the catch-all. Every path here is bookmarkable.
    expect(screens.map((route) => route.path)).toEqual([
      '/',
      '/environments',
      '/environments/:environmentId',
      '/environments/:environmentId/decoys',
      '/environments/:environmentId/decoys/:decoyId',
      '/environments/:environmentId/devices/:deviceId',
      '/account',
      '*',
    ]);
    expect(children.some((route) => route.path === '/login')).toBe(true);
    // The catch-all lives inside the authenticated shell, so a signed-out
    // visitor still reaches sign-in rather than a not-found page.
    expect(children.some((route) => route.path === '*')).toBe(false);
  });

  it.each([
    ['/environments', 'Environments'],
    [`/environments/${environmentID}`, 'Lab'],
    [`/environments/${environmentID}/devices/${deviceID}`, 'edge-one'],
    ['/account', 'Account'],
  ])('still renders %s', async (entry, heading) => {
    renderApp(entry);
    expect(await screen.findByRole('heading', { name: heading, level: 1 })).toBeVisible();
  });

  it('renders not-found for an unknown path instead of hiding it', async () => {
    // WCX-10 section 9.7.3. `P1-W11` redirected everything unmatched to the
    // environment list, so an operator following a stale link landed on a
    // working screen and concluded the link was right. The address is now
    // reported as unresolvable and the address bar keeps what was asked for.
    const { router } = renderApp('/not-a-route');
    expect(await screen.findByRole('heading', { name: 'Page not found', level: 1 })).toBeVisible();
    expect(router.state.location.pathname).toBe('/not-a-route');
    // Navigation stays mounted, so the operator is not stranded.
    expect(screen.getByRole('navigation', { name: 'Primary navigation' })).toBeInTheDocument();
  });
});

describe('lazy route loading', () => {
  it('announces the transition through the loading state, not a blank region', async () => {
    // Section 9.2.1 and 9.6.1. A chunk that has not arrived yet must look like
    // every other pending read, announced once through `status`.
    let release: (() => void) | undefined;
    const Slow = lazy(
      () =>
        new Promise<{ default: () => React.ReactElement }>((resolve) => {
          release = () => resolve({ default: () => <h1>Arrived</h1> });
        }),
    );

    render(
      <MemoryRouter>
        <RouteErrorBoundary>
          <Suspense fallback={<LoadingState activity="Loading this screen" />}>
            <Slow />
          </Suspense>
        </RouteErrorBoundary>
      </MemoryRouter>,
    );

    expect(screen.getByRole('status')).toHaveTextContent('Loading this screen');
    release?.();
    expect(await screen.findByRole('heading', { name: 'Arrived' })).toBeVisible();
  });

  it('renders the route fallback when a chunk fails to load, keeping navigation', async () => {
    // Section 9.9.4. A failed chunk must not blank the console: `Suspense`
    // sits inside the route boundary, so the rejection lands in the fallback.
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    const Missing = lazy(() => Promise.reject(new Error('chunk-token-7c2 failed to load')));

    render(
      <MemoryRouter>
        <nav aria-label="Primary navigation"><a href="/environments">Environments</a></nav>
        <RouteErrorBoundary>
          <Suspense fallback={<LoadingState activity="Loading this screen" />}>
            <Missing />
          </Suspense>
        </RouteErrorBoundary>
      </MemoryRouter>,
    );

    expect(await screen.findByRole('alert')).toHaveTextContent('This screen stopped unexpectedly');
    expect(screen.getByRole('navigation', { name: 'Primary navigation' })).toBeVisible();
    // The exception message never reaches the DOM, chunk load or not.
    expect(document.body.innerHTML).not.toContain('chunk-token-7c2');
    consoleError.mockRestore();
  });
});

describe('polling and the session', () => {
  it('permits polling while a session is present', async () => {
    renderApp('/environments');
    await screen.findByRole('heading', { name: 'Environments', level: 1 });
    await waitFor(() => { expect(pollingIsPermitted()).toBe(true); });
  });

  it('issues no request after the unauthorized event fires', async () => {
    // Section 8.1 and 10.1.6. Signing out removes the queries, but an interval
    // firing between removal and the next tick would still reach the Control
    // Plane for an operator who has left.
    const { api } = renderApp('/environments');
    await screen.findByRole('heading', { name: 'Environments', level: 1 });

    const before = api.calls.length;
    window.dispatchEvent(new Event(UNAUTHORIZED_EVENT));

    await waitFor(() => { expect(pollingIsPermitted()).toBe(false); });
    // Give any in-flight interval a chance to fire before counting.
    await new Promise((resolve) => setTimeout(resolve, 60));
    const authOnly = api.calls.slice(before).filter((call) => !call.key.includes('/v1/auth/'));
    expect(authOnly, 'a non-session request was issued after the session ended').toEqual([]);
  });

  it('follows the single existing expiry path on a background 401', async () => {
    // Section 8.2 and 10.1.7. Background refresh must not create a second way
    // to handle an expired session.
    const { api } = renderApp('/environments', {
      'GET /v1/environments?limit=200': () => json({ error: 'unauthorized' }, 401),
    });

    // The operator is returned to sign-in by the same path a foreground 401
    // uses; nothing here re-implements it.
    expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeVisible();
    // The refused read really was a background one, not the session probe.
    expect(api.called('GET /v1/environments?limit=200').length).toBeGreaterThan(0);
    await waitFor(() => { expect(pollingIsPermitted()).toBe(false); });
  });

  it('does not move focus or announce again on a refresh that changes nothing', async () => {
    // Sections 9.5.4 and 9.6.2.
    renderApp('/environments');
    const heading = await screen.findByRole('heading', { name: 'Environments', level: 1 });
    await waitFor(() => { expect(heading).toHaveFocus(); });

    await userEvent.tab();
    const chosen = document.activeElement;
    await new Promise((resolve) => setTimeout(resolve, 80));

    expect(document.activeElement).toBe(chosen);
    expect(screen.queryAllByText(/ screen$/)).toHaveLength(1);
  });
});
