import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { cleanup, render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { RequireAuth } from '@app/router';
import { AuthProvider } from '@features/auth';
import { SignedIn, csrfToken, json, loginHandlers, mockApi, type MockResponder } from '@shared/testing/harness';
import { expectNoAxeViolations } from '@shared/testing/axe';
import { Shell } from './Shell';

afterEach(() => vi.unstubAllGlobals());

function Explode(): never {
  throw new Error('screen-token-4b1e leaked through an exception message');
}

function renderShell(options: {
  authenticated: boolean;
  handlers?: Record<string, MockResponder>;
  entry?: string;
}) {
  const stub = mockApi({ ...loginHandlers(), ...options.handlers });
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const tree = (
    <MemoryRouter initialEntries={[options.entry ?? '/environments']}>
      <Routes>
        <Route path="/login" element={<h1>Sign in</h1>} />
        <Route element={<RequireAuth />}>
          <Route element={<Shell />}>
            <Route path="/environments" element={<h2>Environment workspace</h2>} />
            <Route path="/broken" element={<Explode />} />
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
    // The session cookie rides on the credentials mode, and MSW reports the
    // mode the console actually asked for rather than what a stub was handed.
    expect(stub.calls.find((call) => call.key === 'POST /v1/auth/logout')?.credentials).toBe('include');
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

  it('keeps navigation and sign-out reachable when a screen throws', async () => {
    // `P1-W11` GAP-2: without a boundary this blanked the console, leaving an
    // operator with no way to end the session.
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    renderShell({ authenticated: true, entry: '/broken' });

    expect(await screen.findByRole('button', { name: 'Sign out' })).toBeVisible();
    expect(screen.getByRole('navigation', { name: 'Primary navigation' })).toBeVisible();
    expect(screen.getByRole('alert')).toHaveTextContent('This screen stopped unexpectedly');
    expect(document.body.innerHTML).not.toContain('screen-token-4b1e');
    consoleError.mockRestore();
  });

  it('recovers the failed screen when the operator navigates away', async () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    renderShell({ authenticated: true, entry: '/broken' });
    expect(await screen.findByRole('alert')).toBeVisible();

    await userEvent.click(screen.getByRole('link', { name: 'Environments' }));

    expect(await screen.findByText('Environment workspace')).toBeVisible();
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    consoleError.mockRestore();
  });

  it('reports no serious or critical axe violation, signed in or read-only', async () => {
    renderShell({ authenticated: true });
    await screen.findByRole('button', { name: 'Sign out' });
    await expectNoAxeViolations(document.body);
    // A second shell alongside the first would duplicate every landmark and
    // the `main-content` id the skip link targets.
    cleanup();

    renderShell({ authenticated: false });
    await screen.findByText(/Read-only session restored\./);
    await expectNoAxeViolations(document.body);
  });

  it('keeps the read-only session banner the browser suite reloads into', async () => {
    renderShell({ authenticated: false });

    const banner = await screen.findByText(/Read-only session restored\./);
    expect(banner).toBeVisible();
    expect(banner).toHaveAttribute('role', 'status');
    expect(banner).toHaveTextContent('before changing configuration or signing out.');
  });
});
