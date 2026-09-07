import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AuthProvider } from './AuthContext';
import { json, mockApi, session, type MockResponder } from '@shared/testing/harness';
import { expectNoAxeViolations } from '@shared/testing/axe';
import { LoginPage } from './LoginPage';

afterEach(() => vi.unstubAllGlobals());

/** No session yet, which is the state the sign-in screen exists for. */
function signedOut(overrides: Record<string, MockResponder> = {}): Record<string, MockResponder> {
  return {
    'GET /v1/auth/session': () => json({ error: 'unauthorized' }, 401),
    ...overrides,
  };
}

function renderLogin() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}><AuthProvider><MemoryRouter><LoginPage /></MemoryRouter></AuthProvider></QueryClientProvider>,
  );
}

describe('LoginPage', () => {
  it('sends credentials with one selected MFA proof and persists no secrets', async () => {
    const api = mockApi(signedOut({
      'POST /v1/auth/login': () => json({
        csrf_token: 'ccccccccccccccccccccccccccccccccccccccccccc',
        session: session(),
      }),
    }));
    localStorage.clear(); sessionStorage.clear();
    renderLogin();

    await userEvent.type(await screen.findByLabelText('Username'), 'owner');
    await userEvent.type(screen.getByLabelText('Password'), 'correct horse battery staple');
    await userEvent.type(screen.getByLabelText('6-digit authenticator code'), '123456');
    await userEvent.click(screen.getByRole('button', { name: 'Continue securely' }));

    await waitFor(() => { expect(api.called('POST /v1/auth/login')).toHaveLength(1); });
    const login = api.called('POST /v1/auth/login')[0];
    // The session cookie rides on the credentials mode; MSW reports the mode
    // the console asked for, not one a stub was handed.
    expect(login?.credentials).toBe('include');
    expect(login?.body).toEqual({ username: 'owner', password: 'correct horse battery staple', totp_code: '123456' });
    // Exactly one proof travels: the recovery field the operator did not use
    // must not be sent as an empty string alongside the one they did.
    expect(login?.body).not.toHaveProperty('recovery_code');
    expect(localStorage).toHaveLength(0);
    expect(sessionStorage).toHaveLength(0);
  });

  it('carries the screen heading at every width and passes axe', async () => {
    // The headline used to be the `h1` and sits in a column hidden below 900
    // pixels, so the sign-in screen lost its only heading on a narrow viewport
    // and route-change focus had nothing to land on (WCX-05).
    mockApi(signedOut());
    const { container } = renderLogin();

    const heading = await screen.findByRole('heading', { name: 'Sign in', level: 1 });
    expect(heading).toHaveAttribute('tabindex', '-1');
    expect(screen.getByRole('main')).toContainElement(heading);
    expect(screen.queryByRole('heading', { name: /Know what is protected/ })).not.toBeInTheDocument();
    await expectNoAxeViolations(container);
  });

  it('operates the MFA method control with arrow keys and exposes its state', async () => {
    // Section 9.5.3. A segmented control that only responds to Tab and click
    // is operable but not idiomatic; an operator reaching it with the keyboard
    // expects the arrows to move between the options.
    mockApi(signedOut());
    renderLogin();

    const authenticator = await screen.findByRole('button', { name: 'Authenticator' });
    const recovery = screen.getByRole('button', { name: 'Recovery code' });
    expect(authenticator).toHaveAttribute('aria-pressed', 'true');
    expect(recovery).toHaveAttribute('aria-pressed', 'false');

    authenticator.focus();
    await userEvent.keyboard('{ArrowRight}');
    expect(recovery).toHaveFocus();
    expect(recovery).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByLabelText('Recovery code')).toBeVisible();

    await userEvent.keyboard('{ArrowLeft}');
    expect(authenticator).toHaveFocus();
    expect(authenticator).toHaveAttribute('aria-pressed', 'true');

    await userEvent.keyboard('{End}');
    expect(recovery).toHaveFocus();
    await userEvent.keyboard('{Home}');
    expect(authenticator).toHaveFocus();
    expect(screen.getByLabelText('6-digit authenticator code')).toBeVisible();
  });

  it('marks required fields programmatically and names the correction on failure', async () => {
    // Sections 9.6.4 and 9.6.5. `required` is the programmatic marker; the
    // message says what to do, not only that something went wrong.
    mockApi(signedOut({ 'POST /v1/auth/login': () => json({ error: 'unauthorized' }, 401) }));
    renderLogin();

    for (const label of ['Username', 'Password', '6-digit authenticator code']) {
      expect(await screen.findByLabelText(label), label).toBeRequired();
    }

    await userEvent.type(screen.getByLabelText('Username'), 'owner');
    await userEvent.type(screen.getByLabelText('Password'), 'correct horse battery staple');
    await userEvent.type(screen.getByLabelText('6-digit authenticator code'), '123456');
    await userEvent.click(screen.getByRole('button', { name: 'Continue securely' }));

    const alerts = await screen.findAllByRole('alert');
    // Section 9.7.2: one assertive region, not one per field.
    expect(alerts).toHaveLength(1);
    expect(alerts[0]).toHaveTextContent('Check your credentials and MFA proof.');
    // Every field the message covers points at it and is marked invalid.
    for (const label of ['Username', 'Password', '6-digit authenticator code']) {
      const input = screen.getByLabelText(label);
      expect(input, label).toHaveAttribute('aria-invalid', 'true');
      expect(input, label).toHaveAccessibleDescription('Sign-in was denied. Check your credentials and MFA proof.');
    }
  });

  it('renders a generic denied state without reflecting submitted secrets', async () => {
    mockApi(signedOut({ 'POST /v1/auth/login': () => json({ error: 'unauthorized' }, 401) }));
    renderLogin();
    await userEvent.type(await screen.findByLabelText('Username'), 'owner');
    await userEvent.type(screen.getByLabelText('Password'), 'never-reflect-this-password');
    await userEvent.type(screen.getByLabelText('6-digit authenticator code'), '654321');
    await userEvent.click(screen.getByRole('button', { name: 'Continue securely' }));
    const error = await screen.findByRole('alert');
    expect(error).toHaveTextContent('Sign-in was denied.');
    expect(error).not.toHaveTextContent('never-reflect-this-password');
    expect(error).not.toHaveTextContent('654321');
  });
});
