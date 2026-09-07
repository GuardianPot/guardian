import type { ConsoleError, ConsoleErrorKind } from '@shared/api/error';
import { isBeyondFreshness } from './freshness';

/**
 * The one place a query result becomes a data state (WCX-04 section 9.1).
 *
 * Pure and exhaustive over `ConsoleErrorKind`: adding a kind to the taxonomy
 * fails typecheck here rather than silently falling through to `error`. A
 * screen never classifies its own failure, so `denied`, `unknown`, and
 * `degraded` cannot drift apart between routes the way they did when `P1-W11`
 * wrote the rules inline.
 */
export type DataState =
  | 'loading'
  | 'empty'
  | 'unknown'
  | 'stale'
  | 'partial'
  | 'degraded'
  | 'denied'
  | 'error';

/** `ready` is the ninth outcome: there is data, and the screen renders it. */
export type DataOutcome = DataState | 'ready';

export const DATA_STATES: readonly DataState[] = [
  'loading',
  'empty',
  'unknown',
  'stale',
  'partial',
  'degraded',
  'denied',
  'error',
];

export type DataStateInput = {
  /** First load in flight. */
  isPending: boolean;
  /** Cached data is available to show, from this read or a previous one. */
  hasData: boolean;
  /** Classified failure, or `null` on success. */
  error: ConsoleError | null;
  /** Success, and the collection the backend returned holds zero items. */
  isEmpty?: boolean;
  /** Success, and the domain defines this projection's absence as unknown. */
  isAbsent?: boolean;
  /** A `not-found` on this resource means "no observation", not "gone". */
  observationShaped?: boolean;
  /** Sources that failed while others succeeded. */
  partialFailures?: readonly string[];
  /** When the shown data was observed. Drives the freshness rule. */
  observedAt?: string | null;
  now?: number;
};

/**
 * All three authorization refusals map to `denied`.
 *
 * The section 9.1 table names `forbidden` and `unauthenticated`.
 * `reauthentication-required` is the third refusal in the taxonomy, and letting
 * it fall through to the catch-all `error` would render a refusal as an
 * unexplained failure — the confusion section 8.3 exists to prevent. A read
 * carries no CSRF proof, so the transport classifies a read's 401 as
 * `unauthenticated` and this kind is currently unreachable from a read.
 *
 * A `not-found` that is not observation-shaped means the record is gone, so it
 * stays in the catch-all: showing cached data for it would present a deleted
 * record as current.
 */
function stateForError(kind: ConsoleErrorKind, hasData: boolean): DataState {
  switch (kind) {
    case 'forbidden':
    case 'unauthenticated':
    case 'reauthentication-required':
      return 'denied';
    case 'unavailable':
    case 'timeout':
      return hasData ? 'stale' : 'degraded';
    case 'network':
      return hasData ? 'stale' : 'error';
    case 'not-found':
    case 'validation':
    case 'conflict':
    case 'rate-limited':
    case 'unexpected':
      return 'error';
  }
}

export function resolveDataState(input: DataStateInput): DataOutcome {
  if (input.isPending && !input.hasData) return 'loading';

  if (input.error) {
    // `not-found` on an observation-shaped resource is the absence of an
    // observation, which is `unknown` — never a failure and never healthy.
    if (input.error.kind === 'not-found' && input.observationShaped) return 'unknown';
    return stateForError(input.error.kind, input.hasData);
  }

  if (input.partialFailures && input.partialFailures.length > 0) return 'partial';

  // Freshness is checked before the success-shaped states on purpose. `empty`
  // asserts the backend confirmed zero items; a read past its freshness policy
  // confirmed nothing about now, so claiming an empty collection there would
  // be exactly the "renders as complete" failure section 8.2 forbids.
  if (input.observedAt && isBeyondFreshness(input.observedAt, input.now)) return 'stale';

  if (input.isAbsent) return 'unknown';
  if (input.isEmpty) return 'empty';
  return 'ready';
}
