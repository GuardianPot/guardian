import { afterEach, describe, expect, it, vi } from 'vitest';
import { toConsoleError } from './error';
import { request, UNAUTHORIZED_EVENT } from './transport';

afterEach(() => vi.unstubAllGlobals());

type FetchMock = ReturnType<typeof vi.fn<(input: RequestInfo | URL, init?: RequestInit) => Promise<Response>>>;

function stub(response: Response): FetchMock {
  const mock: FetchMock = vi.fn(() => Promise.resolve(response));
  vi.stubGlobal('fetch', mock);
  return mock;
}

/** Reads the recorded request init, failing loudly when nothing was sent. */
function initOf(mock: FetchMock, index = 0): RequestInit {
  const call = mock.mock.calls[index];
  if (!call) throw new Error(`no request recorded at index ${index}`);
  return call[1] ?? {};
}

const headerOf = (mock: FetchMock, name: string, index = 0): string | null =>
  new Headers(initOf(mock, index).headers).get(name);

const ok = (body: unknown = {}) =>
  new Response(JSON.stringify(body), { status: 200, headers: { 'Content-Type': 'application/json' } });

describe('transport', () => {
  it('preserves the reviewed security behaviour on every request', async () => {
    const mock = stub(ok());
    await request('/v1/environments');
    const init = initOf(mock);
    expect(init.credentials).toBe('include');
    expect(init.cache).toBe('no-store');
    expect(new Headers(init.headers).get('Accept')).toBe('application/json');
  });

  it('sends the CSRF proof on mutations only', async () => {
    const read = stub(ok());
    await request('/v1/environments');
    expect(headerOf(read, 'X-CSRF-Token')).toBeNull();

    const write = stub(ok());
    await request('/v1/environments', { method: 'POST', body: {}, csrf: 'proof' });
    expect(headerOf(write, 'X-CSRF-Token')).toBe('proof');
  });

  it('never sets an Origin header', async () => {
    // `Origin` is a forbidden fetch header that browsers ignore. The Control
    // Plane compares the browser-supplied value; the console must not pretend
    // to control it.
    const mock = stub(ok());
    await request('/v1/environments', { method: 'POST', body: {}, csrf: 'proof' });
    expect(headerOf(mock, 'Origin')).toBeNull();
  });

  it('sends If-Match for a revisioned update', async () => {
    const mock = stub(ok());
    await request('/v1/environments/x', { method: 'PATCH', body: {}, csrf: 'proof', etag: 7 });
    expect(headerOf(mock, 'If-Match')).toBe('"7"');
  });

  it('raises the unauthorized event on an unexpected 401 only', async () => {
    const listener = vi.fn();
    window.addEventListener(UNAUTHORIZED_EVENT, listener);

    stub(new Response('{}', { status: 401 }));
    await expect(request('/v1/environments')).rejects.toThrow();
    expect(listener).toHaveBeenCalledTimes(1);

    stub(new Response('{}', { status: 401 }));
    await expect(request('/v1/auth/session', { allowUnauthorized: true })).rejects.toThrow();
    expect(listener).toHaveBeenCalledTimes(1);

    window.removeEventListener(UNAUTHORIZED_EVENT, listener);
  });

  it('classifies a rejected mutation proof as reauthentication-required', async () => {
    stub(new Response(JSON.stringify({ status: 'unauthorized' }), { status: 401 }));
    await expect(request('/v1/environments', { method: 'POST', body: {}, csrf: 'proof' }))
      .rejects.toMatchObject({ consoleError: { kind: 'reauthentication-required' } });
  });

  it('distinguishes a conflict from a validation failure', async () => {
    stub(new Response(JSON.stringify({ status: 'revision_conflict' }), { status: 412 }));
    await expect(request('/v1/environments/x', { method: 'PATCH', body: {}, csrf: 'p', etag: 1 }))
      .rejects.toMatchObject({ consoleError: { kind: 'conflict', statusSlug: 'revision_conflict' } });

    stub(new Response(JSON.stringify({ status: 'invalid_argument' }), { status: 400 }));
    await expect(request('/v1/environments', { method: 'POST', body: {}, csrf: 'p' }))
      .rejects.toMatchObject({ consoleError: { kind: 'validation' } });
  });

  it('classifies a fetch failure as network without dispatching expiry', async () => {
    vi.stubGlobal('fetch', vi.fn(() => Promise.reject(new TypeError('Failed to fetch'))));
    await expect(request('/v1/environments')).rejects.toMatchObject({
      consoleError: { kind: 'network', retryable: true },
    });
  });

  it('returns no body for 204 and 304', async () => {
    stub(new Response(null, { status: 204 }));
    await expect(request('/v1/auth/logout', { method: 'POST', csrf: 'p' })).resolves.toBeUndefined();
    stub(new Response(null, { status: 304 }));
    await expect(request('/v1/environments', { ifNoneMatch: '"1"' })).resolves.toBeUndefined();
  });

  it('survives an error response with no JSON body', async () => {
    stub(new Response('not json', { status: 500 }));
    await expect(request('/v1/environments')).rejects.toSatisfy(
      (error: unknown) => toConsoleError(error).kind === 'unavailable',
    );
  });
});
