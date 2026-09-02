import { screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { device, deviceID, environmentID, healthView, json, loginHandlers, renderRoute, stubFetch, type StubHandler } from '../test/harness';
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

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'No current device health projection exists. Inventory presence is not a healthy signal.',
    );
    expect(screen.getByText('Inventory: active')).toBeVisible();
    expect(screen.queryByText('Healthy')).not.toBeInTheDocument();
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

  it('presents a denied device record as unavailable', async () => {
    renderDevice({
      [`GET /v1/environments/${environmentID}/devices/${deviceID}`]: () => json({ error: 'forbidden' }, 403),
    });

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'The device record is unavailable or outside this environment.',
    );
    expect(screen.queryByRole('heading', { name: 'Eight-condition health' })).not.toBeInTheDocument();
  });
});
