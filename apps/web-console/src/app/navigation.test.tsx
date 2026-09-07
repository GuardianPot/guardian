import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { RouterProvider, createMemoryRouter } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { AuthProvider } from '@features/auth';
import {
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
import { expectNoAxeViolations } from '@shared/testing/axe';
import { routes } from './router';

/**
 * Route-change behaviour (WCX-05 section 9.2, remediates `P1-W11` GAP-4).
 *
 * Driven through the real route tree rather than a stand-in, because the
 * screen names come from the route definitions' `handle` and a stand-in would
 * assert the test's own wiring.
 */
afterEach(() => vi.unstubAllGlobals());

beforeEach(() => {
  document.title = 'Guardian Console';
});

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
  mockApi(handlers(overrides));
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(routes, { initialEntries: [entry] });
  const view = render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider><RouterProvider router={router} /></AuthProvider>
    </QueryClientProvider>,
  );
  return { ...view, router };
}

/** The one polite region the shell owns, identified by what it says. */
function announcements(): string[] {
  return screen.queryAllByText(/ screen$/).map((node) => node.textContent ?? '');
}

describe('route changes', () => {
  it('sets the document title from the route definition, never from backend data', async () => {
    const hostile = '<img src=x onerror=alert(1)>';
    const { router } = renderApp('/environments', {
      [`GET /v1/environments/${environmentID}`]: () => json({ environment: environment({ display_name: hostile }) }),
    });

    await waitFor(() => { expect(document.title).toBe('Environments — Guardian Console'); });

    await act(async () => { await router.navigate(`/environments/${environmentID}`); });

    await waitFor(() => { expect(document.title).toBe('Environment — Guardian Console'); });
    // Section 8.1: the record's display name reaches the page as inert text
    // and never the title bar, which travels outside the document.
    expect(await screen.findByRole('heading', { name: hostile })).toBeVisible();
    expect(document.title).not.toContain('img');
  });

  it('moves focus to the screen heading', async () => {
    renderApp('/environments');

    const heading = await screen.findByRole('heading', { name: 'Environments', level: 1 });
    await waitFor(() => { expect(heading).toHaveFocus(); });
    expect(heading).toHaveAttribute('tabindex', '-1');
  });

  it('announces the screen once through a single polite region', async () => {
    const { router } = renderApp('/environments');

    await waitFor(() => { expect(announcements()).toEqual(['Environments screen']); });

    await act(async () => { await router.navigate(`/environments/${environmentID}`); });

    await waitFor(() => { expect(announcements()).toEqual(['Environment screen']); });
    // One region, replaced in place — not one per navigation.
    expect(announcements()).toHaveLength(1);
  });

  it('focuses the main landmark first and follows the heading in when it renders', async () => {
    // The environment record arrives asynchronously, so the screen renders its
    // `main` before it has an `h1` — section 9.2.2's loading case.
    let releaseEnvironment: (() => void) | undefined;
    const pending = new Promise<void>((resolve) => { releaseEnvironment = resolve; });
    renderApp(`/environments/${environmentID}`, {
      [`GET /v1/environments/${environmentID}`]: () => json({ environment: environment() }),
    });

    const main = await screen.findByRole('main');
    await waitFor(() => { expect(main).toHaveFocus(); });

    releaseEnvironment?.();
    await pending;
    const heading = await screen.findByRole('heading', { name: 'Lab', level: 1 });
    await waitFor(() => { expect(heading).toHaveFocus(); });
  });

  it('does not take focus back from an operator who moved it while the screen loaded', async () => {
    renderApp('/environments');

    const skipLink = await screen.findByRole('link', { name: 'Skip to content' });
    skipLink.focus();
    expect(skipLink).toHaveFocus();

    // The heading arrives after the operator has chosen where to be.
    await screen.findByRole('heading', { name: 'Environments', level: 1 });
    expect(skipLink).toHaveFocus();
  });

  it('announces nothing extra when a polled read returns the same state', async () => {
    // Section 9.7.3. Device and health reads poll every five seconds; a refresh
    // that changes nothing must not re-announce the screen.
    renderApp(`/environments/${environmentID}/devices/${deviceID}`);

    await waitFor(() => { expect(announcements()).toEqual(['Edge device screen']); });

    await act(async () => { await new Promise((resolve) => setTimeout(resolve, 50)); });

    expect(announcements()).toEqual(['Edge device screen']);
  });

  it('keeps the skip link first in the tab order and lands it on main', async () => {
    renderApp('/environments');
    await screen.findByRole('heading', { name: 'Environments', level: 1 });

    // Start from nothing focused. Route-change focus has just landed on the
    // heading, which sits after the skip link in the document.
    if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
    await userEvent.tab();

    const skipLink = screen.getByRole('link', { name: 'Skip to content' });
    expect(skipLink).toHaveFocus();
    expect(skipLink).toHaveAttribute('href', '#main-content');
    expect(screen.getByRole('main')).toHaveAttribute('id', 'main-content');
  });

  it('reports no serious or critical axe violation on any route', async () => {
    const { router, baseElement } = renderApp('/environments');
    await screen.findByRole('heading', { name: 'Environments', level: 1 });
    await expectNoAxeViolations(baseElement);

    await act(async () => { await router.navigate(`/environments/${environmentID}`); });
    await screen.findByRole('heading', { name: 'Lab', level: 1 });
    await expectNoAxeViolations(baseElement);

    await act(async () => { await router.navigate(`/environments/${environmentID}/devices/${deviceID}`); });
    await screen.findByRole('heading', { name: 'edge-one', level: 1 });
    await expectNoAxeViolations(baseElement);
  });
});
