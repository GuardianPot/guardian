import { queryOptions } from '@tanstack/react-query';
import { toConsoleError } from '@shared/api/error';
import { freshness } from '@shared/api/freshness';
import { request } from '@shared/api/transport';
import type { Session, SessionCredentials } from '@shared/api/types';

export const authKeys = {
  all: ['auth'] as const,
  session: () => [...authKeys.all, 'session'] as const,
};

export type LoginInput = {
  username: string;
  password: string;
  totp_code?: string;
  recovery_code?: string;
};

/** Probes the session. A 401 is the expected signed-out answer, not an error. */
async function readSession(signal?: AbortSignal): Promise<Session | null> {
  try {
    const response = await request<{ session: Session }>('/v1/auth/session', {
      allowUnauthorized: true,
      ...(signal === undefined ? {} : { signal }),
    });
    return response.session;
  } catch (error) {
    if (toConsoleError(error).httpStatus === 401) return null;
    throw error;
  }
}

export const sessionQuery = () =>
  queryOptions({
    queryKey: authKeys.session(),
    queryFn: ({ signal }) => readSession(signal),
    ...freshness('session'),
  });

export function login(input: LoginInput): Promise<SessionCredentials> {
  return request<SessionCredentials>('/v1/auth/login', {
    method: 'POST',
    body: input,
    allowUnauthorized: true,
  });
}

/**
 * Exchanges a valid session cookie for a new synchronizer proof.
 *
 * Change proposal 0003. A reload leaves the cookie intact and the proof gone,
 * because `W11-C3-A` keeps the proof in memory only. This is how the console
 * gets one back without a full sign-in — and it is the only request in the
 * console that deliberately carries no CSRF token, because not having one is
 * the situation it exists to resolve.
 *
 * The Control Plane requires the session cookie and an exact origin match, and
 * the cookie is `SameSite=Strict`, so a cross-site page cannot reach this with
 * an operator session at all.
 */
export async function reissueCsrf(): Promise<string> {
  return (await request<{ csrf_token: string }>('/v1/auth/csrf', { method: 'POST' })).csrf_token;
}

export function logout(csrf: string): Promise<void> {
  return request<void>('/v1/auth/logout', { method: 'POST', csrf });
}
