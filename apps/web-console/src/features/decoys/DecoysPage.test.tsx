import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, describe, expect, it } from 'vitest';
import { decoyID, decoyView, environmentID, json, zone, zoneID } from '@shared/testing/harness';
import { renderApp } from '@shared/testing/renderApp';

const ENTRY = `/environments/${environmentID}/decoys`;

/**
 * The whole application, at the decoy route.
 *
 * `renderApp` mounts the real route tree on a data router, which is what the
 * unsaved-changes guard needs and what the shell gives a screen in production.
 * A component-only harness would prove the table renders and nothing about
 * whether the route reaches it.
 */
function renderDecoys(overrides: Record<string, () => Response> = {}) {
  return renderApp(ENTRY, {
    handlers: {
      [`GET /v1/environments/${environmentID}/zones?limit=200`]: () =>
        json({ zones: [zone({ zone_id: zoneID, cidr: '10.20.0.0/24' })] }),
      [`GET /v1/environments/${environmentID}/decoys?limit=200`]: () =>
        json({ decoys: [decoyView()] }),
      ...overrides,
    },
  });
}

afterEach(() => {
  localStorage.clear();
  sessionStorage.clear();
});

describe('the decoy list tells the truth about coverage', () => {
  /**
   * Section 10.1.8 and 8.7. The default fixture is a decoy nothing has reported
   * on. It must read `Unknown` — not deployed because someone asked for it, and
   * not absent because absence is a positive claim an Edge has to make.
   */
  it('shows a decoy nothing has reported on as unknown, never healthy', async () => {
    renderDecoys();
    const row = await screen.findByRole('row', { name: /Finance file server/ });
    expect(within(row).getAllByText('Unknown').length).toBeGreaterThan(0);
    expect(within(row).queryByText('Deployed')).not.toBeInTheDocument();
    expect(screen.getByText(/Nothing has reported on this decoy yet/)).toBeInTheDocument();
  });

  /**
   * Section 10.1.7. `Never` would assert that no attacker has touched this
   * decoy. Nobody established that; the observation is simply missing.
   */
  it('renders a missing interaction as unknown rather than never', async () => {
    renderDecoys();
    const row = await screen.findByRole('row', { name: /Finance file server/ });
    expect(within(row).queryByText(/\bNever\b/)).not.toBeInTheDocument();
    expect(within(row).getAllByText('Unknown').length).toBeGreaterThan(0);
  });

  /** Section 10.1.6 and 9.3.1: two columns, and the health one is not the
   * configuration one. */
  it('keeps observed state and configuration convergence in separate columns', async () => {
    renderDecoys();
    await screen.findByRole('row', { name: /Finance file server/ });
    expect(screen.getByRole('columnheader', { name: 'Observed' })).toBeInTheDocument();
    expect(screen.getByRole('columnheader', { name: 'Configuration' })).toBeInTheDocument();
    for (const column of ['Decoy', 'Type and persona', 'Address and zone', 'Pack version', 'Last interaction']) {
      expect(screen.getByRole('columnheader', { name: column })).toBeInTheDocument();
    }
  });

  /**
   * Section 10.1.15 and SEC-06. The decoy stays visible, is marked unmanaged,
   * and is never presented as healthy or as something Guardian still drives.
   */
  it('keeps a revoked device’s decoy visible and marks it unmanaged', async () => {
    renderDecoys({
      [`GET /v1/environments/${environmentID}/decoys?limit=200`]: () =>
        json({
          decoys: [decoyView({ observed: { observed_state: 'unmanaged', reported_at: '2026-09-08T11:00:00.000Z' } })],
        }),
    });
    const row = await screen.findByRole('row', { name: /Finance file server/ });
    expect(within(row).getByText('Unmanaged')).toBeInTheDocument();
    expect(within(row).queryByText('Deployed')).not.toBeInTheDocument();
    expect(screen.getByText(/Guardian can no longer manage this decoy/)).toBeInTheDocument();
    // Convergence is not asked about a decoy the Control Plane cannot drive.
    expect(screen.getByText(/Not applicable while Guardian cannot manage/)).toBeInTheDocument();
  });

  /** Section 10.1.16 and AC-SMB-002: a persona is an emulation and says so. */
  it('labels every persona as emulated', async () => {
    renderDecoys();
    const row = await screen.findByRole('row', { name: /Finance file server/ });
    expect(within(row).getByText('Emulated')).toBeInTheDocument();
  });

  /**
   * Section 9.5 and the convergence rule. Past the window the console reports a
   * wait; it does not invent a failure it was never told about.
   */
  it('reports an overdue convergence as a wait, not as a failure', async () => {
    renderDecoys({
      [`GET /v1/environments/${environmentID}/decoys?limit=200`]: () =>
        json({
          decoys: [decoyView({
            decoy: { revision: 4, updated_at: new Date(Date.now() - 600_000).toISOString() },
            observed: { desired_revision: 3 },
          })],
        }),
    });
    await screen.findByRole('row', { name: /Finance file server/ });
    expect(screen.getByText(/Guardian has not been told anything failed/)).toBeInTheDocument();
  });

  /**
   * Section 10.1.10. A decoy's display name round-trips through the API, so the
   * console cannot tell an operator's typing from an attacker with a stolen
   * session. It renders as text and creates no markup.
   */
  it('renders a hostile display name inert', async () => {
    const hostile = '<img src=x onerror="alert(1)">';
    const { container } = renderDecoys({
      [`GET /v1/environments/${environmentID}/decoys?limit=200`]: () =>
        json({ decoys: [decoyView({ decoy: { display_name: hostile } })] }),
    });
    await screen.findByRole('table');
    expect(container.querySelector('img')).toBeNull();
    expect(document.body.textContent).toContain('onerror');
  });
});

describe('decoy lifecycle', () => {
  /**
   * Section 10.1.3 and 9.1.4. A failed enable leaves the displayed state where
   * it was and says nothing changed. Flipping the badge optimistically and
   * flipping it back is how a console ends up disagreeing with the product.
   */
  it('applies no optimistic update when a transition fails', async () => {
    renderDecoys({
      [`POST /v1/environments/${environmentID}/decoys/${decoyID}/disable`]: () =>
        json({ status: 'internal_error' }, 500),
    });
    const user = userEvent.setup();
    await screen.findByRole('row', { name: /Finance file server/ });
    await user.click(screen.getByRole('button', { name: /Disable Finance file server/ }));

    expect(await screen.findByText(/The disable request failed. Nothing changed./)).toBeInTheDocument();
    // The row still offers to disable, because nothing was disabled.
    expect(screen.getByRole('button', { name: /Disable Finance file server/ })).toBeInTheDocument();
  });

  /** Section 9.9.1: a revision conflict is its own state and writes nothing. */
  it('renders a revision conflict as a conflict and offers the current value', async () => {
    renderDecoys({
      [`POST /v1/environments/${environmentID}/decoys/${decoyID}/disable`]: () =>
        json({ status: 'precondition_failed', code: 'decoy.revision.stale' }, 412),
    });
    const user = userEvent.setup();
    await screen.findByRole('row', { name: /Finance file server/ });
    await user.click(screen.getByRole('button', { name: /Disable Finance file server/ }));

    expect(await screen.findByText(/Another change was recorded first/)).toBeInTheDocument();
    expect(screen.getByRole('button', { name: 'Reload current configuration' })).toBeInTheDocument();
  });

  /** Section 10.1.13 and the 9.5 table: removal is level 2 and names the decoy. */
  it('confirms removal at level 2, naming the decoy and keeping its history', async () => {
    renderDecoys();
    const user = userEvent.setup();
    await screen.findByRole('row', { name: /Finance file server/ });
    await user.click(screen.getByRole('button', { name: /Remove Finance file server/ }));

    const dialog = await screen.findByRole('dialog');
    // The confirm control names the effect, never `OK` (WCX-04 section 9.3).
    expect(within(dialog).getByRole('button', { name: 'Remove decoy' })).toBeInTheDocument();
    // The object is named, and CS-06's guarantee is stated: removal retires a
    // decoy, it does not erase what Guardian recorded about it.
    expect(dialog).toHaveTextContent(/“Finance file server”/);
    expect(dialog).toHaveTextContent(/Anything Guardian already recorded about it is kept/);
    // Level 2, not level 3: no typed object name and no step-up.
    expect(within(dialog).queryByLabelText('Object name')).not.toBeInTheDocument();
  });

  /** Enable and disable are level 1: reversible, so no modal stands in the way. */
  it('does not confirm a reversible transition', async () => {
    renderDecoys({
      [`POST /v1/environments/${environmentID}/decoys/${decoyID}/disable`]: () =>
        json(decoyView({ decoy: { desired_state: 'disabled', revision: 4 } })),
    });
    const user = userEvent.setup();
    await screen.findByRole('row', { name: /Finance file server/ });
    await user.click(screen.getByRole('button', { name: /Disable Finance file server/ }));

    await waitFor(() => {
      expect(screen.getByText(/Disable requested/)).toBeInTheDocument();
    });
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
  });

  /** Section 8.5 and 10.1.12: nothing about a decoy is written to storage. */
  it('writes nothing to browser storage', async () => {
    renderDecoys();
    const user = userEvent.setup();
    await screen.findByRole('row', { name: /Finance file server/ });
    await user.click(screen.getByRole('button', { name: 'Deploy decoy' }));
    await user.type(screen.getByLabelText('Display name'), 'Draft name');

    expect(localStorage.length).toBe(0);
    expect(sessionStorage.length).toBe(0);
  });
});

describe('the decoy configuration form', () => {
  async function openForm() {
    const view = renderDecoys({
      [`POST /v1/environments/${environmentID}/decoys`]: () =>
        json({
          status: 'invalid_request',
          code: 'decoy.request.invalid',
          field_errors: [{ field: 'address', code: 'outside_zone' }],
          request_id: 'a'.repeat(32),
        }, 400),
    });
    const user = userEvent.setup();
    await screen.findByRole('row', { name: /Finance file server/ });
    await user.click(screen.getByRole('button', { name: 'Deploy decoy' }));
    return { ...view, user };
  }

  /**
   * Section 8.3, reviewed as security-critical wording. The warning is
   * programmatically associated with the control, not a visual adornment
   * beside it (section 9.7.3).
   */
  it('associates the attacker-visibility warning with the fields it applies to', async () => {
    const { user } = await openForm();
    await user.click(screen.getByRole('button', { name: 'Deploy decoy' }));

    for (const label of ['Display name', 'Address']) {
      const field = screen.getByLabelText(label);
      const describedBy = field.getAttribute('aria-describedby') ?? '';
      const described = describedBy.split(' ').map((id) => document.getElementById(id)?.textContent ?? '').join(' ');
      expect(described).toMatch(/Attackers can see this value/);
      expect(described).toMatch(/Never enter a real credential/);
    }
  });

  /**
   * Section 10.1.1 and 8.1, and the reason change proposal 0004 exists.
   *
   * The submitted address is a perfectly well-formed private IPv4 address, so
   * client validation passes it. The Control Plane rejects it anyway, because
   * only the Control Plane knows the zone's prefix. The rejection is rendered
   * truthfully, and on the field the backend named.
   */
  it('renders a client-valid, server-invalid submission on the field the backend named', async () => {
    const { user } = await openForm();
    await user.type(screen.getByLabelText('Display name'), 'Finance file server');
    await user.type(screen.getByLabelText('Address'), '10.99.0.40');
    await user.type(screen.getByLabelText('Pack'), 'smb-fileshare');
    await user.type(screen.getByLabelText('Pack version'), '0.1.0');
    await user.click(screen.getAllByRole('button', { name: 'Deploy decoy' }).at(-1)!);

    const address = await screen.findByLabelText('Address');
    await waitFor(() => {
      expect(address).toHaveAttribute('aria-invalid', 'true');
    });
    const describedBy = address.getAttribute('aria-describedby') ?? '';
    const described = describedBy.split(' ').map((id) => document.getElementById(id)?.textContent ?? '').join(' ');
    expect(described).toMatch(/This address is not inside the selected zone/);
  });

  /** Section 8.9: no surface states or implies a real host of this kind. */
  it('says the persona is an emulation', async () => {
    await openForm();
    expect(
      screen.getByText(/Every persona is an emulation presented by Guardian/),
    ).toBeInTheDocument();
  });
});
