import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  device,
  environment,
  environmentID,
  healthView,
  json,
  loginHandlers,
  renderRoute,
  mockApi,
  zone,
  type MockResponder,
} from '@shared/testing/harness';
import { authKeys } from '@features/auth';
import { expectNoAxeViolations } from '@shared/testing/axe';
import { EnvironmentPage } from './EnvironmentPage';

afterEach(() => vi.unstubAllGlobals());

const entry = `/environments/${environmentID}`;
const routePath = '/environments/:environmentId';

function readHandlers(overrides: Record<string, MockResponder> = {}): Record<string, MockResponder> {
  return {
    ...loginHandlers(),
    [`GET /v1/environments/${environmentID}`]: () => json({ environment: environment() }),
    [`GET /v1/environments/${environmentID}/zones?limit=200`]: () => json({ zones: [] }),
    [`GET /v1/environments/${environmentID}/devices`]: () => json({ devices: [] }),
    [`GET /v1/environments/${environmentID}/health`]: () => json({ error: 'not_found' }, 404),
    ...overrides,
  };
}

describe('EnvironmentPage', () => {
  it('keeps a reload-restored session read-only until the operator reauthenticates', async () => {
    const stub = mockApi(readHandlers());
    renderRoute(<EnvironmentPage />, { path: routePath, entry, authenticated: false });

    expect(await screen.findByRole('heading', { name: 'Lab' })).toBeVisible();
    for (const label of ['Device name', 'Zone name', 'Private CIDR', 'Display name']) {
      expect(screen.getByLabelText(label)).toBeDisabled();
    }
    for (const name of ['Create one-time secret', 'Add zone', 'Save name']) {
      expect(screen.getByRole('button', { name })).toBeDisabled();
    }
    expect(stub.calls.filter((call) => !call.key.startsWith('GET '))).toEqual([]);
  });

  it('reports a rejected zone without claiming success or leaking the submitted values', async () => {
    const stub = mockApi(readHandlers({
      [`POST /v1/environments/${environmentID}/zones`]: () => json({ error: 'invalid_cidr' }, 422),
    }));
    renderRoute(<EnvironmentPage />, { path: routePath, entry });

    await userEvent.type(await screen.findByLabelText('Zone name'), 'Overlapping');
    await userEvent.type(screen.getByLabelText('Private CIDR'), '10.20.0.0/33');
    await userEvent.click(screen.getByRole('button', { name: 'Add zone' }));

    const notice = await screen.findByText(/Zone creation failed/);
    expect(notice).toHaveTextContent('Use a canonical, non-overlapping RFC1918 CIDR.');
    expect(notice).not.toHaveTextContent('10.20.0.0/33');
    // An error is never toast-only: this is a banner, so it persists until the
    // condition clears rather than expiring unseen (section 8.5).
    expect(notice).toHaveAttribute('role', 'alert');
    expect(screen.getByText('No private network zones recorded')).toBeVisible();
    expect(stub.header(`POST /v1/environments/${environmentID}/zones`, 'X-CSRF-Token')).not.toBeNull();
  });

  it('confirms an accepted zone only after the backend records it', async () => {
    mockApi(readHandlers({
      [`POST /v1/environments/${environmentID}/zones`]: () => json({ zone: zone() }, 201),
      [`GET /v1/environments/${environmentID}/zones?limit=200`]: () => json({ zones: [zone()] }),
    }));
    renderRoute(<EnvironmentPage />, { path: routePath, entry });

    await userEvent.type(await screen.findByLabelText('Zone name'), 'Lab zone');
    await userEvent.type(screen.getByLabelText('Private CIDR'), '10.20.0.0/24');
    await userEvent.click(screen.getByRole('button', { name: 'Add zone' }));

    expect(await screen.findByText('Private network zone added.')).toBeVisible();
    expect(screen.getByText('10.20.0.0/24')).toBeVisible();
  });

  it('shows the one-time enrollment secret once and removes it from the DOM, storage, and query cache', async () => {
    const secretToken = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';
    mockApi(readHandlers({
      [`POST /v1/environments/${environmentID}/enrollment-tokens`]: () => json({
        token_id: '018f1f7e-6d31-7cc5-8db8-17547f78e6c6',
        device_id: device().device_id,
        environment_id: environmentID,
        device_name: 'edge-one',
        token: secretToken,
        expires_at: '2026-08-29T12:15:00Z',
      }, 201),
    }));
    localStorage.clear();
    sessionStorage.clear();
    const { queryClient } = renderRoute(<EnvironmentPage />, { path: routePath, entry });

    await userEvent.type(await screen.findByLabelText('Device name'), 'edge-one');
    await userEvent.click(screen.getByRole('button', { name: 'Create one-time secret' }));
    expect(await screen.findByTestId('one-time-secret')).toHaveTextContent(secretToken);

    await userEvent.click(screen.getByRole('button', { name: 'I have stored it securely' }));
    await waitFor(() => expect(screen.queryByTestId('one-time-secret')).not.toBeInTheDocument());
    expect(document.body.innerHTML).not.toContain(secretToken);
    expect(localStorage).toHaveLength(0);
    expect(sessionStorage).toHaveLength(0);
    const cached = JSON.stringify(queryClient.getQueryCache().getAll().map((query) => query.state.data));
    expect(cached).not.toContain(secretToken);
  });

  it('never renders an unavailable health projection as healthy', async () => {
    mockApi(readHandlers({
      [`GET /v1/environments/${environmentID}/health`]: () => json({ error: 'unavailable' }, 503),
    }));
    renderRoute(<EnvironmentPage />, { path: routePath, entry, authenticated: false });

    // No cached projection and an impaired upstream is `degraded`: it names
    // the dependency, and it never presents the absence as health.
    expect(await screen.findByText('The health projection is impaired')).toBeVisible();
    expect(screen.getByText(/Not answering: every health condition for this scope/)).toBeVisible();
    expect(screen.getByText(/Still answering: inventory and configuration/)).toBeVisible();
    expect(screen.queryByText('Healthy')).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Eight-condition health' })).not.toBeInTheDocument();
  });

  it('separates inventory state from the backend health projection', async () => {
    mockApi(readHandlers({
      [`GET /v1/environments/${environmentID}/devices`]: () => json({
        devices: [device({ state: 'pending', display_name: 'edge-pending' })],
      }),
      [`GET /v1/environments/${environmentID}/health`]: () => json(
        healthView({ type: 'edge_connected', status: 'False', reason: 'channel_disconnected' }),
      ),
    }));
    renderRoute(<EnvironmentPage />, { path: routePath, entry, authenticated: false });

    expect(await screen.findByText('edge-pending')).toBeVisible();
    expect(screen.getByText('pending')).toBeVisible();
    expect(await screen.findByRole('heading', { name: 'Eight-condition health' })).toBeVisible();
    expect(screen.getByText(/Blocking: Edge connection/)).toHaveTextContent('channel_disconnected');
    // The eight conditions plus the one device. Scoped to the lists that hold
    // them: `WCX-10` added breadcrumbs, whose entries are list items too, and
    // a bare document-wide count would silently absorb any list added later.
    expect(screen.getByRole('list', { name: 'Device health conditions' })
      .querySelectorAll('li')).toHaveLength(8);
    expect(screen.getByRole('list', { name: 'Edge devices' })
      .querySelectorAll('li')).toHaveLength(1);
  });

  it('renders hostile backend text as inert content', async () => {
    const hostile = '<img src=x onerror=alert(1)>';
    mockApi(readHandlers({
      [`GET /v1/environments/${environmentID}`]: () => json({ environment: environment({ display_name: hostile }) }),
      [`GET /v1/environments/${environmentID}/devices`]: () => json({ devices: [device({ display_name: hostile })] }),
    }));
    const { container } = renderRoute(<EnvironmentPage />, { path: routePath, entry, authenticated: false });

    expect(await screen.findByRole('heading', { name: hostile })).toBeVisible();
    expect(container.querySelector('img')).toBeNull();
    expect(document.querySelectorAll('img')).toHaveLength(0);
  });

  it('presents an authorization failure as refused rather than empty configuration', async () => {
    mockApi(readHandlers({
      [`GET /v1/environments/${environmentID}`]: () => json({ error: 'forbidden' }, 403),
    }));
    renderRoute(<EnvironmentPage />, { path: routePath, entry, authenticated: false });

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Access was refused');
    // A refusal reports nothing about what exists, so no zone count, no device
    // count, and no configuration state reaches the screen (section 8.3).
    expect(alert).toHaveTextContent(/reports nothing about what is or is not here/);
    expect(screen.queryByRole('heading', { name: 'Edge devices' })).not.toBeInTheDocument();
    expect(screen.queryByText('No private network zones recorded')).not.toBeInTheDocument();
  });

  it('reports no serious or critical axe violation with health present or absent', async () => {
    mockApi(readHandlers({
      [`GET /v1/environments/${environmentID}/devices`]: () => json({ devices: [device()] }),
      [`GET /v1/environments/${environmentID}/health`]: () => json(healthView()),
      [`GET /v1/environments/${environmentID}/zones?limit=200`]: () => json({ zones: [zone()] }),
    }));
    const withHealth = renderRoute(<EnvironmentPage />, { path: routePath, entry, authenticated: false });
    await screen.findByRole('heading', { name: 'Eight-condition health' });
    await expectNoAxeViolations(withHealth.container);
    withHealth.unmount();

    // The read-only session renders every control disabled with a reason, which
    // is the state most likely to leave a control unnamed.
    mockApi(readHandlers());
    const withoutHealth = renderRoute(<EnvironmentPage />, { path: routePath, entry, authenticated: false });
    await screen.findByRole('heading', { name: 'Lab' });
    await expectNoAxeViolations(withoutHealth.container);
  });

  it('drops the memory-only CSRF proof when a mutation is rejected as unauthorized', async () => {
    const patch = `PATCH /v1/environments/${environmentID}`;
    const stub = mockApi(readHandlers({ [patch]: () => json({ error: 'unauthorized' }, 401) }));
    const { queryClient } = renderRoute(<EnvironmentPage />, { path: routePath, entry });

    await userEvent.click(await screen.findByRole('button', { name: 'Save name' }));

    await waitFor(() => expect(queryClient.getQueryData(authKeys.session())).toBeNull());
    await waitFor(() => expect(screen.getByLabelText('Display name')).toBeDisabled());
    expect(screen.getByRole('button', { name: 'Save name' })).toBeDisabled();
    expect(stub.called(patch)).toHaveLength(1);
  });
});
