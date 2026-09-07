import { screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { device, deviceID, environmentID, healthView, json, loginHandlers, renderRoute, stubFetch, type StubHandler } from '@shared/testing/harness';
import { expectNoAxeViolations } from '@shared/testing/axe';
import { DevicePage } from './DevicePage';

afterEach(() => vi.unstubAllGlobals());

const entry = `/environments/${environmentID}/devices/${deviceID}`;
const routePath = '/environments/:environmentId/devices/:deviceId';

function handlers(overrides: Record<string, StubHandler> = {}): Record<string, StubHandler> {
  return {
    ...loginHandlers(),
    [`GET /v1/environments/${environmentID}/devices/${deviceID}`]: () => json({ device: device() }),
    [`GET /v1/devices/${deviceID}/health`]: () => json(healthView()),
    ...overrides,
  };
}

function renderDevice(overrides: Record<string, StubHandler> = {}) {
  stubFetch(handlers(overrides));
  return renderRoute(<DevicePage />, { path: routePath, entry, authenticated: false });
}

describe('DevicePage', () => {
  it('renders inventory facts alongside the full eight-condition projection', async () => {
    renderDevice({
      [`GET /v1/environments/${environmentID}/devices/${deviceID}`]: () => json({
        device: device({ active_certificate_expires_at: '2027-08-29T12:00:00Z' }),
      }),
    });

    expect(await screen.findByRole('heading', { name: 'edge-one' })).toBeVisible();
    expect(screen.getByText('Inventory: active')).toBeVisible();
    expect(await screen.findByRole('heading', { name: 'Eight-condition health' })).toBeVisible();
    expect(screen.getByRole('list', { name: 'Device health conditions' }).querySelectorAll('li')).toHaveLength(8);
    expect(screen.queryByText('No active certificate')).not.toBeInTheDocument();
  });

  it('reports a missing certificate as a fact rather than an omission', async () => {
    renderDevice();

    expect(await screen.findByText('No active certificate')).toBeVisible();
  });

  it('never treats an active inventory record as a healthy signal', async () => {
    renderDevice({ [`GET /v1/devices/${deviceID}/health`]: () => json({ error: 'not_found' }, 404) });

    // `WCX-04` section 9.6: the bespoke message is now the canonical `unknown`
    // state, with the same meaning. A missing projection is an absent
    // observation, so it announces as `status` rather than as a failure — but
    // it still says, in those words, that this is not a healthy signal.
    expect(await screen.findByText(/Guardian holds no observation of the health of this device/)).toBeVisible();
    expect(screen.getByText(/not a healthy signal/)).toBeVisible();
    expect(screen.getByText(/an enrolled Edge reports its eight health conditions/)).toBeVisible();
    expect(screen.getByText('Inventory: active')).toBeVisible();
    expect(screen.queryByText('Healthy')).not.toBeInTheDocument();
    expect(screen.queryByRole('heading', { name: 'Eight-condition health' })).not.toBeInTheDocument();
  });

  it('keeps a revoked device visible without implying it is reachable', async () => {
    renderDevice({
      [`GET /v1/environments/${environmentID}/devices/${deviceID}`]: () => json({ device: device({ state: 'revoked' }) }),
      [`GET /v1/devices/${deviceID}/health`]: () => json(
        healthView({ type: 'device_certificate_ready', status: 'Unknown', reason: 'not_observed' }),
      ),
    });

    expect(await screen.findByText('Inventory: revoked')).toBeVisible();
    const conditions = await screen.findByRole('list', { name: 'Device health conditions' });
    expect(conditions).toHaveTextContent('Unknown · not_observed');
    expect(screen.getByText(/Blocking: Device certificate/)).toBeVisible();
  });

  it('marks a health projection older than the freshness policy as stale (OPS-03)', async () => {
    // The fixture's projection is dated well in the past, so a successful read
    // still is not current data. The conditions stay visible — the operator
    // needs them — with the age and the reason stated above them.
    renderDevice();

    expect(await screen.findByText('Showing the last data Guardian received')).toBeVisible();
    expect(screen.getByText(/Observed .* ago\./)).toBeVisible();
    expect(screen.getByText(/the last health refresh did not return/)).toBeVisible();
    expect(screen.getByRole('heading', { name: 'Eight-condition health' })).toBeVisible();
    expect(screen.getByRole('list', { name: 'Device health conditions' }).querySelectorAll('li')).toHaveLength(8);
  });

  it('shows a current projection without a staleness claim', async () => {
    renderDevice({
      [`GET /v1/devices/${deviceID}/health`]: () => json({
        ...healthView(),
        received_at: new Date().toISOString(),
      }),
    });

    expect(await screen.findByRole('heading', { name: 'Eight-condition health' })).toBeVisible();
    expect(screen.queryByText('Showing the last data Guardian received')).not.toBeInTheDocument();
  });

  it('reports no serious or critical axe violation with health present or absent', async () => {
    const withHealth = renderDevice();
    await screen.findByRole('heading', { name: 'Eight-condition health' });
    await expectNoAxeViolations(withHealth.container);
    withHealth.unmount();

    const withoutHealth = renderDevice({
      [`GET /v1/devices/${deviceID}/health`]: () => json({ error: 'not_found' }, 404),
    });
    await screen.findByText(/Guardian holds no observation/);
    await expectNoAxeViolations(withoutHealth.container);
  });

  it('presents a denied device record as refused, not as an absent one', async () => {
    renderDevice({
      [`GET /v1/environments/${environmentID}/devices/${deviceID}`]: () => json({ error: 'forbidden' }, 403),
    });

    // Section 8.3. The Phase 1 wording ran "unavailable or access was denied"
    // together; the two are now distinct states and this one is the refusal.
    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Access was refused');
    expect(alert).toHaveTextContent(/reports nothing about what is or is not here/);
    expect(screen.queryByRole('heading', { name: 'Eight-condition health' })).not.toBeInTheDocument();
    expect(screen.queryByText('edge-one')).not.toBeInTheDocument();
  });
});
