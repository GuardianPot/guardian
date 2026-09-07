import { http, type HttpHandler, type RequestHandler } from 'msw';
import { setupServer } from 'msw/node';
import { afterEach } from 'vitest';

/**
 * Network mocking through MSW (WCX-06 section 9.3, decision WC-D26).
 *
 * This replaces a hand-written `fetch` stub. The stub worked, but it only ever
 * saw what the console asked for; MSW intercepts at the request layer, so what
 * a test observes is a real `Request` — real headers, real body, real
 * credentials mode. An assertion about `X-CSRF-Token` now checks the header
 * that would actually travel.
 *
 * `onUnhandledRequest: 'error'` is mandatory and carries forward the guarantee
 * the stub gave by answering 404: a call nobody mocked fails loudly instead of
 * being mistaken for an authorised or healthy outcome. With MSW it is louder
 * still — the request throws rather than returning a plausible response.
 *
 * The key format is unchanged from the stub — `METHOD /path` — so the
 * migration moved the mechanism and left every assertion where it was.
 */
export type MockResponder = (request: Request) => Response | Promise<Response>;

export type RecordedCall = {
  key: string;
  request: Request;
  headers: Headers;
  /** Credentials mode the console asked for. Cookies ride on this. */
  credentials: RequestCredentials;
  body: unknown;
};

export type MockApi = {
  calls: RecordedCall[];
  called: (key: string) => RecordedCall[];
  header: (key: string, name: string) => string | null;
  /** Adds or replaces a handler mid-test, for a second response to one path. */
  override: (key: string, responder: MockResponder) => void;
};

const METHODS: Readonly<Record<string, keyof typeof http>> = {
  GET: 'get',
  POST: 'post',
  PATCH: 'patch',
  PUT: 'put',
  DELETE: 'delete',
};

let active: ReturnType<typeof setupServer> | null = null;

afterEach(() => {
  active?.close();
  active = null;
});

/** Builds one MSW handler from a `METHOD /path` key. */
function toHandler(key: string, responder: MockResponder, record: (call: RecordedCall) => void): HttpHandler {
  const [method = 'GET', target = '/'] = key.split(' ');
  const verb = METHODS[method];
  if (verb === undefined) throw new Error(`unsupported method in mock key: ${key}`);
  // The query string stays in the key for assertions but is dropped from the
  // matcher: MSW matches on path, and every console read has a unique path.
  const path = target.split('?')[0] ?? '/';

  return http[verb](path, async ({ request }) => {
    const clone = request.clone();
    let body: unknown;
    try {
      body = clone.body === null ? undefined : JSON.parse(await clone.text());
    } catch {
      body = undefined;
    }
    record({
      key,
      request,
      headers: new Headers(request.headers),
      credentials: request.credentials,
      body,
    });
    return responder(request);
  });
}

/**
 * Starts a server for one test. Closed automatically after it.
 */
export function mockApi(handlers: Record<string, MockResponder>): MockApi {
  const calls: RecordedCall[] = [];
  const record = (call: RecordedCall): void => { calls.push(call); };
  const built: RequestHandler[] = Object.entries(handlers).map(([key, responder]) =>
    toHandler(key, responder, record),
  );

  // The previous server is closed *before* the next one listens. MSW patches
  // the global request path, so two live servers fight over it and closing the
  // older one afterwards tears the interception down under the newer one.
  active?.close();
  const server = setupServer(...built);
  // Section 9.3.3. An unmocked call is a test defect, never a silent pass.
  server.listen({ onUnhandledRequest: 'error' });
  active = server;

  return {
    calls,
    called: (key) => calls.filter((call) => call.key === key),
    header: (key, name) => calls.find((call) => call.key === key)?.headers.get(name) ?? null,
    override: (key, responder) => {
      server.use(toHandler(key, responder, record));
    },
  };
}
