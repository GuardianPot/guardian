import type { StatusResponse } from './types';

/**
 * Normalized console error taxonomy (WC-D03 option A).
 *
 * Components consume `ConsoleError` and never inspect an HTTP status. The
 * operator-facing message is a catalogue key drawn from a fixed table; a
 * backend-supplied string is never rendered, so a hostile or unexpected
 * `status` slug cannot reach the DOM. `statusSlug` and `httpStatus` are kept
 * for diagnostics only.
 */
export type ConsoleErrorKind =
  | 'unauthenticated'
  | 'reauthentication-required'
  | 'forbidden'
  | 'not-found'
  | 'validation'
  | 'conflict'
  | 'rate-limited'
  | 'unavailable'
  | 'timeout'
  | 'network'
  | 'unexpected';

export type ConsoleError = {
  kind: ConsoleErrorKind;
  /** Stable catalogue key for the operator-facing message. */
  messageKey: string;
  /** Backend status slug, diagnostics only. Never rendered. */
  statusSlug?: string;
  httpStatus?: number;
  retryable: boolean;
};

const RETRYABLE: ReadonlySet<ConsoleErrorKind> = new Set([
  'rate-limited',
  'unavailable',
  'timeout',
  'network',
]);

/**
 * Temporary English text for each kind. `WCX-08` moves these into the
 * operator catalogue keyed by `messageKey` and deletes this map.
 */
export const CONSOLE_ERROR_TEXT: Readonly<Record<ConsoleErrorKind, string>> = {
  'unauthenticated': 'Your session is no longer valid. Sign in again.',
  'reauthentication-required': 'Re-authenticate before changing configuration.',
  'forbidden': 'This action was refused.',
  'not-found': 'This record is unavailable or outside this environment.',
  'validation': 'Guardian rejected the submitted values.',
  'conflict': 'Another change was recorded first. Reload the current value.',
  'rate-limited': 'Too many attempts. Wait before trying again.',
  'unavailable': 'Guardian could not complete the request.',
  'timeout': 'The request took too long to complete.',
  'network': 'Guardian could not be reached.',
  'unexpected': 'Guardian could not complete the request.',
};

/** Raised by the transport so every failure carries a classified shape. */
export class ConsoleRequestError extends Error {
  readonly consoleError: ConsoleError;

  constructor(consoleError: ConsoleError) {
    super(CONSOLE_ERROR_TEXT[consoleError.kind]);
    this.name = 'ConsoleRequestError';
    this.consoleError = consoleError;
  }
}

const build = (
  kind: ConsoleErrorKind,
  extra: { statusSlug?: string; httpStatus?: number } = {},
): ConsoleError => ({
  kind,
  messageKey: `errors.${kind}`,
  retryable: RETRYABLE.has(kind),
  ...(extra.statusSlug === undefined ? {} : { statusSlug: extra.statusSlug }),
  ...(extra.httpStatus === undefined ? {} : { httpStatus: extra.httpStatus }),
});

/** Maps an HTTP status to a kind. `sessionActive` separates the 401 cases. */
export function kindForStatus(httpStatus: number, sessionActive: boolean): ConsoleErrorKind {
  if (httpStatus === 401) return sessionActive ? 'reauthentication-required' : 'unauthenticated';
  if (httpStatus === 403) return 'forbidden';
  if (httpStatus === 404) return 'not-found';
  if (httpStatus === 400 || httpStatus === 422) return 'validation';
  if (httpStatus === 409 || httpStatus === 412) return 'conflict';
  if (httpStatus === 429) return 'rate-limited';
  if (httpStatus >= 500 && httpStatus <= 504) return 'unavailable';
  return 'unexpected';
}

/** Reads the backend status slug for diagnostics; never for rendering. */
export function statusSlug(body: unknown): string | undefined {
  if (typeof body !== 'object' || body === null) return undefined;
  const candidate = (body as Partial<StatusResponse>).status;
  return typeof candidate === 'string' ? candidate : undefined;
}

/** Classifies any thrown value into a `ConsoleError`. */
export function toConsoleError(input: unknown): ConsoleError {
  if (input instanceof ConsoleRequestError) return input.consoleError;
  if (input instanceof DOMException && input.name === 'AbortError') return build('timeout');
  if (input instanceof TypeError) return build('network');
  return build('unexpected');
}

export const consoleError = build;
