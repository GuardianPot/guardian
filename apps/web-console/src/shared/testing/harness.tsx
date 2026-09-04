import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { render } from '@testing-library/react';
import { useEffect, useRef, useState, type ReactNode } from 'react';
import { MemoryRouter, Route, Routes } from 'react-router-dom';
import { vi } from 'vitest';
import { AuthProvider, useAuth } from '@features/auth';
import type { Device, Environment, HealthCondition, HealthConditionType, HealthView, Session, Zone } from '@shared/api/types';

export const environmentID = '018f1f7e-6d31-7cc5-8db8-17547f78e6c1';
export const deviceID = '018f1f7e-6d31-7cc5-8db8-17547f78e6c2';
export const csrfToken = 'cccccccccccccccccccccccccccccccccccccccccc1';

export type StubHandler = (init?: RequestInit) => Response;

/** Resolves the URL of a fetch input without stringifying an object. */
export function requestURL(input: RequestInfo | URL): string {
  if (typeof input === 'string') return input;
  return input instanceof URL ? input.href : input.url;
}

/** Reads a recorded JSON request body. */
export function requestBody(init: RequestInit | undefined): unknown {
  return typeof init?.body === 'string' ? JSON.parse(init.body) : undefined;
}

/**
 * Routes stubbed responses by `METHOD /path`. An unmapped call answers 404 so a
 * missing stub can never be mistaken for an authorization or health outcome.
 */
export function stubFetch(handlers: Record<string, StubHandler>) {
  const calls: { key: string; init: RequestInit | undefined }[] = [];
  const mock = vi.fn((input: RequestInfo | URL, init?: RequestInit) => {
    const key = `${init?.method ?? 'GET'} ${requestURL(input)}`;
    calls.push({ key, init });
    return Promise.resolve(handlers[key]?.(init) ?? json({ error: 'unstubbed' }, 404));
  });
  vi.stubGlobal('fetch', mock);
  return {
    calls,
    called: (key: string) => calls.filter((call) => call.key === key),
    header: (key: string, name: string) => {
      const call = calls.find((entry) => entry.key === key);
      return call ? new Headers(call.init?.headers).get(name) : null;
    },
  };
}

export function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'Content-Type': 'application/json' } });
}

export function session(): Session {
  return {
    session_id: '018f1f7e-6d31-7cc5-8db8-17547f78e6c3',
    user_id: '018f1f7e-6d31-7cc5-8db8-17547f78e6c4',
    username: 'owner',
    role: 'owner',
    created_at: '2026-08-29T12:00:00Z',
    last_seen_at: '2026-08-29T12:00:00Z',
    expires_at: '2026-08-29T13:00:00Z',
    current: true,
  };
}

export function environment(overrides: Partial<Environment> = {}): Environment {
  return {
    environment_id: environmentID,
    organization_id: '018f1f7e-6d31-7cc5-8db8-17547f78e6c0',
    display_name: 'Lab',
    revision: 3,
    zone_count: 0,
    status: 'needs_zones',
    created_at: '2026-08-29T12:00:00Z',
    updated_at: '2026-08-29T12:00:00Z',
    ...overrides,
  };
}

export function device(overrides: Partial<Device> = {}): Device {
  return {
    device_id: deviceID,
    environment_id: environmentID,
    display_name: 'edge-one',
    state: 'active',
    created_at: '2026-08-29T12:00:00Z',
    updated_at: '2026-08-29T12:05:00Z',
    ...overrides,
  };
}

export function zone(overrides: Partial<Zone> = {}): Zone {
  return {
    zone_id: '018f1f7e-6d31-7cc5-8db8-17547f78e6c5',
    environment_id: environmentID,
    display_name: 'Lab zone',
    cidr: '10.20.0.0/24',
    revision: 1,
    created_at: '2026-08-29T12:00:00Z',
    updated_at: '2026-08-29T12:00:00Z',
    ...overrides,
  };
}

const conditionTypes: HealthConditionType[] = [
  'edge_connected',
  'device_certificate_ready',
  'config_converged',
  'local_database_healthy',
  'spool_healthy',
  'clock_quality',
  'container_runtime_reachable',
  'privileged_helper_reachable',
];

/** Builds the full eight-condition backend projection with an optional override. */
export function healthView(override: Partial<HealthCondition> & { type?: string } = {}): HealthView {
  const conditions: HealthCondition[] = conditionTypes.map((type) => ({
    type,
    status: 'True',
    reason: 'observed',
    message: '',
    source_device_id: deviceID,
    last_transition_time: '2026-08-29T12:00:00Z',
    ...(override.type === type ? override : {}),
  }));
  const blocking = conditions.find((condition) => condition.status !== 'True');
  return {
    aggregate: blocking
      ? { status: blocking.status, blocking_type: blocking.type, reason: blocking.reason, blocking_device_id: deviceID }
      : { status: 'True' },
    conditions,
    received_at: '2026-08-29T12:06:00Z',
  };
}

/**
 * Establishes the memory-only CSRF proof exactly as the login route does, so a
 * mutation test exercises the real capability gate instead of bypassing it.
 */
export function SignedIn({ children }: { children: ReactNode }) {
  const auth = useAuth();
  const requested = useRef(false);
  const [established, setEstablished] = useState(false);
  useEffect(() => {
    if (requested.current) return;
    requested.current = true;
    void auth.login({ username: 'owner', password: 'correct horse battery staple', totp_code: '123456' })
      .then(() => setEstablished(true));
  }, [auth]);
  // Gate on the completed sign-in, not on the capability, so a later expiry keeps
  // the route mounted and observable instead of unmounting it.
  if (!established) return <p>establishing test session</p>;
  return <>{children}</>;
}

export type RenderRouteOptions = {
  /** When false the console keeps the reload-restored, read-only session state. */
  authenticated?: boolean;
  path: string;
  entry: string;
};

export function renderRoute(element: ReactNode, options: RenderRouteOptions) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const content = options.authenticated === false ? element : <SignedIn>{element}</SignedIn>;
  const view = render(
    <QueryClientProvider client={queryClient}>
      <AuthProvider>
        <MemoryRouter initialEntries={[options.entry]}>
          <Routes>
            <Route path="/login" element={<h1>Sign in</h1>} />
            <Route path={options.path} element={content} />
          </Routes>
        </MemoryRouter>
      </AuthProvider>
    </QueryClientProvider>,
  );
  return { ...view, queryClient };
}

export function loginHandlers(): Record<string, StubHandler> {
  return {
    'GET /v1/auth/session': () => json({ session: session() }),
    'POST /v1/auth/login': () => json({ csrf_token: csrfToken, session: session() }),
  };
}
