import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { environmentID, json, loginHandlers, mockApi, renderRoute } from '@shared/testing/harness';
import { expectNoAxeViolations } from '@shared/testing/axe';
import { HOSTILE_CORPUS } from '@shared/hostile/corpus';
import { EnrollmentTokenPanel } from './EnrollmentTokenPanel';
import { enrollmentTokenState } from './api';
import type { EnrollmentToken } from '@shared/api/types';
import { untrusted } from '@shared/api/untrusted';

/**
 * Enrollment token management (WCX-09 sections 8.9, 9.4, 9.5.4, test 10.1.10).
 *
 * The load-bearing assertion is the negative one: no token value appears in
 * the DOM. It cannot, because the contract's summary has no field for one —
 * but a future change that added a field would find this test in the way.
 */
const LIST = `GET /v1/environments/${environmentID}/enrollment-tokens`;
const OPEN_TOKEN = '018f1f7e-6d31-7cc5-8db8-17547f78e6f1';
const CLOSED_TOKEN = '018f1f7e-6d31-7cc5-8db8-17547f78e6f2';
const REVOKE = `DELETE /v1/environments/${environmentID}/enrollment-tokens/${OPEN_TOKEN}`;

/** The wire shape: `device_name` is a plain string until the API taints it. */
function tokenRow(overrides: Record<string, unknown> = {}) {
  return {
    token_id: OPEN_TOKEN,
    device_id: '018f1f7e-6d31-7cc5-8db8-17547f78e6c2',
    environment_id: environmentID,
    device_name: 'edge-two',
    // Far enough out that the row is open however slowly the suite runs.
    expires_at: '2099-01-01T00:00:00Z',
    ...overrides,
  };
}

async function openPanel(rows = [tokenRow()], extra = {}) {
  const api = mockApi({
    ...loginHandlers(),
    [LIST]: () => json({ tokens: rows }),
    ...extra,
  });
  const view = renderRoute(
    <EnrollmentTokenPanel environmentID={environmentID} />,
    { path: '/environments/:environmentId', entry: `/environments/${environmentID}` },
  );
  await screen.findByRole('table', { name: /Enrollment tokens for this environment/ });
  return { api, ...view };
}

describe('the token list', () => {
  it('shows no token value anywhere in the DOM', async () => {
    // Section 8.9 and test 10.1.10. The Control Plane never sends one to a
    // list, so there is nothing here to leak — and a change that started
    // sending one would fail here rather than reaching an operator's screen.
    const secret = 'this-would-be-a-token-value';
    await openPanel([tokenRow({ token: secret })]);

    expect(document.body.textContent).not.toContain(secret);
    expect(document.body.innerHTML).not.toContain(secret);
  });

  it('is a table with a caption and header cells', async () => {
    await openPanel();
    for (const column of ['Device name', 'State', 'Expires', 'Action']) {
      expect(screen.getByRole('columnheader', { name: column })).toBeVisible();
    }
  });

  it('keeps a closed handoff visible rather than removing it', async () => {
    // Section 9.5.4. A window that closed is the first thing an operator
    // wants when an Edge failed to enrol.
    await openPanel([
      tokenRow(),
      tokenRow({ token_id: CLOSED_TOKEN, device_name: 'edge-three', consumed_at: '2026-08-29T12:00:00Z' }),
    ]);

    expect(screen.getByText('Open')).toBeVisible();
    expect(screen.getByText('Used for enrollment')).toBeVisible();
    const closed = screen.getByRole('button', { name: 'Revoke the token for edge-three' });
    expect(closed).toBeDisabled();
    expect(closed).toHaveAccessibleDescription(/already closed/);
  });

  it('renders a hostile device name as inert text', async () => {
    // The name round-trips through the API, so the console cannot tell an
    // operator's typing from an attacker with a stolen session.
    const hostile = HOSTILE_CORPUS.find((fixture) => fixture.id === 'img-onerror')?.value
      ?? '<img src=x onerror=alert(1)>';
    const { container } = await openPanel([tokenRow({ device_name: hostile })]);

    expect(container.querySelector('img')).toBeNull();
    expect(container.querySelector('script')).toBeNull();
    expect(document.body.textContent).toContain(hostile);
  });

  it('reports no serious or critical axe violation', async () => {
    const { container } = await openPanel();
    await expectNoAxeViolations(container);
  });
});

describe('revoking a token', () => {
  it('is level 2, so it confirms without a step-up', async () => {
    // The action table puts this at L2: destructive but re-issuable.
    const { api } = await openPanel([tokenRow()], {
      [REVOKE]: () => new Response(null, { status: 204 }),
    });

    await userEvent.click(screen.getByRole('button', { name: 'Revoke the token for edge-two' }));
    expect(screen.queryByRole('dialog', { name: 'Confirm it is you' })).toBeNull();

    await userEvent.click(await screen.findByRole('button', { name: 'Revoke enrollment token' }));
    await waitFor(() => { expect(api.called(REVOKE)).toHaveLength(1); });
  });

  it('says nothing changed when the window had already closed', async () => {
    await openPanel([tokenRow()], { [REVOKE]: () => json({ error: 'conflict' }, 409) });

    await userEvent.click(screen.getByRole('button', { name: 'Revoke the token for edge-two' }));
    await userEvent.click(await screen.findByRole('button', { name: 'Revoke enrollment token' }));

    expect(await screen.findByText(/had already closed, so nothing changed/)).toBeVisible();
  });

  it('issues nothing when the confirmation is cancelled', async () => {
    const { api } = await openPanel();
    await userEvent.click(screen.getByRole('button', { name: 'Revoke the token for edge-two' }));
    await screen.findByRole('dialog');
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }));

    expect(api.called(REVOKE)).toHaveLength(0);
  });
});

describe('the derived token state', () => {
  const at = Date.parse('2026-08-29T12:00:00Z');
  const base = (overrides: Partial<EnrollmentToken> = {}): EnrollmentToken => ({
    token_id: OPEN_TOKEN,
    device_id: 'd',
    environment_id: environmentID,
    device_name: untrusted('edge-two'),
    expires_at: '2026-08-29T12:15:00Z',
    ...overrides,
  });

  it.each([
    ['active while the window is open', base(), 'active'],
    ['expired once it has passed', base({ expires_at: '2026-08-29T11:00:00Z' }), 'expired'],
    ['consumed', base({ consumed_at: '2026-08-29T11:30:00Z' }), 'consumed'],
    ['revoked', base({ revoked_at: '2026-08-29T11:30:00Z' }), 'revoked'],
    // A decision outranks the clock: saying "expired" would lose the fact
    // that someone revoked it.
    ['revoked even when also expired', base({ expires_at: '2026-08-29T11:00:00Z', revoked_at: '2026-08-29T10:00:00Z' }), 'revoked'],
    // An unreadable expiry cannot be shown to be current, so it is not.
    ['expired when the expiry is unreadable', base({ expires_at: 'not-a-time' }), 'expired'],
  ] as const)('reads %s', (_name, token, expected) => {
    expect(enrollmentTokenState(token, at)).toBe(expected);
  });
});
