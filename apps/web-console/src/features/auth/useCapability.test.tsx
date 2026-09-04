import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render, screen } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { EnvironmentsPage } from '@features/environments';
import { MemoryRouter } from 'react-router-dom';
import { json, loginHandlers, SignedIn, stubFetch } from '@shared/testing/harness';
import { AuthProvider } from './AuthContext';
import { useCapability } from './useCapability';
import type { Capability } from '@shared/auth/capability';

afterEach(() => vi.unstubAllGlobals());

const CAPABILITIES: Capability[] = [
  'environment.create',
  'environment.update',
  'zone.create',
  'zone.update',
  'zone.delete',
  'device.enroll',
  'device.disable',
  'device.revoke',
  'session.revoke',
  'account.password',
];

/** One hook call per component keeps the call order fixed, as hooks require. */
function Probe({ capability }: { capability: Capability }) {
  const decision = useCapability(capability);
  return <span>{`${capability}=${decision.allowed ? 'allowed' : decision.denial}`} </span>;
}

function Probes() {
  return (
    <p data-testid="decisions">
      {CAPABILITIES.map((capability) => <Probe key={capability} capability={capability} />)}
    </p>
  );
}

function renderProbe(authenticated: boolean) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        {authenticated ? <SignedIn><Probes /></SignedIn> : <Probes />}
      </AuthProvider>
    </QueryClientProvider>,
  );
}

describe('useCapability', () => {
  it('allows every capability once the operator holds a mutation proof', async () => {
    stubFetch(loginHandlers());
    renderProbe(true);
    const decisions = await screen.findByTestId('decisions');
    for (const capability of CAPABILITIES) {
      expect(decisions).toHaveTextContent(`${capability}=allowed`);
    }
  });

  it('denies every capability on a reload-restored read-only session', async () => {
    // The cookie restores the session but the memory-only CSRF proof is gone.
    stubFetch({ 'GET /v1/auth/session': loginHandlers()['GET /v1/auth/session']! });
    renderProbe(false);
    const decisions = await screen.findByTestId('decisions');
    for (const capability of CAPABILITIES) {
      expect(decisions).toHaveTextContent(`${capability}=reauthentication-required`);
    }
  });

  it('renders a denied control as present and disabled rather than hiding it', async () => {
    // A missing control reads as a broken product; a disabled one with its
    // reason tells the operator what to do. WC-D07 requires the latter.
    stubFetch({
      'GET /v1/auth/session': loginHandlers()['GET /v1/auth/session']!,
      'GET /v1/environments?limit=200': () => json({ environments: [] }),
    });
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    render(
      <QueryClientProvider client={queryClient}>
        <AuthProvider>
          <MemoryRouter><EnvironmentsPage /></MemoryRouter>
        </AuthProvider>
      </QueryClientProvider>,
    );
    const submit = await screen.findByRole('button', { name: 'Create environment' });
    expect(submit).toBeInTheDocument();
    expect(submit).toBeDisabled();
    expect(await screen.findByLabelText('Display name')).toBeDisabled();
  });
});
