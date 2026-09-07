import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import {
  csrfToken,
  deviceID,
  device,
  environmentID,
  healthView,
  json,
  loginHandlers,
  mockApi,
  renderRoute,
} from '@shared/testing/harness';
import { DevicePage } from './DevicePage';

/**
 * Device lifecycle (WCX-09 sections 8.10-8.12, 9.4, 9.5, and tests 10.1.2-15).
 *
 * The property under test throughout is that the console never claims an
 * outcome it was not told about. A revoke that failed leaves the device shown
 * exactly as it was; a re-enrollment token that was issued does not make the
 * device active; a step-up that was cancelled issues no request at all.
 */
const DETAIL = `/v1/environments/${environmentID}/devices/${deviceID}`;
const ROUTE = {
  path: '/environments/:environmentId/devices/:deviceId',
  entry: `/environments/${environmentID}/devices/${deviceID}`,
};

/** `204 No Content`, which is what every lifecycle transition returns. */
const noContent = () => new Response(null, { status: 204 });

function handlers(state: 'pending' | 'active' | 'disabled' | 'revoked' = 'active') {
  return {
    ...loginHandlers(),
    [`GET ${DETAIL}`]: () => json({ device: device({ state }) }),
    [`GET /v1/devices/${deviceID}/health`]: () => json(healthView()),
    [`POST ${DETAIL}/disable`]: noContent,
    [`POST ${DETAIL}/revoke`]: noContent,
  };
}

/** Signs in, renders the device screen, and waits for the lifecycle panel. */
async function openDevice(state: Parameters<typeof handlers>[0] = 'active', extra = {}) {
  const api = mockApi({ ...handlers(state), ...extra });
  const view = renderRoute(<DevicePage />, ROUTE);
  await screen.findByRole('heading', { name: 'Device lifecycle', level: 2 });
  return { api, ...view };
}

/** Completes the step-up prompt with the credentials the harness accepts. */
async function passStepUp() {
  await screen.findByRole('dialog', { name: 'Confirm it is you' });
  await userEvent.type(screen.getByLabelText('Username'), 'owner');
  await userEvent.type(screen.getByLabelText('Password'), 'correct horse battery staple');
  await userEvent.type(screen.getByLabelText('6-digit authenticator code'), '123456');
  await userEvent.click(screen.getByRole('button', { name: 'Reauthenticate' }));
}

describe('the transitions on offer', () => {
  it.each([
    ['pending', ['Re-enroll device']],
    ['active', ['Disable device', 'Revoke device', 'Re-enroll device']],
    ['disabled', ['Revoke device', 'Re-enroll device']],
    ['revoked', ['Re-enroll device']],
  ] as const)('offers %s exactly the transitions the contract has', async (state, available) => {
    await openDevice(state);
    // WC-D07: every transition renders in every state. A control the contract
    // does not offer from here is disabled with the reason, never hidden.
    for (const label of ['Disable device', 'Revoke device', 'Re-enroll device']) {
      const control = screen.getByRole('button', { name: label });
      expect(control, label).toBeInTheDocument();
      if (available.includes(label as never)) expect(control, label).toBeEnabled();
      else {
        expect(control, label).toBeDisabled();
        expect(control, label).toHaveAccessibleDescription(/Control Plane offers no|not completed enrollment|revoked/);
      }
    }
  });

  it('states each effect before the control is used, not only in the modal', async () => {
    // Section 9.5.1. By the time a modal is open the operator has decided.
    await openDevice('active');
    expect(screen.getByText(/Blocks new authenticated sessions from this Edge/)).toBeVisible();
    expect(screen.getByText(/Permanently withdraws trust and revokes every active certificate/)).toBeVisible();
    expect(screen.getByText(/Issues one new 15-minute enrollment token/)).toBeVisible();
  });

  it('keeps a revoked device visible with its state and the time it changed', async () => {
    // Section 9.5.2 and 10.1.12. Removing it would hide the history, and it is
    // never described as reachable or healthy.
    await openDevice('revoked');
    expect(screen.getByText(/This device was revoked/)).toBeVisible();
    expect(screen.getByText('revoked')).toBeVisible();
    expect(document.body.textContent).not.toMatch(/\b(reachable|healthy|online|connected)\b/i);
  });

  it('says re-enrollment leaves decoys unmanaged, matching SEC-06', async () => {
    await openDevice('revoked');
    expect(screen.getByText(/decoys on this Edge stay unmanaged until re-enrollment completes/)).toBeVisible();
  });
});

describe('a level 3 transition', () => {
  it('issues no request when step-up is cancelled', async () => {
    // Section 10.1.2. Cancelling must not leave a half-authorised action.
    const { api } = await openDevice('active');
    await userEvent.click(screen.getByRole('button', { name: 'Revoke device' }));

    await screen.findByRole('dialog', { name: 'Confirm it is you' });
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Reauthentication was not completed');
    expect(api.called(`POST ${DETAIL}/revoke`)).toHaveLength(0);
  });

  it('issues no request when step-up is denied', async () => {
    const { api } = await openDevice('active');
    // Only the step-up login is refused. Overriding before the screen renders
    // would break the sign-in the harness itself needs.
    api.override('POST /v1/auth/login', () => json({ error: 'authentication_denied' }, 401));
    await userEvent.click(screen.getByRole('button', { name: 'Revoke device' }));
    await passStepUp();

    expect(await screen.findByText('Sign-in was denied. Check your credentials and MFA proof.')).toBeVisible();
    expect(api.called(`POST ${DETAIL}/revoke`)).toHaveLength(0);
  });

  it('issues no request until the exact device name is typed', async () => {
    const { api } = await openDevice('active');
    await userEvent.click(screen.getByRole('button', { name: 'Revoke device' }));
    await passStepUp();

    // Two dialogs named 'Revoke device' would be ambiguous; the confirmation
    // is the one carrying the typed field.
    const field = await screen.findByLabelText('Object name');
    await userEvent.type(field, 'edge-on');
    expect(screen.getByRole('button', { name: 'Revoke device' })).toBeDisabled();
    expect(api.called(`POST ${DETAIL}/revoke`)).toHaveLength(0);

    await userEvent.type(field, 'e');
    await userEvent.click(screen.getByRole('button', { name: 'Revoke device' }));
    await waitFor(() => { expect(api.called(`POST ${DETAIL}/revoke`)).toHaveLength(1); });
  });

  it('sends the CSRF proof and no typed value', async () => {
    // Section 8.5: the typed name is compared locally and never transmitted.
    const { api } = await openDevice('active');
    await userEvent.click(screen.getByRole('button', { name: 'Revoke device' }));
    await passStepUp();
    await userEvent.type(await screen.findByLabelText('Object name'), 'edge-one');
    await userEvent.click(screen.getByRole('button', { name: 'Revoke device' }));

    await waitFor(() => { expect(api.called(`POST ${DETAIL}/revoke`)).toHaveLength(1); });
    const [call] = api.called(`POST ${DETAIL}/revoke`);
    expect(call?.headers.get('X-CSRF-Token')).toBe(csrfToken);
    expect(JSON.stringify(call?.body ?? null)).not.toContain('edge-one');
  });

  it('requires a second step-up for a second action', async () => {
    // Section 10.1.3. The mark is single-use, so the prompt returns.
    const { api } = await openDevice('active');

    await userEvent.click(screen.getByRole('button', { name: 'Revoke device' }));
    await passStepUp();
    await userEvent.type(await screen.findByLabelText('Object name'), 'edge-one');
    await userEvent.click(screen.getByRole('button', { name: 'Revoke device' }));
    await waitFor(() => { expect(api.called(`POST ${DETAIL}/revoke`)).toHaveLength(1); });

    await userEvent.click(screen.getByRole('button', { name: 'Re-enroll device' }));
    // A second prompt, not a second free pass.
    expect(await screen.findByRole('dialog', { name: 'Confirm it is you' })).toBeVisible();
    expect(api.called('POST /v1/auth/login').length).toBeGreaterThanOrEqual(2);
  });
});

describe('a destructive action that fails', () => {
  it('leaves the displayed state unchanged and says nothing changed', async () => {
    // Sections 9.5.5, 9.8.3, and test 10.1.5.
    await openDevice('active', {
      [`POST ${DETAIL}/revoke`]: () => json({ error: 'unavailable' }, 503),
    });
    await userEvent.click(screen.getByRole('button', { name: 'Revoke device' }));
    await passStepUp();
    await userEvent.type(await screen.findByLabelText('Object name'), 'edge-one');
    await userEvent.click(screen.getByRole('button', { name: 'Revoke device' }));

    expect(await screen.findByText(/Revoke device did not complete\. Nothing changed on this device\./)).toBeVisible();
    // The inventory badge still says what the last successful read said.
    expect(screen.getByText('active')).toBeVisible();
  });

  it('distinguishes a conflict from a plain failure', async () => {
    await openDevice('active', {
      [`POST ${DETAIL}/disable`]: () => json({ error: 'conflict' }, 409),
    });
    await userEvent.click(screen.getByRole('button', { name: 'Disable device' }));
    await userEvent.click(await screen.findByRole('button', { name: 'Disable device' }));

    expect(await screen.findByText(/Another change reached this device first/)).toBeVisible();
  });
});

describe('re-enrollment', () => {
  const reenroll = `POST ${DETAIL}/re-enrollment-token`;
  const issued = {
    token_id: '018f1f7e-6d31-7cc5-8db8-17547f78e6d0',
    device_id: deviceID,
    environment_id: environmentID,
    device_name: 'edge-one',
    token: 'reenrollment-token-value-shown-once',
    expires_at: '2026-08-29T12:20:00Z',
  };

  /** Runs the full level 3 path and returns the recording API. */
  async function issue(state: 'revoked' | 'active' = 'revoked') {
    const { api, queryClient } = await openDevice(state, { [reenroll]: () => json(issued, 201) });
    await userEvent.click(screen.getByRole('button', { name: 'Re-enroll device' }));
    await passStepUp();
    await userEvent.type(await screen.findByLabelText('Object name'), 'edge-one');
    await userEvent.click(screen.getByRole('button', { name: 'Re-enroll device' }));
    return { api, queryClient };
  }

  it('needs typed confirmation and step-up, and issues nothing without them', async () => {
    // Section 10.1.13.
    const { api } = await openDevice('revoked', { [reenroll]: () => json(issued, 201) });
    await userEvent.click(screen.getByRole('button', { name: 'Re-enroll device' }));
    await screen.findByRole('dialog', { name: 'Confirm it is you' });
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }));
    expect(api.called(reenroll)).toHaveLength(0);
  });

  it('shows the token once and removes it from the DOM on dismissal', async () => {
    // Section 10.1.14.
    await issue();
    const box = await screen.findByTestId('one-time-secret');
    expect(box).toHaveTextContent(issued.token);

    await userEvent.click(screen.getByRole('button', { name: 'I have stored it securely' }));
    await waitFor(() => { expect(screen.queryByTestId('one-time-secret')).not.toBeInTheDocument(); });
    expect(document.body.textContent).not.toContain(issued.token);
  });

  it('puts the token in no query cache and no browser storage', async () => {
    const { queryClient } = await issue();
    await screen.findByTestId('one-time-secret');

    const cached = JSON.stringify(queryClient.getQueryCache().getAll().map((entry) => entry.state.data));
    expect(cached).not.toContain(issued.token);
    expect(localStorage).toHaveLength(0);
    expect(sessionStorage).toHaveLength(0);
  });

  it('does not change the displayed device state', async () => {
    // Section 8.12 and test 10.1.15. Issuing a token is not a restoration. The
    // device becomes active only when the backend reports that it did.
    await issue('revoked');
    await screen.findByTestId('one-time-secret');
    await userEvent.click(screen.getByRole('button', { name: 'I have stored it securely' }));

    expect(screen.getByText('revoked')).toBeVisible();
    expect(screen.queryByText('active')).not.toBeInTheDocument();
  });
});
