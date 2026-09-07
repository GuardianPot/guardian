import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AuthProvider } from './AuthContext';
import { requestBody, requestURL } from '@shared/testing/harness';
import { expectNoAxeViolations } from '@shared/testing/axe';
import { LoginPage } from './LoginPage';

afterEach(() => vi.unstubAllGlobals());

describe('LoginPage', () => {
  it('sends credentials with one selected MFA proof and persists no secrets', async () => {
    const fetchMock = vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>((input) => {
      if (requestURL(input) === '/v1/auth/session') return Promise.resolve(new Response('{}', { status: 401 }));
      return Promise.resolve(new Response(JSON.stringify({
        csrf_token: 'ccccccccccccccccccccccccccccccccccccccccccc',
        session: {
          session_id: '018f1f7e-6d31-7cc5-8db8-17547f78e6c3', user_id: '018f1f7e-6d31-7cc5-8db8-17547f78e6c4',
          username: 'owner', role: 'owner', created_at: '2026-08-29T12:00:00Z', last_seen_at: '2026-08-29T12:00:00Z',
          expires_at: '2026-08-29T13:00:00Z', current: true,
        },
      }), { status: 200, headers: { 'Content-Type': 'application/json' } }));
    });
    vi.stubGlobal('fetch', fetchMock);
    localStorage.clear(); sessionStorage.clear();
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}><AuthProvider><MemoryRouter><LoginPage /></MemoryRouter></AuthProvider></QueryClientProvider>,
    );
    await userEvent.type(await screen.findByLabelText('Username'), 'owner');
    await userEvent.type(screen.getByLabelText('Password'), 'correct horse battery staple');
    await userEvent.type(screen.getByLabelText('6-digit authenticator code'), '123456');
    await userEvent.click(screen.getByRole('button', { name: 'Continue securely' }));
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
    const loginCall = fetchMock.mock.calls[1];
    if (!loginCall) throw new Error('the login request was not recorded');
    expect(loginCall[0]).toBe('/v1/auth/login');
    expect(loginCall[1]?.credentials).toBe('include');
    expect(requestBody(loginCall[1])).toEqual({ username: 'owner', password: 'correct horse battery staple', totp_code: '123456' });
    expect(localStorage).toHaveLength(0);
    expect(sessionStorage).toHaveLength(0);
  });

  it('carries the screen heading at every width and passes axe', async () => {
    // The headline used to be the `h1` and sits in a column hidden below 900
    // pixels, so the sign-in screen lost its only heading on a narrow viewport
    // and route-change focus had nothing to land on (WCX-05).
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(new Response('{}', { status: 401 }))));
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    const { container } = render(
      <QueryClientProvider client={queryClient}><AuthProvider><MemoryRouter><LoginPage /></MemoryRouter></AuthProvider></QueryClientProvider>,
    );

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
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(new Response('{}', { status: 401 }))));
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}><AuthProvider><MemoryRouter><LoginPage /></MemoryRouter></AuthProvider></QueryClientProvider>,
    );

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
    const fetchMock = vi.fn<(input: RequestInfo | URL) => Promise<Response>>(() =>
      Promise.resolve(new Response('{}', { status: 401 })),
    );
    vi.stubGlobal('fetch', fetchMock);
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}><AuthProvider><MemoryRouter><LoginPage /></MemoryRouter></AuthProvider></QueryClientProvider>,
    );

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
    const fetchMock = vi.fn<(input: RequestInfo | URL) => Promise<Response>>((input) =>
      Promise.resolve(new Response('{}', { status: requestURL(input) === '/v1/auth/session' ? 401 : 401 })),
    );
    vi.stubGlobal('fetch', fetchMock);
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}><AuthProvider><MemoryRouter><LoginPage /></MemoryRouter></AuthProvider></QueryClientProvider>,
    );
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
