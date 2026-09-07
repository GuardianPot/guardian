import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { csrfToken, json, loginHandlers, mockApi, renderRoute, session } from '@shared/testing/harness';
import { expectNoAxeViolations } from '@shared/testing/axe';
import type { Session } from '@shared/api/types';
import { AccountPage } from './AccountPage';

/**
 * The account area (WCX-09 sections 8.6-8.7, 9.5.3, 9.5.6, tests 10.1.7-8).
 *
 * The two properties worth the most here are both about not lying: revoking
 * another session must leave this one alone, and a password change must report
 * only what the response confirmed.
 */
const OTHER_SESSION = '018f1f7e-6d31-7cc5-8db8-17547f78e6e1';
const REVOKED_SESSION = '018f1f7e-6d31-7cc5-8db8-17547f78e6e2';

function other(overrides: Partial<Session> = {}): Session {
  return {
    ...session(),
    session_id: OTHER_SESSION,
    current: false,
    created_at: '2026-08-29T09:00:00Z',
    last_seen_at: '2026-08-29T11:55:00Z',
    ...overrides,
  };
}

function sessions(): Session[] {
  return [
    session(),
    other(),
    other({ session_id: REVOKED_SESSION, revoked_at: '2026-08-29T10:00:00Z' }),
  ];
}

async function openAccount(extra = {}) {
  const api = mockApi({
    ...loginHandlers(),
    'GET /v1/auth/sessions': () => json({ sessions: sessions() }),
    ...extra,
  });
  const view = renderRoute(<AccountPage />, { path: '/account', entry: '/account' });
  // The table, not the panel heading: the panel renders while its data
  // boundary is still loading, so waiting on the heading proves nothing.
  await screen.findByRole('table', { name: /Guardian sessions for this owner/ });
  return { api, ...view };
}

async function passStepUp() {
  await screen.findByRole('dialog', { name: 'Confirm it is you' });
  await userEvent.type(screen.getByLabelText('Username'), 'owner');
  await userEvent.type(screen.getByLabelText('Password'), 'correct horse battery staple');
  await userEvent.type(screen.getByLabelText('6-digit authenticator code'), '123456');
  await userEvent.click(screen.getByRole('button', { name: 'Reauthenticate' }));
}

describe('the session list', () => {
  it('is a table with a caption and header cells', async () => {
    // Section 9.6.5. Several facts about several rows is tabular data, and a
    // screen reader needs the header association to say which is which.
    await openAccount();
    const table = screen.getByRole('table', { name: /Guardian sessions for this owner/ });
    expect(table).toBeVisible();
    for (const column of ['Session', 'Signed in', 'Last seen', 'Expires', 'Action']) {
      expect(screen.getByRole('columnheader', { name: column })).toBeVisible();
    }
  });

  it('labels the current session and gives it no revoke control', async () => {
    // Sections 8.6 and 9.5.3. Two paths to ending this session is one path too
    // many when the other rows are the ones an operator is aiming at.
    await openAccount();
    expect(screen.getByText('This session')).toBeVisible();
    expect(screen.getByText('Use Sign out to end this session.')).toBeVisible();
    expect(screen.queryByRole('button', { name: `Revoke session ${session().session_id}` })).toBeNull();
  });

  it('keeps a revoked session listed rather than removing it', async () => {
    await openAccount();
    expect(screen.getByText('Revoked')).toBeVisible();
    const control = screen.getByRole('button', { name: `Revoke session ${REVOKED_SESSION}` });
    expect(control).toBeDisabled();
    expect(control).toHaveAccessibleDescription(/already ended/);
  });

  it('names the row in every row action', async () => {
    // Section 9.6.5: "Revoke" alone is never an accessible name.
    await openAccount();
    expect(screen.getByRole('button', { name: `Revoke session ${OTHER_SESSION}` })).toBeEnabled();
  });

  it('reports no serious or critical axe violation', async () => {
    const { container } = await openAccount();
    await expectNoAxeViolations(container);
  });
});

describe('revoking another session', () => {
  it('leaves the current session intact', async () => {
    // Section 10.1.7. The request targets one id and nothing else changes.
    const { api } = await openAccount({
      [`DELETE /v1/auth/sessions/${OTHER_SESSION}`]: () => new Response(null, { status: 204 }),
    });

    await userEvent.click(screen.getByRole('button', { name: `Revoke session ${OTHER_SESSION}` }));
    await passStepUp();
    await userEvent.type(await screen.findByLabelText('Object name'), OTHER_SESSION);
    await userEvent.click(screen.getByRole('button', { name: 'Revoke session' }));

    await waitFor(() => {
      expect(api.called(`DELETE /v1/auth/sessions/${OTHER_SESSION}`)).toHaveLength(1);
    });
    expect(api.called(`DELETE /v1/auth/sessions/${session().session_id}`)).toHaveLength(0);
    expect(api.header(`DELETE /v1/auth/sessions/${OTHER_SESSION}`, 'X-CSRF-Token')).toBe(csrfToken);
    // Still signed in, still showing this session.
    expect(screen.getByText('This session')).toBeVisible();
  });

  it('says the session is still active when the revoke fails', async () => {
    // Section 9.5.5: never leave an operator unsure whether access was removed.
    await openAccount({
      [`DELETE /v1/auth/sessions/${OTHER_SESSION}`]: () => json({ error: 'forbidden' }, 403),
    });

    await userEvent.click(screen.getByRole('button', { name: `Revoke session ${OTHER_SESSION}` }));
    await passStepUp();
    await userEvent.type(await screen.findByLabelText('Object name'), OTHER_SESSION);
    await userEvent.click(screen.getByRole('button', { name: 'Revoke session' }));

    expect(await screen.findByText(/Guardian refused to revoke that session\. It is still active\./)).toBeVisible();
  });

  it('issues nothing when step-up is cancelled', async () => {
    const { api } = await openAccount();
    await userEvent.click(screen.getByRole('button', { name: `Revoke session ${OTHER_SESSION}` }));
    await screen.findByRole('dialog', { name: 'Confirm it is you' });
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }));

    expect(api.called(`DELETE /v1/auth/sessions/${OTHER_SESSION}`)).toHaveLength(0);
  });
});

describe('changing the password', () => {
  const rotated = {
    csrf_token: 'cccccccccccccccccccccccccccccccccccccccccc2',
    session: { ...session(), session_id: '018f1f7e-6d31-7cc5-8db8-17547f78e6e9' },
  };

  async function submitChange() {
    await userEvent.type(screen.getByLabelText('Current password'), 'correct horse battery staple');
    await userEvent.type(screen.getByLabelText('New password'), 'a much longer replacement');
    await userEvent.click(screen.getByRole('button', { name: 'Change password' }));
    await passStepUp();
    await userEvent.type(await screen.findByLabelText('Object name'), 'owner');
    await userEvent.click(screen.getByRole('button', { name: 'Change password' }));
  }

  it('requires step-up and typed confirmation before it is sent', async () => {
    const { api } = await openAccount();
    await userEvent.type(screen.getByLabelText('Current password'), 'correct horse battery staple');
    await userEvent.type(screen.getByLabelText('New password'), 'a much longer replacement');
    await userEvent.click(screen.getByRole('button', { name: 'Change password' }));

    await screen.findByRole('dialog', { name: 'Confirm it is you' });
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(api.called('POST /v1/auth/password')).toHaveLength(0);
  });

  it('reports only what the response confirmed', async () => {
    // Sections 8.7 and 9.5.6. The Control Plane sends credentials, not a list
    // of what it revoked, so the message points at the list rather than
    // claiming a count.
    const { api } = await openAccount({
      'POST /v1/auth/password': () => json(rotated),
    });
    await submitChange();

    await waitFor(() => { expect(api.called('POST /v1/auth/password')).toHaveLength(1); });
    const message = await screen.findByText(/Password changed and this session was replaced/);
    expect(message).toBeVisible();
    expect(message.textContent).not.toMatch(/\b\d+ (?:other )?sessions?\b/);
  });

  it('sends the passwords in the body and never in a URL', async () => {
    const { api } = await openAccount({ 'POST /v1/auth/password': () => json(rotated) });
    await submitChange();

    await waitFor(() => { expect(api.called('POST /v1/auth/password')).toHaveLength(1); });
    const [call] = api.called('POST /v1/auth/password');
    expect(call?.body).toEqual({
      current_password: 'correct horse battery staple',
      new_password: 'a much longer replacement',
    });
    expect(call?.request.url).not.toContain('replacement');
  });

  it('says nothing changed when the backend rejects the new password', async () => {
    await openAccount({ 'POST /v1/auth/password': () => json({ error: 'invalid_request' }, 400) });
    await submitChange();

    expect(await screen.findByText(/does not meet policy, so nothing changed/)).toBeVisible();
    expect(screen.queryByText(/Password changed/)).toBeNull();
  });

  it('writes nothing to browser storage along the way', async () => {
    await openAccount({ 'POST /v1/auth/password': () => json(rotated) });
    await submitChange();
    await screen.findByText(/Password changed/);

    expect(localStorage).toHaveLength(0);
    expect(sessionStorage).toHaveLength(0);
  });
});
