import { queryOptions } from '@tanstack/react-query';
import { toConsoleError } from '@shared/api/error';
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
    staleTime: 30_000,
    retry: false,
  });

export function login(input: LoginInput): Promise<SessionCredentials> {
  return request<SessionCredentials>('/v1/auth/login', {
    method: 'POST',
    body: input,
    allowUnauthorized: true,
  });
}

export function logout(csrf: string): Promise<void> {
  return request<void>('/v1/auth/logout', { method: 'POST', csrf });
}
