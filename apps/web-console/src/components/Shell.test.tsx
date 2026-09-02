import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { RequireAuth } from '../App';
import { AuthProvider } from '../auth/AuthContext';
import { SignedIn, csrfToken, json, loginHandlers, stubFetch, type StubHandler } from '../test/harness';
import { Shell } from './Shell';

afterEach(() => vi.unstubAllGlobals());

function renderShell(options: { authenticated: boolean; handlers?: Record<string, StubHandler> }) {
  const stub = stubFetch({ ...loginHandlers(), ...options.handlers });
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const tree = (
    <MemoryRouter initialEntries={['/environments']}>
      <Routes>
        <Route path="/login" element={<h1>Sign in</h1>} />
        <Route element={<RequireAuth />}>
          <Route element={<Shell />}>
            <Route path="/environments" element={<h2>Environment workspace</h2>} />
          </Route>
        </Route>
      </Routes>
    </MemoryRouter>
  );
  render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>{options.authenticated ? <SignedIn>{tree}</SignedIn> : tree}</AuthProvider>
    </QueryClientProvider>,
  );
  return stub;
}

describe('Shell', () => {
  it('offers reauthentication instead of sign-out while the session is read-only', async () => {
    const stub = renderShell({ authenticated: false });

    expect(await screen.findByText(/Read-only session restored\./)).toBeVisible();
    expect(screen.getAllByRole('link', { name: 'Re-authenticate' })).toHaveLength(2);
    expect(screen.queryByRole('button', { name: 'Sign out' })).not.toBeInTheDocument();
    expect(stub.called('POST /v1/auth/logout')).toEqual([]);
  });

  it('signs out with the memory-only CSRF proof and returns the operator to sign-in', async () => {
    const stub = renderShell({
      authenticated: true,
      handlers: { 'POST /v1/auth/logout': () => new Response(null, { status: 204 }) },
    });

    await userEvent.click(await screen.findByRole('button', { name: 'Sign out' }));

    expect(await screen.findByRole('heading', { name: 'Sign in' })).toBeVisible();
    expect(stub.called('POST /v1/auth/logout')).toHaveLength(1);
    expect(stub.header('POST /v1/auth/logout', 'X-CSRF-Token')).toBe(csrfToken);
    expect(stub.calls.find((call) => call.key === 'POST /v1/auth/logout')?.init?.credentials).toBe('include');
    expect(screen.queryByText('Environment workspace')).not.toBeInTheDocument();
  });

  it('keeps the operator signed in when the backend refuses the sign-out', async () => {
    const stub = renderShell({
      authenticated: true,
      handlers: { 'POST /v1/auth/logout': () => json({ error: 'forbidden' }, 403) },
    });

    await userEvent.click(await screen.findByRole('button', { name: 'Sign out' }));

    expect(await screen.findByRole('alert')).toHaveTextContent('Sign-out failed. This session is still active.');
    expect(screen.getByText('Environment workspace')).toBeVisible();
    expect(stub.called('POST /v1/auth/logout')).toHaveLength(1);
    expect(screen.queryByRole('heading', { name: 'Sign in' })).not.toBeInTheDocument();
  });
});
