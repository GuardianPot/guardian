import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { AuthProvider } from './AuthContext';
import { requestBody, requestURL } from '@shared/testing/harness';
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
