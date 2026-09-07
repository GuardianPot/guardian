import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { csrfToken, environment, environmentID, json } from '@shared/testing/harness';
import { renderApp } from '@shared/testing/renderApp';

/**
 * Restoring write access after a reload (change proposal 0003, `WCX-09`
 * section 9.3).
 *
 * `W11-C3-A` keeps the synchronizer proof in browser memory, so a reload
 * leaves a valid session that cannot change anything. Until now the only way
 * forward was a full sign-in every time — and an operator who finds reloading
 * expensive stops reloading, which in a detection product works against the
 * product itself. That behavioural risk is what the proposal was written
 * about, not a technical one.
 *
 * `signedIn: false` in this harness is exactly that state: the Control Plane
 * accepts the session cookie, and the console holds no proof.
 */
const REISSUED = 'cccccccccccccccccccccccccccccccccccccccccc9';
const RENAME = `PATCH /v1/environments/${environmentID}`;

/** The environment detail screen, which carries a level 1 mutation control. */
async function openReadOnly(handlers = {}) {
  const view = renderApp(`/environments/${environmentID}`, { signedIn: false, handlers });
  await screen.findByText(/Read-only session restored\./);
  // And the screen behind it: the rename control lives inside a data
  // boundary, so the banner is on screen before the control is.
  await screen.findByRole('button', { name: 'Save name' });
  return view;
}

describe('the read-only banner', () => {
  it('offers both restoring write access and a full sign-in', async () => {
    // A refused re-issue must leave a way forward rather than a dead end, so
    // the full path stays on the banner beside the cheap one.
    await openReadOnly();
    expect(screen.getByRole('button', { name: 'Restore write access' })).toBeEnabled();
    expect(screen.getAllByRole('link', { name: 'Re-authenticate' }).length).toBeGreaterThanOrEqual(1);
  });

  it('sends no CSRF token, because not having one is the point', async () => {
    // The one request in the console that deliberately carries no proof. The
    // Control Plane requires the cookie and an exact origin instead.
    const { api } = await openReadOnly({
      'POST /v1/auth/csrf': () => json({ csrf_token: REISSUED }),
    });

    await userEvent.click(screen.getByRole('button', { name: 'Restore write access' }));

    await waitFor(() => { expect(api.called('POST /v1/auth/csrf')).toHaveLength(1); });
    expect(api.header('POST /v1/auth/csrf', 'X-CSRF-Token')).toBeNull();
    // Cookies ride on this, and they are the whole credential here.
    expect(api.called('POST /v1/auth/csrf')[0]?.credentials).toBe('include');
  });

  it('re-enables mutations with the new proof and dismisses itself', async () => {
    const { api } = await openReadOnly({
      'POST /v1/auth/csrf': () => json({ csrf_token: REISSUED }),
      [RENAME]: () => json({ environment: environment({ display_name: 'Renamed', revision: 4 }) }),
    });

    const rename = screen.getByRole('button', { name: 'Save name' });
    expect(rename).toBeDisabled();

    await userEvent.click(screen.getByRole('button', { name: 'Restore write access' }));

    // The banner is the console's statement that it cannot write. Once it can,
    // the statement is false and the banner goes.
    await waitFor(() => {
      expect(screen.queryByText(/Read-only session restored\./)).not.toBeInTheDocument();
    });
    await waitFor(() => {
      expect(screen.getByRole('button', { name: 'Save name' })).toBeEnabled();
    });

    // And the mutation carries the re-issued proof, not the old one.
    await userEvent.clear(screen.getByLabelText('Display name'));
    await userEvent.type(screen.getByLabelText('Display name'), 'Renamed');
    await userEvent.click(screen.getByRole('button', { name: 'Save name' }));

    await waitFor(() => { expect(api.called(RENAME)).toHaveLength(1); });
    expect(api.header(RENAME, 'X-CSRF-Token')).toBe(REISSUED);
    expect(api.header(RENAME, 'X-CSRF-Token')).not.toBe(csrfToken);
  });

  it('keeps the proof out of every storage area', async () => {
    // Constraint 4: the re-issued proof lives exactly where the sign-in path's
    // does, which is memory. Storage, URLs, and the query cache are excluded.
    const { queryClient, router } = await openReadOnly({
      'POST /v1/auth/csrf': () => json({ csrf_token: REISSUED }),
    });

    await userEvent.click(screen.getByRole('button', { name: 'Restore write access' }));
    await waitFor(() => {
      expect(screen.queryByText(/Read-only session restored\./)).not.toBeInTheDocument();
    });

    expect(localStorage).toHaveLength(0);
    expect(sessionStorage).toHaveLength(0);
    expect(document.body.innerHTML).not.toContain(REISSUED);
    expect(router.state.location.search).not.toContain(REISSUED);
    const cached = JSON.stringify(queryClient.getQueryCache().getAll().map((entry) => entry.state.data));
    expect(cached).not.toContain(REISSUED);
  });

  it('ends the session when the Control Plane refuses the cookie', async () => {
    // A 401 here means the session was not valid after all, so the console
    // takes the existing expiry path rather than inventing a third state. An
    // operator left sitting on a read-only banner for a session that no longer
    // exists is being told something untrue.
    const { api } = await openReadOnly({
      'POST /v1/auth/csrf': () => json({ error: 'unauthorized' }, 401),
    });

    await userEvent.click(screen.getByRole('button', { name: 'Restore write access' }));

    expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeVisible();
    expect(api.called(RENAME)).toHaveLength(0);
  });

  it('names rate limiting, stays read-only, and does not retry', async () => {
    // A throttled re-issue leaves the session exactly as it was: still valid,
    // still unable to write. The console never retries an authentication.
    const { api } = await openReadOnly({
      'POST /v1/auth/csrf': () => json({ error: 'rate_limited' }, 429),
    });

    await userEvent.click(screen.getByRole('button', { name: 'Restore write access' }));

    expect(await screen.findByText('Too many attempts. Wait before trying again, or sign in.')).toBeVisible();
    expect(screen.getByText(/Read-only session restored\./)).toBeVisible();
    expect(screen.getByRole('button', { name: 'Save name' })).toBeDisabled();
    expect(api.called('POST /v1/auth/csrf')).toHaveLength(1);
    expect(api.called(RENAME)).toHaveLength(0);
  });
});

describe('what restoring does not do', () => {
  it('leaves level 3 gated behind step-up', async () => {
    // Change proposal 0003 keeps these separate on purpose. A fresh proof says
    // the browser has a session; it is not evidence that the operator is still
    // the one holding it.
    await openReadOnly({ 'POST /v1/auth/csrf': () => json({ csrf_token: REISSUED }) });
    await userEvent.click(screen.getByRole('button', { name: 'Restore write access' }));
    await waitFor(() => {
      expect(screen.queryByText(/Read-only session restored\./)).not.toBeInTheDocument();
    });

    // A level 3 control on this screen: zone delete is level 2, so the device
    // screen owns the irreversible ones. What is asserted here is the seam —
    // the confirmation table still puts them at 3 regardless of proof age.
    const { ACTION_CONFIRMATION } = await import('@shared/ui');
    for (const action of ['device.revoke', 'session.revoke', 'account.password'] as const) {
      expect(ACTION_CONFIRMATION[action].level, action).toBe(3);
    }
  });
});
