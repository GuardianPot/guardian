import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { useState, type ReactNode } from 'react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { RequireAuth } from '@app/router';
import { EnvironmentsPage } from '@features/environments';
import { SignedIn, environment, json, loginHandlers, stubFetch, type StubHandler } from '@shared/testing/harness';
import { AuthProvider, useAuth } from './AuthContext';

afterEach(() => vi.unstubAllGlobals());

function renderConsole(options: { authenticated: boolean; handlers?: Record<string, StubHandler>; children?: ReactNode }) {
  const stub = stubFetch({
    ...loginHandlers(),
    'GET /v1/environments?limit=200': () => json({ environments: [environment()] }),
    ...options.handlers,
  });
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const tree = (
    <MemoryRouter initialEntries={['/environments']}>
      <Routes>
        <Route path="/login" element={<h1>Sign in</h1>} />
        <Route element={<RequireAuth />}>
          <Route path="/environments" element={options.children ?? <EnvironmentsPage />} />
        </Route>
      </Routes>
    </MemoryRouter>
  );
  render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>{options.authenticated ? <SignedIn>{tree}</SignedIn> : tree}</AuthProvider>
    </QueryClientProvider>,
  );
  return { stub, queryClient };
}

function LogoutProbe() {
  const auth = useAuth();
  const [message, setMessage] = useState('');
  return (
    <div>
      <button type="button" onClick={() => { void auth.logout().catch((error: Error) => setMessage(error.message)); }}>
        Attempt sign-out
      </button>
      {message && <p role="alert">{message}</p>}
    </div>
  );
}

describe('AuthContext', () => {
  it('returns the operator to sign-in and drops non-session query state on a mid-session 401', async () => {
    const { stub, queryClient } = renderConsole({
      authenticated: true,
      handlers: { 'POST /v1/environments': () => json({ error: 'unauthorized' }, 401) },
    });

    await userEvent.type(await screen.findByLabelText('Display name'), 'Second lab');
    await userEvent.click(screen.getByRole('button', { name: 'Create environment' }));

    expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeVisible();
    await waitFor(() => expect(queryClient.getQueryData(['environments'])).toBeUndefined());
    expect(queryClient.getQueryData(['session'])).toBeNull();
    expect(stub.called('POST /v1/environments')).toHaveLength(1);
  });

  it('refuses logout before any request when the memory-only CSRF proof is gone', async () => {
    const { stub } = renderConsole({ authenticated: false, children: <LogoutProbe /> });

    await userEvent.click(await screen.findByRole('button', { name: 'Attempt sign-out' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Re-authentication is required before logout.');
    expect(stub.called('POST /v1/auth/logout')).toEqual([]);
  });

  it('keeps a restored read-only session from attempting a mutation', async () => {
    const { stub } = renderConsole({ authenticated: false });

    expect(await screen.findByRole('heading', { name: 'Environments' })).toBeVisible();
    expect(screen.getByLabelText('Display name')).toBeDisabled();
    expect(screen.getByRole('button', { name: 'Create environment' })).toBeDisabled();
    expect(stub.called('POST /v1/environments')).toEqual([]);
  });

  it('sends no credential material to browser storage across a full sign-in', async () => {
    localStorage.clear();
    sessionStorage.clear();
    renderConsole({ authenticated: true });

    expect(await screen.findByRole('heading', { name: 'Environments' })).toBeVisible();
    expect(localStorage).toHaveLength(0);
    expect(sessionStorage).toHaveLength(0);
    expect(document.cookie).toBe('');
  });
});
