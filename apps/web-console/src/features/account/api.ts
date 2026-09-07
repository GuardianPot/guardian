import { queryOptions, type QueryClient } from '@tanstack/react-query';
import { freshness } from '@shared/api/freshness';
import { request } from '@shared/api/transport';
import type { Session, SessionCredentials } from '@shared/api/types';

/**
 * Owner account operations (WCX-09 sections 9.4 and 9.7).
 *
 * Every one of these already exists in the contract and is already audited
 * server-side under `AUTH-06`. Nothing here is new capability; it is the
 * console finally exposing what the Control Plane has always enforced.
 */
export const accountKeys = {
  all: ['account'] as const,
  sessions: () => [...accountKeys.all, 'sessions'] as const,
};

/**
 * The owner's sessions, including revoked history.
 *
 * `operational` freshness: an operator revoking a session from another device
 * needs to see it disappear without hunting for a refresh control.
 *
 * A session is not free text — every field is a UUID, a timestamp, an enum, or
 * the operator's own username, all of which the Control Plane validates — so
 * nothing here crosses the untrusted boundary. Marking them would make the
 * brand mean "string" rather than "untrusted" (`WCX-06` section 9.8).
 */
export const sessionsQuery = () =>
  queryOptions({
    queryKey: accountKeys.sessions(),
    queryFn: async ({ signal }) =>
      (await request<{ sessions: Session[] }>('/v1/auth/sessions', { signal })).sessions,
    ...freshness('operational'),
  });

export function revokeSession(sessionID: string, csrf: string): Promise<void> {
  return request<void>(`/v1/auth/sessions/${sessionID}`, { method: 'DELETE', csrf });
}

/**
 * Changes the owner password.
 *
 * The Control Plane revokes every session for the user and issues a new one,
 * so the response carries fresh credentials that the caller must install — the
 * proof held in memory a moment ago is no longer valid.
 *
 * What the response does *not* carry is a list of what it revoked. Section 8.7
 * forbids claiming an outcome the response did not confirm, so the caller
 * refetches the session list and lets it say what happened rather than
 * narrating a count nobody sent.
 */
export function changePassword(
  input: { current_password: string; new_password: string },
  csrf: string,
): Promise<SessionCredentials> {
  return request<SessionCredentials>('/v1/auth/password', { method: 'POST', body: input, csrf });
}

export const accountInvalidation = {
  afterSessionChange: (client: QueryClient) =>
    client.invalidateQueries({ queryKey: accountKeys.sessions() }),
};
