import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { environment, json, loginHandlers, renderRoute, stubFetch, type StubHandler } from '@shared/testing/harness';
import { EnvironmentsPage } from './EnvironmentsPage';

afterEach(() => vi.unstubAllGlobals());

const routePath = '/environments';

function handlers(overrides: Record<string, StubHandler> = {}): Record<string, StubHandler> {
  return {
    ...loginHandlers(),
    'GET /v1/environments?limit=200': () => json({ environments: [] }),
    ...overrides,
  };
}

describe('EnvironmentsPage', () => {
  it('lists environments with configuration state, never with a health claim', async () => {
    stubFetch(handlers({
      'GET /v1/environments?limit=200': () => json({
        environments: [
          environment({ display_name: 'Lab', zone_count: 1, status: 'zones_defined' }),
          environment({ environment_id: '018f1f7e-6d31-7cc5-8db8-17547f78e6ff', display_name: 'Staging' }),
        ],
      }),
    }));
    renderRoute(<EnvironmentsPage />, { path: routePath, entry: routePath, authenticated: false });

    expect(await screen.findByRole('link', { name: /Lab/ })).toBeVisible();
    expect(screen.getByRole('link', { name: /Staging/ })).toBeVisible();
    expect(screen.getByText('1 zone')).toBeVisible();
    expect(screen.getByText('0 zones')).toBeVisible();
    // Configuration completeness is not protection, and the screen says so.
    expect(screen.getByText('Configuration is not health')).toBeVisible();
    expect(screen.getByText('Configured')).toBeVisible();
    expect(screen.getByText('Needs zones')).toBeVisible();
    expect(screen.queryByText('Healthy')).not.toBeInTheDocument();
  });

  it('offers the creating action on an empty list and calls it a confirmed count', async () => {
    stubFetch(handlers());
    renderRoute(<EnvironmentsPage />, { path: routePath, entry: routePath });

    expect(await screen.findByText('No environments recorded')).toBeVisible();
    expect(screen.getByText(/found zero environments/)).toBeVisible();
    await userEvent.click(screen.getByRole('button', { name: 'Name the first environment' }));
    expect(screen.getByLabelText('Display name')).toHaveFocus();
  });

  it('hides the creating action from a read-only session but never the control itself', async () => {
    stubFetch(handlers());
    renderRoute(<EnvironmentsPage />, { path: routePath, entry: routePath, authenticated: false });

    expect(await screen.findByText('No environments recorded')).toBeVisible();
    // The empty state's shortcut is dropped because the operator cannot act,
    // but the real control stays visible and disabled, with its reason
    // readable (WC-D07, section 9.7.5).
    expect(screen.queryByRole('button', { name: 'Name the first environment' })).not.toBeInTheDocument();
    const create = screen.getByRole('button', { name: 'Create environment' });
    expect(create).toBeVisible();
    expect(create).toBeDisabled();
    expect(create).toHaveAccessibleDescription('Re-authenticate before creating an environment.');
  });

  it('presents a refused list as refused, not as an empty organization', async () => {
    stubFetch(handlers({
      'GET /v1/environments?limit=200': () => json({ error: 'forbidden' }, 403),
    }));
    renderRoute(<EnvironmentsPage />, { path: routePath, entry: routePath, authenticated: false });

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('Access was refused');
    expect(screen.queryByText('No environments recorded')).not.toBeInTheDocument();
  });

  it('confirms a created environment with a toast and reports a failure inline', async () => {
    stubFetch(handlers({
      'POST /v1/environments': () => json({ environment: environment({ display_name: 'Lab' }) }, 201),
    }));
    renderRoute(<EnvironmentsPage />, { path: routePath, entry: routePath });

    await userEvent.type(await screen.findByLabelText('Display name'), 'Lab');
    await userEvent.click(screen.getByRole('button', { name: 'Create environment' }));

    expect(await screen.findByText('Environment created.')).toBeVisible();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('never leaves an error on the toast surface alone', async () => {
    stubFetch(handlers({
      'POST /v1/environments': () => json({ error: 'validation_failed' }, 422),
    }));
    renderRoute(<EnvironmentsPage />, { path: routePath, entry: routePath });

    await userEvent.type(await screen.findByLabelText('Display name'), 'Lab');
    await userEvent.click(screen.getByRole('button', { name: 'Create environment' }));

    // The failure is inline, and no toast was raised at all: a toast expires
    // unseen, so it may accompany an error but never carry one (section 8.5).
    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent('The environment could not be created.');
    expect(screen.queryByRole('button', { name: 'Dismiss' })).not.toBeInTheDocument();
    expect(screen.queryByText('Environment created.')).not.toBeInTheDocument();
  });

  it('writes nothing to browser storage while listing and creating', async () => {
    localStorage.clear();
    sessionStorage.clear();
    stubFetch(handlers({
      'POST /v1/environments': () => json({ environment: environment() }, 201),
    }));
    renderRoute(<EnvironmentsPage />, { path: routePath, entry: routePath });

    await userEvent.type(await screen.findByLabelText('Display name'), 'Lab');
    await userEvent.click(screen.getByRole('button', { name: 'Create environment' }));

    await waitFor(() => { expect(screen.getByText('Environment created.')).toBeVisible(); });
    expect(localStorage).toHaveLength(0);
    expect(sessionStorage).toHaveLength(0);
  });
});
