import { consoleError, ConsoleRequestError, kindForStatus, statusSlug } from './error';

/**
 * The single HTTP transport. Its security behaviour is fixed:
 *
 * - the session cookie travels via `credentials: 'include'`;
 * - responses are never cached (`no-store`);
 * - the memory-only CSRF proof is sent on mutations only;
 * - a revisioned update sends `If-Match`;
 * - an unexpected 401 raises `guardian:unauthorized` so the session view
 *   expires through one path.
 *
 * The browser supplies the `Origin` header on non-GET requests and the
 * Control Plane compares it to its configured public origin. Origin validation
 * is a Control Plane responsibility; the console must not set the header,
 * because `Origin` is a forbidden fetch header that browsers ignore, and
 * setting it would suggest a control the console does not have.
 */
export type RequestOptions = {
  method?: 'GET' | 'POST' | 'PATCH' | 'DELETE';
  body?: unknown;
  csrf?: string;
  /** Strong revision for `If-Match` on a revisioned update. */
  etag?: number;
  /** Suppresses the unauthorized event for probes that expect a 401. */
  allowUnauthorized?: boolean;
  signal?: AbortSignal;
  /** Prepared for conditional revalidation; nothing supplies one yet. */
  ifNoneMatch?: string;
};

export const UNAUTHORIZED_EVENT = 'guardian:unauthorized';

export async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const headers = new Headers({ Accept: 'application/json' });
  if (options.body !== undefined) headers.set('Content-Type', 'application/json');
  if (options.csrf) headers.set('X-CSRF-Token', options.csrf);
  if (options.etag !== undefined) headers.set('If-Match', `"${options.etag}"`);
  if (options.ifNoneMatch !== undefined) headers.set('If-None-Match', options.ifNoneMatch);

  let response: Response;
  try {
    response = await fetch(path, {
      method: options.method ?? 'GET',
      // Spread rather than assign `undefined`: under exactOptionalPropertyTypes
      // an explicit `undefined` body is not the same as an omitted one.
      ...(options.body === undefined ? {} : { body: JSON.stringify(options.body) }),
      ...(options.signal === undefined ? {} : { signal: options.signal }),
      headers,
      credentials: 'include',
      cache: 'no-store',
    });
  } catch (caught) {
    if (caught instanceof DOMException && caught.name === 'AbortError') {
      throw new ConsoleRequestError(consoleError('timeout'));
    }
    throw new ConsoleRequestError(consoleError('network'));
  }

  // A conditional read that was unchanged is a success, but `ok` is false for
  // 304, so it must be handled before the error branch.
  if (response.status === 304) return undefined as T;

  if (!response.ok) {
    // An expected 401 (the session probe) must not expire the session view.
    const sessionActive = Boolean(options.csrf);
    if (response.status === 401 && !options.allowUnauthorized) {
      window.dispatchEvent(new Event(UNAUTHORIZED_EVENT));
    }
    let slug: string | undefined;
    try {
      slug = statusSlug(await response.clone().json());
    } catch {
      slug = undefined;
    }
    throw new ConsoleRequestError(
      consoleError(kindForStatus(response.status, sessionActive), {
        httpStatus: response.status,
        ...(slug === undefined ? {} : { statusSlug: slug }),
      }),
    );
  }

  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}
