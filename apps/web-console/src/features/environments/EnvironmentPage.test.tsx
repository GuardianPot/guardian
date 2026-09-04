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
  stubFetch,
  zone,
  type StubHandler,
} from '@shared/testing/harness';
import { EnvironmentPage } from './EnvironmentPage';

afterEach(() => vi.unstubAllGlobals());

const entry = `/environments/${environmentID}`;
const routePath = '/environments/:environmentId';

function readHandlers(overrides: Record<string, StubHandler> = {}): Record<string, StubHandler> {
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
    const stub = stubFetch(readHandlers());
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
    const stub = stubFetch(readHandlers({
      [`POST /v1/environments/${environmentID}/zones`]: () => json({ error: 'invalid_cidr' }, 422),
    }));
    renderRoute(<EnvironmentPage />, { path: routePath, entry });

    await userEvent.type(await screen.findByLabelText('Zone name'), 'Overlapping');
    await userEvent.type(screen.getByLabelText('Private CIDR'), '10.20.0.0/33');
    await userEvent.click(screen.getByRole('button', { name: 'Add zone' }));

    const notice = await screen.findByText(/Zone creation failed/);
    expect(notice).toHaveTextContent('Use a canonical, non-overlapping RFC1918 CIDR.');
    expect(notice).not.toHaveTextContent('10.20.0.0/33');
    expect(screen.getByText('No zones defined.')).toBeVisible();
    expect(stub.header(`POST /v1/environments/${environmentID}/zones`, 'X-CSRF-Token')).not.toBeNull();
  });

  it('confirms an accepted zone only after the backend records it', async () => {
    stubFetch(readHandlers({
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
    stubFetch(readHandlers({
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
    expect(await screen.findByTestId('enrollment-secret')).toHaveTextContent(secretToken);

    await userEvent.click(screen.getByRole('button', { name: 'I have stored it securely' }));
    await waitFor(() => expect(screen.queryByTestId('enrollment-secret')).not.toBeInTheDocument());
    expect(document.body.innerHTML).not.toContain(secretToken);
    expect(localStorage).toHaveLength(0);
    expect(sessionStorage).toHaveLength(0);
    const cached = JSON.stringify(queryClient.getQueryCache().getAll().map((query) => query.state.data));
    expect(cached).not.toContain(secretToken);
  });

  it('never renders an unavailable health projection as healthy', async () => {
    stubFetch(readHandlers({
      [`GET /v1/environments/${environmentID}/health`]: () => json({ error: 'unavailable' }, 503),
    }));
    renderRoute(<EnvironmentPage />, { path: routePath, entry, authenticated: false });

    expect(await screen.findByText('The current health projection is unavailable. This is not a healthy signal.')).toBeVisible();
    expect(screen.queryByText('Healthy')).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Eight-condition health' })).not.toBeInTheDocument();
  });

  it('separates inventory state from the backend health projection', async () => {
    stubFetch(readHandlers({
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
    expect(screen.getAllByRole('listitem')).toHaveLength(9);
  });

  it('renders hostile backend text as inert content', async () => {
    const hostile = '<img src=x onerror=alert(1)>';
    stubFetch(readHandlers({
      [`GET /v1/environments/${environmentID}`]: () => json({ environment: environment({ display_name: hostile }) }),
      [`GET /v1/environments/${environmentID}/devices`]: () => json({ devices: [device({ display_name: hostile })] }),
    }));
    const { container } = renderRoute(<EnvironmentPage />, { path: routePath, entry, authenticated: false });

    expect(await screen.findByRole('heading', { name: hostile })).toBeVisible();
    expect(container.querySelector('img')).toBeNull();
    expect(document.querySelectorAll('img')).toHaveLength(0);
  });

  it('presents an authorization failure as unavailable rather than empty configuration', async () => {
    stubFetch(readHandlers({
      [`GET /v1/environments/${environmentID}`]: () => json({ error: 'forbidden' }, 403),
    }));
    renderRoute(<EnvironmentPage />, { path: routePath, entry, authenticated: false });

    expect(await screen.findByRole('alert')).toHaveTextContent('This environment is unavailable or access was denied.');
  });

  it('drops the memory-only CSRF proof when a mutation is rejected as unauthorized', async () => {
    const patch = `PATCH /v1/environments/${environmentID}`;
    const stub = stubFetch(readHandlers({ [patch]: () => json({ error: 'unauthorized' }, 401) }));
    const { queryClient } = renderRoute(<EnvironmentPage />, { path: routePath, entry });

    await userEvent.click(await screen.findByRole('button', { name: 'Save name' }));

    await waitFor(() => expect(queryClient.getQueryData(['session'])).toBeNull());
    await waitFor(() => expect(screen.getByLabelText('Display name')).toBeDisabled());
    expect(screen.getByRole('button', { name: 'Save name' })).toBeDisabled();
    expect(stub.called(patch)).toHaveLength(1);
  });
});
