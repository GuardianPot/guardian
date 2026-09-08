import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { RouterProvider, createMemoryRouter } from 'react-router';
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
  mockApi,
  type MockResponder,
} from '@shared/testing/harness';
import { routes } from './router';

/**
 * The form and live-region contract, asserted across the real routes
 * (WCX-05 sections 9.6 and 9.7).
 *
 * The component tests prove each control can be built correctly. This proves
 * every control the console actually renders *is*, which is the only version
 * of the claim an operator experiences.
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

function renderApp(entry: string, options: { signedIn?: boolean; overrides?: Record<string, MockResponder> } = {}) {
  mockApi(handlers(options.overrides));
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const router = createMemoryRouter(routes, { initialEntries: [entry] });
  const tree = <RouterProvider router={router} />;
  return render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>{options.signedIn === false ? tree : <SignedIn>{tree}</SignedIn>}</AuthProvider>
    </QueryClientProvider>,
  );
}

const SCREENS: readonly {
  name: string;
  entry: string;
  heading: string;
  /** Controls a read-only session disables: fields plus their submit buttons. */
  readOnlyControls: number;
}[] = [
  { name: 'environments', entry: '/environments', heading: 'Environments', readOnlyControls: 2 },
  { name: 'environment', entry: `/environments/${environmentID}`, heading: 'Lab', readOnlyControls: 7 },
  { name: 'device', entry: `/environments/${environmentID}/devices/${deviceID}`, heading: 'edge-one', readOnlyControls: 3 },
];

describe('forms', () => {
  it.each(SCREENS)('names every input on the $name screen', async ({ entry, heading }) => {
    renderApp(entry);
    await screen.findByRole('heading', { name: heading, level: 1 });

    const inputs = [...document.querySelectorAll('input')];
    expect(inputs.length).toBeGreaterThanOrEqual(entry === '/environments' ? 1 : 0);
    for (const input of inputs) {
      expect(input, `${input.name} has no accessible name`).toHaveAccessibleName();
    }
  });

  it.each(SCREENS)('gives every disabled control on the $name screen a reason', async ({ entry, heading, readOnlyControls }) => {
    // Section 9.7.5 and WC-D07: a control the operator cannot use is disabled,
    // never hidden, and says why. Read-only is the state that produces them.
    renderApp(entry, { signedIn: false });
    await screen.findByRole('heading', { name: heading, level: 1 });

    const disabled = [
      ...document.querySelectorAll('button[disabled], input[disabled]'),
    ];
    // Counted, not merely iterated: a screen that rendered nothing would pass
    // an assertion that only checks the controls it found.
    expect(disabled).toHaveLength(readOnlyControls);
    for (const control of disabled) {
      const identity = control.textContent?.trim() || control.getAttribute('name') || control.tagName;
      expect(control, `${identity} is disabled with no reason`).toHaveAccessibleDescription();
    }
  });

  it('associates a rejected submission with the control it concerns', async () => {
    renderApp(`/environments/${environmentID}`, {
      overrides: { [`POST /v1/environments/${environmentID}/zones`]: () => json({ error: 'invalid_cidr' }, 422) },
    });
    await screen.findByRole('heading', { name: 'Lab', level: 1 });

    await userEvent.type(screen.getByLabelText('Zone name'), 'Overlapping');
    // Well-formed, so the client validator passes it and the Control Plane is
    // what refuses it. A malformed value would never leave the browser.
    await userEvent.type(screen.getByLabelText('Private CIDR'), '10.30.0.0/24');
    await userEvent.click(screen.getByRole('button', { name: 'Add zone' }));

    const alert = await screen.findByRole('alert');
    // Section 9.6.5: the message names the correction, not just the failure.
    expect(alert).toHaveTextContent('Use a canonical, non-overlapping RFC1918 CIDR.');
    expect(alert).not.toHaveTextContent('10.30.0.0/24');
  });
});

describe('live regions', () => {
  it.each(SCREENS)('keeps at most one assertive region on the $name screen', async ({ entry, heading }) => {
    // Section 9.7.2. Two competing assertive regions interrupt each other, so
    // an operator hears a fragment of each.
    renderApp(entry, { signedIn: false });
    await screen.findByRole('heading', { name: heading, level: 1 });

    expect(document.querySelectorAll('[role="alert"], [aria-live="assertive"]').length).toBeLessThanOrEqual(1);
  });

  it('keeps at most one assertive region when a read is refused', async () => {
    renderApp(`/environments/${environmentID}`, {
      signedIn: false,
      overrides: { [`GET /v1/environments/${environmentID}`]: () => json({ error: 'forbidden' }, 403) },
    });

    await screen.findByText('Access was refused');
    expect(document.querySelectorAll('[role="alert"], [aria-live="assertive"]')).toHaveLength(1);
  });

  it('announces through a polite region, so a screen change never interrupts', async () => {
    renderApp('/environments');
    await screen.findByRole('heading', { name: 'Environments', level: 1 });

    const announcer = screen.getByText('Environments screen');
    await waitFor(() => { expect(announcer).toBeInTheDocument(); });
    expect(announcer.getAttribute('role')).toBe('status');
    expect(announcer.getAttribute('aria-live')).not.toBe('assertive');
  });
});
