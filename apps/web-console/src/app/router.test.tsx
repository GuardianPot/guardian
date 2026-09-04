import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { RequireAuth } from '@app/router';
import { AuthProvider } from '@features/auth';

afterEach(() => vi.unstubAllGlobals());

describe('authenticated routes', () => {
  it('redirects a denied session to sign-in without rendering protected content', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.resolve(new Response('{}', { status: 401 }))));
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <MemoryRouter initialEntries={['/environments']}>
            <Routes>
              <Route path="/login" element={<h1>Sign in route</h1>} />
              <Route element={<RequireAuth />}><Route path="/environments" element={<h1>Protected environments</h1>} /></Route>
            </Routes>
          </MemoryRouter>
        </AuthProvider>
      </QueryClientProvider>,
    );
    expect(await screen.findByRole('heading', { name: 'Sign in route' })).toBeVisible();
    expect(screen.queryByText('Protected environments')).not.toBeInTheDocument();
  });
});
