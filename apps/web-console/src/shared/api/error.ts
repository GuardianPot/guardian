import { t, type PlainCatalogueKey } from '@shared/text';
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

/**
 * One field-level rejection (change proposal 0004).
 *
 * `field` is a request-body field path from the backend's closed vocabulary.
 * It is matched against a form's own control names and is never rendered, so
 * an unrecognised path cannot reach the DOM — it becomes a form-level error
 * instead, because a rejection reason must never be silently dropped.
 *
 * `messageKey` is resolved here from the backend's closed reason vocabulary.
 * The backend's own string is never rendered.
 */
export type ConsoleFieldError = {
  field: string;
  messageKey: PlainCatalogueKey;
};

export type ConsoleError = {
  kind: ConsoleErrorKind;
  /** Stable catalogue key for the operator-facing message. */
  messageKey: string;
  /** Backend status slug, diagnostics only. Never rendered. */
  statusSlug?: string;
  httpStatus?: number;
  retryable: boolean;
  /**
   * Backend error code, diagnostics only. Never rendered: an unrecognised code
   * falls back to the generic entry for its `status`.
   */
  code?: string;
  /** Field-level rejections, absent when the backend named no field. */
  fieldErrors?: readonly ConsoleFieldError[];
  /**
   * Opaque correlation identifier. Safe to display and the only field from
   * this contract permitted in the `WCX-15` diagnostic report.
   */
  requestId?: string;
  /** Back-off hint in whole seconds. A hint, never a promise. */
  retryAfter?: number;
};

const RETRYABLE: ReadonlySet<ConsoleErrorKind> = new Set([
  'rate-limited',
  'unavailable',
  'timeout',
  'network',
]);

/**
 * The operator-facing text for a classified failure.
 *
 * `WCX-02` reserved `errors.<kind>` as the catalogue key and kept a temporary
 * map here; `WCX-08` deleted that map, so this reads the catalogue directly.
 * The key and the kind are the same word by construction, which is why the
 * reservation worked.
 */
export function consoleErrorText(kind: ConsoleErrorKind): string {
  return t(`errors.${kind}`);
}

/** Raised by the transport so every failure carries a classified shape. */
export class ConsoleRequestError extends Error {
  readonly consoleError: ConsoleError;

  constructor(consoleError: ConsoleError) {
    super(consoleErrorText(consoleError.kind));
    this.name = 'ConsoleRequestError';
    this.consoleError = consoleError;
  }
}

/**
 * The backend's closed field-reason vocabulary, mapped to catalogue keys.
 *
 * A reason outside this table is not rendered verbatim; it falls back to the
 * generic entry, exactly as an unrecognised `code` falls back to its `status`.
 * That is what keeps all operator wording in `WCX-08` even when the backend
 * grows a reason this console has not been taught.
 */
const FIELD_REASON_KEYS: Readonly<Record<string, PlainCatalogueKey>> = {
  malformed: 'errors.field.malformed',
  out_of_range: 'errors.field.outOfRange',
  unsupported: 'errors.field.unsupported',
  unknown: 'errors.field.unknown',
  outside_zone: 'errors.field.outsideZone',
  conflicting: 'errors.field.conflicting',
};

export const GENERIC_FIELD_MESSAGE_KEY: PlainCatalogueKey = 'errors.field.rejected';

/** Resolves one backend reason to a catalogue key, never to backend text. */
export function fieldMessageKey(reason: unknown): PlainCatalogueKey {
  return typeof reason === 'string' && reason in FIELD_REASON_KEYS
    ? (FIELD_REASON_KEYS[reason] ?? GENERIC_FIELD_MESSAGE_KEY)
    : GENERIC_FIELD_MESSAGE_KEY;
}

type ErrorExtra = {
  statusSlug?: string;
  httpStatus?: number;
  code?: string;
  fieldErrors?: readonly ConsoleFieldError[];
  requestId?: string;
  retryAfter?: number;
};

const build = (kind: ConsoleErrorKind, extra: ErrorExtra = {}): ConsoleError => ({
  kind,
  messageKey: `errors.${kind}`,
  retryable: RETRYABLE.has(kind),
  ...(extra.statusSlug === undefined ? {} : { statusSlug: extra.statusSlug }),
  ...(extra.httpStatus === undefined ? {} : { httpStatus: extra.httpStatus }),
  ...(extra.code === undefined ? {} : { code: extra.code }),
  ...(extra.fieldErrors === undefined || extra.fieldErrors.length === 0
    ? {}
    : { fieldErrors: extra.fieldErrors }),
  ...(extra.requestId === undefined ? {} : { requestId: extra.requestId }),
  ...(extra.retryAfter === undefined ? {} : { retryAfter: extra.retryAfter }),
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

/** Bounds what one response may attach to a form, so a hostile or broken
 * backend cannot flood a screen with controls' worth of messages. The contract
 * caps `field_errors` at 16; this is the console refusing to trust that. */
const MAX_FIELD_ERRORS = 16;

/**
 * Reads the optional change-proposal-0004 fields from an error body.
 *
 * Every field is optional by contract, so an unmigrated endpoint returns
 * `{"status"}` and this yields nothing. Each value is shape-checked rather than
 * trusted: these arrive from the network, and the taxonomy has to keep working
 * against a backend that is wrong as well as one that is merely older.
 */
export function errorContract(body: unknown): ErrorExtra {
  if (typeof body !== 'object' || body === null) return {};
  const record = body as Record<string, unknown>;
  const fieldErrors: ConsoleFieldError[] = [];
  if (Array.isArray(record['field_errors'])) {
    for (const entry of record['field_errors'].slice(0, MAX_FIELD_ERRORS)) {
      if (typeof entry !== 'object' || entry === null) continue;
      const field = (entry as Record<string, unknown>)['field'];
      if (typeof field !== 'string' || field === '') continue;
      fieldErrors.push({
        field,
        messageKey: fieldMessageKey((entry as Record<string, unknown>)['code']),
      });
    }
  }
  const code = record['code'];
  const requestId = record['request_id'];
  const retryAfter = record['retry_after'];
  return {
    ...(typeof code === 'string' && code !== '' ? { code } : {}),
    ...(fieldErrors.length === 0 ? {} : { fieldErrors }),
    // A correlation identifier is 32 lowercase hex characters by contract. A
    // value of any other shape is not one, and is dropped rather than shown to
    // an operator who would then quote it to support.
    ...(typeof requestId === 'string' && /^[0-9a-f]{32}$/.test(requestId) ? { requestId } : {}),
    ...(typeof retryAfter === 'number' && Number.isInteger(retryAfter) && retryAfter > 0
      ? { retryAfter }
      : {}),
  };
}

/** Classifies any thrown value into a `ConsoleError`. */
export function toConsoleError(input: unknown): ConsoleError {
  if (input instanceof ConsoleRequestError) return input.consoleError;
  if (input instanceof DOMException && input.name === 'AbortError') return build('timeout');
  if (input instanceof TypeError) return build('network');
  return build('unexpected');
}

export const consoleError = build;
