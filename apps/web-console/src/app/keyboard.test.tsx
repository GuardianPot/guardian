import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { readFileSync } from 'node:fs';
import { RouterProvider, createMemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AuthProvider } from '@features/auth';
import {
  SignedIn,
  device,
  deviceID,
  environment,
  environmentID,
  healthView,
  json,
  loginHandlers,
  stubFetch,
  type StubHandler,
} from '@shared/testing/harness';
import { routes } from './router';

/**
 * Keyboard operability (WCX-05 section 9.5).
 *
 * Every interactive control must be reachable by keyboard alone, in a logical
 * order. In a security console this is not a courtesy: an operator who cannot
 * reach the sign-out control cannot end a session.
 */
afterEach(() => vi.unstubAllGlobals());

function handlers(): Record<string, StubHandler> {
  return {
    ...loginHandlers(),
    'GET /v1/environments?limit=200': () => json({ environments: [environment()] }),
    [`GET /v1/environments/${environmentID}`]: () => json({ environment: environment() }),
    [`GET /v1/environments/${environmentID}/zones?limit=200`]: () => json({ zones: [] }),
    [`GET /v1/environments/${environmentID}/devices`]: () => json({ devices: [device()] }),
    [`GET /v1/environments/${environmentID}/health`]: () => json(healthView()),
    [`GET /v1/environments/${environmentID}/devices/${deviceID}`]: () => json({ device: device() }),
    [`GET /v1/devices/${deviceID}/health`]: () => json(healthView()),
  };
}

function renderApp(entry: string) {
  stubFetch(handlers());
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(routes, { initialEntries: [entry] });
  return render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider><SignedIn><RouterProvider router={router} /></SignedIn></AuthProvider>
    </QueryClientProvider>,
  );
}

/**
 * Walks the tab order and returns the accessible name of everything reached.
 *
 * Stops when focus returns to where it started, or after a bound, so a broken
 * trap cannot hang the run.
 */
async function tabOrder(limit = 40): Promise<string[]> {
  if (document.activeElement instanceof HTMLElement) document.activeElement.blur();
  const reached: string[] = [];
  const seen = new Set<Element>();
  for (let step = 0; step < limit; step += 1) {
    await userEvent.tab();
    const active = document.activeElement;
    if (!(active instanceof HTMLElement) || active === document.body) break;
    if (seen.has(active)) break;
    seen.add(active);
    reached.push((active.textContent ?? '').trim() || active.getAttribute('aria-label') || active.tagName);
  }
  return reached;
}

describe('keyboard operability', () => {
  it('reaches the skip link, navigation, and sign-out on the environments screen', async () => {
    renderApp('/environments');
    await screen.findByRole('heading', { name: 'Environments', level: 1 });
    await waitFor(() => { expect(screen.getByRole('button', { name: 'Sign out' })).toBeVisible(); });

    const reached = await tabOrder();

    expect(reached[0]).toBe('Skip to content');
    expect(reached).toContain('Sign out');
    expect(reached.some((name) => name.includes('Environments'))).toBe(true);
    expect(reached.some((name) => name.includes('Create environment'))).toBe(true);
  });

  it('reaches every control on the environment screen, including every form', async () => {
    renderApp(`/environments/${environmentID}`);
    await screen.findByRole('heading', { name: 'Lab', level: 1 });
    await waitFor(() => { expect(screen.getByRole('button', { name: 'Sign out' })).toBeVisible(); });

    const reached = await tabOrder(60);

    expect(reached[0]).toBe('Skip to content');
    for (const control of ['Sign out', 'Create one-time secret', 'Add zone', 'Save name']) {
      expect(reached, control).toContain(control);
    }
    // Every input is reachable too; `tabOrder` reports an empty-valued input by
    // its tag, so count those rather than name them.
    expect(reached.filter((name) => name === 'INPUT').length).toBeGreaterThanOrEqual(4);
  });

  it('reaches sign-out from the device screen', async () => {
    renderApp(`/environments/${environmentID}/devices/${deviceID}`);
    await screen.findByRole('heading', { name: 'edge-one', level: 1 });
    await waitFor(() => { expect(screen.getByRole('button', { name: 'Sign out' })).toBeVisible(); });

    const reached = await tabOrder();

    expect(reached[0]).toBe('Skip to content');
    expect(reached).toContain('Sign out');
  });

  it('uses no positive tabIndex anywhere in the rendered console', async () => {
    renderApp(`/environments/${environmentID}`);
    await screen.findByRole('heading', { name: 'Lab', level: 1 });

    const positive = [...document.querySelectorAll('[tabindex]')].filter(
      (node) => Number(node.getAttribute('tabindex')) > 0,
    );
    expect(positive).toEqual([]);
  });
});

/**
 * The narrow-viewport half of section 9.5.1.
 *
 * jsdom resolves no media queries, so a rendered tab order cannot see what a
 * breakpoint hides. The stylesheet can.
 *
 * This used to record `P1-W11` GAP-1 as an accepted exception: the operator
 * block, which holds sign-out and the re-authentication link, was
 * `display: none` below 900 pixels, so an operator on a narrow screen could
 * not end their session. That is now fixed and the exception is gone. The
 * test stays, inverted: a breakpoint may only hide things that carry no
 * operator control, and the allowed list is exhaustive.
 */
const HIDDEN_AT_A_BREAKPOINT_BY_DESIGN = [
  // Decorative marketing column. Carries no control and, since WCX-05 moved
  // the heading onto the sign-in card, no heading either.
  'loginIntro',
  // The `Control Plane` subtitle inside the brand link. The link itself stays.
  'brand small',
];

describe('narrow viewport', () => {
  it('removes no operator control at any breakpoint', () => {
    const css = readFileSync('src/shared/styles/app.module.css', 'utf8');
    const hidden: string[] = [];
    for (const block of css.matchAll(/@media[^{]+\{([\s\S]*?)\n\}/g)) {
      for (const rule of (block[1] ?? '').matchAll(/([^{}]+)\{([^}]*)\}/g)) {
        if (!/display:\s*none/.test(rule[2] ?? '')) continue;
        for (const selector of (rule[1] ?? '').split(',')) {
          hidden.push(selector.trim().replaceAll('.', ''));
        }
      }
    }

    expect(hidden.length).toBeGreaterThan(0);
    const unaccounted = hidden.filter((selector) => !HIDDEN_AT_A_BREAKPOINT_BY_DESIGN.includes(selector));
    expect(
      unaccounted,
      'a breakpoint hides a region that is not on the exhaustive allowed list; if it carries an operator control that is GAP-1 returning',
    ).toEqual([]);

    // Named explicitly so re-adding the rule fails here rather than silently
    // widening the allowed list.
    expect(hidden).not.toContain('operator');
  });
});
