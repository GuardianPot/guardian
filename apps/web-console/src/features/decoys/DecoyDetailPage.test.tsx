import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it } from 'vitest';
import { decoyID, decoyView, environmentID, json, zone, zoneID } from '@shared/testing/harness';
import type { DecoyConditionRaw } from '@shared/api/types';
import { renderApp } from '@shared/testing/renderApp';

const ENTRY = `/environments/${environmentID}/decoys/${decoyID}`;

function condition(overrides: Partial<DecoyConditionRaw> = {}): DecoyConditionRaw {
  return {
    type: 'runtime_healthy',
    status: 'Unknown',
    reason: 'not_observed',
    message: '',
    observed_revision: null,
    last_transition_time: '2026-09-08T12:00:00.000Z',
    ...overrides,
  };
}

function renderDetail(overrides: Record<string, () => Response> = {}) {
  return renderApp(ENTRY, {
    handlers: {
      [`GET /v1/environments/${environmentID}/zones?limit=200`]: () =>
        json({ zones: [zone({ zone_id: zoneID, cidr: '10.20.0.0/24' })] }),
      [`GET /v1/environments/${environmentID}/decoys/${decoyID}`]: () => json(decoyView()),
      ...overrides,
    },
  });
}

afterEach(() => {
  localStorage.clear();
  sessionStorage.clear();
});

describe('the decoy detail screen', () => {
  it('keeps desired and observed as two labelled claims', async () => {
    renderDetail();
    await screen.findByRole('heading', { name: 'Finance file server', level: 1 });
    expect(screen.getByText(/Observed: Unknown/)).toBeInTheDocument();
    expect(screen.getByText(/Configuration: Deploy requested/)).toBeInTheDocument();
  });

  /**
   * Section 9.4 and P2-W4. The digest is null until a manifest exists to hash.
   * Rendering a placeholder would assert an artefact identity nothing verified,
   * which is the same failure as reporting an unobserved decoy as healthy.
   */
  it('says no pack digest has been recorded rather than inventing one', async () => {
    renderDetail();
    await screen.findByRole('heading', { name: 'Finance file server', level: 1 });
    expect(screen.getByText(/No digest has been recorded for this pack yet/)).toBeInTheDocument();
  });

  /**
   * The six dimensions stay six. P2-W14's acceptance is that killing the
   * process, removing the address, and breaking telemetry produce distinct
   * degraded states, and a screen that collapsed them would undo that.
   */
  it('reports each observed dimension separately, with its reported reason', async () => {
    renderDetail({
      [`GET /v1/environments/${environmentID}/decoys/${decoyID}`]: () =>
        json(decoyView({
          observed: {
            observed_state: 'degraded',
            reported_at: new Date().toISOString(),
            conditions: [
              condition({ type: 'runtime_healthy', status: 'True', reason: 'running' }),
              condition({ type: 'address_applied', status: 'False', reason: 'address_missing' }),
              condition({ type: 'telemetry_reporting', status: 'Unknown', reason: 'not_observed' }),
            ],
          },
        })),
    });
    const list = await screen.findByRole('list', { name: 'Observed dimensions' });
    const rows = within(list).getAllByRole('listitem');
    expect(rows).toHaveLength(3);
    expect(within(rows[0]!).getByText(/Runtime: Healthy/)).toBeInTheDocument();
    expect(within(rows[1]!).getByText(/Address applied: Action required/)).toBeInTheDocument();
    expect(within(rows[2]!).getByText(/Telemetry: Unknown/)).toBeInTheDocument();
    // The reason is the Edge's, rendered through the untrusted contract.
    expect(within(rows[1]!).getByText(/address_missing/)).toBeInTheDocument();
    // A failing dimension names the operator's next action (section 9.6.3).
    expect(within(rows[1]!).getByText(/Check the reporting Edge/)).toBeInTheDocument();
  });

  /** Section 9.9.3: an old observation states its age instead of looking current. */
  it('says how old a stale observation is', async () => {
    renderDetail({
      [`GET /v1/environments/${environmentID}/decoys/${decoyID}`]: () =>
        json(decoyView({
          observed: {
            observed_state: 'deployed',
            reported_at: new Date(Date.now() - 3_600_000).toISOString(),
            desired_revision: 3,
          },
        })),
    });
    await screen.findByRole('heading', { name: 'Finance file server', level: 1 });
    expect(await screen.findByText(/This observation is .* old, so it may no longer be true/))
      .toBeInTheDocument();
  });

  /** Section 10.1.10: an Edge-supplied condition message renders inert. */
  it('renders a hostile condition message as text', async () => {
    const { container } = renderDetail({
      [`GET /v1/environments/${environmentID}/decoys/${decoyID}`]: () =>
        json(decoyView({
          observed: {
            observed_state: 'degraded',
            reported_at: new Date().toISOString(),
            conditions: [condition({ status: 'False', message: '<img src=x onerror="alert(1)">' })],
          },
        })),
    });
    await screen.findByRole('heading', { name: 'Finance file server', level: 1 });
    expect(container.querySelector('img')).toBeNull();
    expect(document.body.textContent).toContain('onerror');
  });

  /**
   * Section 9.9.1. The form was right and the world moved, so this is a
   * conflict rather than a validation failure, and nothing is written on the
   * strength of the stale revision the form was built from.
   */
  it('renders a stale revision as a conflict and offers the current value', async () => {
    renderDetail({
      [`PATCH /v1/environments/${environmentID}/decoys/${decoyID}`]: () =>
        json({ status: 'precondition_failed', code: 'decoy.revision.stale' }, 412),
    });
    const user = userEvent.setup();
    await screen.findByRole('heading', { name: 'Finance file server', level: 1 });
    await user.click(screen.getByRole('button', { name: 'Edit configuration' }));
    await user.click(screen.getByRole('button', { name: 'Save configuration' }));

    expect(await screen.findByText(/Another change was recorded first/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Reload current configuration' })).toBeInTheDocument();
  });

  /** A saved change says what it did and what it did not do (section 9.6.1). */
  it('says a saved configuration has not been applied yet', async () => {
    renderDetail({
      [`PATCH /v1/environments/${environmentID}/decoys/${decoyID}`]: () =>
        json(decoyView({ decoy: { revision: 4 } })),
    });
    const user = userEvent.setup();
    await screen.findByRole('heading', { name: 'Finance file server', level: 1 });
    await user.click(screen.getByRole('button', { name: 'Edit configuration' }));
    await user.click(screen.getByRole('button', { name: 'Save configuration' }));

    expect(await screen.findByText(/Configuration saved. The Edge has not applied it yet./))
      .toBeInTheDocument();
  });

  /**
   * Section 8.6 and 10.1.4. Editing makes the form dirty; the guard is what
   * warns on the way out, and it stores nothing on the way there.
   */
  it('keeps an edited but unsaved form entirely out of browser storage', async () => {
    renderDetail();
    const user = userEvent.setup();
    await screen.findByRole('heading', { name: 'Finance file server', level: 1 });
    await user.click(screen.getByRole('button', { name: 'Edit configuration' }));
    await user.clear(screen.getByLabelText('Display name'));
    await user.type(screen.getByLabelText('Display name'), 'A draft nobody asked to keep');

    await waitFor(() => {
      expect(screen.getByLabelText('Display name')).toHaveValue('A draft nobody asked to keep');
    });
    expect(localStorage.length).toBe(0);
    expect(sessionStorage.length).toBe(0);
  });
});
