import { describe, expect, it } from 'vitest';
import { consoleError, type ConsoleErrorKind } from '@shared/api/error';
import { FRESHNESS_LIMIT_MS } from './freshness';
import { DATA_STATES, resolveDataState, type DataStateInput } from './resolveDataState';

/**
 * Every row of the `WCX-04` section 9.1 mapping table, asserted.
 *
 * This is the table an operator's understanding of the product rests on: it
 * decides whether a refused read looks like an empty list, and whether an
 * absent observation looks like a healthy one.
 */
const base: DataStateInput = { isPending: false, hasData: false, error: null };

const success = (extra: Partial<DataStateInput> = {}): DataStateInput => ({
  ...base,
  hasData: true,
  ...extra,
});

const failure = (kind: ConsoleErrorKind, extra: Partial<DataStateInput> = {}): DataStateInput => ({
  ...base,
  error: consoleError(kind),
  ...extra,
});

describe('resolveDataState — the section 9.1 mapping table', () => {
  it('maps `isPending` with no cached data to loading', () => {
    expect(resolveDataState({ ...base, isPending: true })).toBe('loading');
  });

  it('does not map `isPending` to loading once cached data exists', () => {
    // A background refetch must not blank data the operator is reading.
    expect(resolveDataState({ ...base, isPending: true, hasData: true })).toBe('ready');
  });

  it('maps success with an empty collection to empty', () => {
    expect(resolveDataState(success({ isEmpty: true }))).toBe('empty');
  });

  it('maps success with an absent projection to unknown', () => {
    expect(resolveDataState(success({ isAbsent: true }))).toBe('unknown');
  });

  it('maps forbidden to denied', () => {
    expect(resolveDataState(failure('forbidden'))).toBe('denied');
  });

  it('maps unauthenticated to denied', () => {
    expect(resolveDataState(failure('unauthenticated'))).toBe('denied');
  });

  it('maps not-found on an observation-shaped resource to unknown', () => {
    expect(resolveDataState(failure('not-found', { observationShaped: true }))).toBe('unknown');
  });

  it('maps unavailable with cached data to stale', () => {
    expect(resolveDataState(failure('unavailable', { hasData: true }))).toBe('stale');
  });

  it('maps timeout with cached data to stale', () => {
    expect(resolveDataState(failure('timeout', { hasData: true }))).toBe('stale');
  });

  it('maps unavailable without cached data to degraded', () => {
    expect(resolveDataState(failure('unavailable'))).toBe('degraded');
  });

  it('maps timeout without cached data to degraded', () => {
    expect(resolveDataState(failure('timeout'))).toBe('degraded');
  });

  it('maps network with cached data to stale', () => {
    expect(resolveDataState(failure('network', { hasData: true }))).toBe('stale');
  });

  it('maps network without cached data to error', () => {
    expect(resolveDataState(failure('network'))).toBe('error');
  });

  it('maps any other error to error', () => {
    for (const kind of ['validation', 'conflict', 'rate-limited', 'unexpected'] as const) {
      expect(resolveDataState(failure(kind)), kind).toBe('error');
    }
    // A `not-found` that is not observation-shaped is a missing record, not a
    // missing observation, so it stays in the catch-all.
    expect(resolveDataState(failure('not-found'))).toBe('error');
    expect(resolveDataState(failure('not-found', { hasData: true }))).toBe('error');
  });

  it('maps a reauthentication requirement to denied rather than to the catch-all', () => {
    // The table names `forbidden` and `unauthenticated`; this kind is the
    // third authorization refusal in the taxonomy, and falling through to
    // `error` would render a refusal as an unexplained failure — exactly the
    // confusion section 8.3 exists to prevent.
    expect(resolveDataState(failure('reauthentication-required'))).toBe('denied');
  });

  it('never resolves an authorization refusal to empty', () => {
    // Section 8.3. An empty list would tell an operator that nothing exists
    // when access was in fact refused.
    for (const kind of ['forbidden', 'unauthenticated', 'reauthentication-required'] as const) {
      expect(resolveDataState(failure(kind, { isEmpty: true, hasData: true })), kind).not.toBe('empty');
    }
  });

  it('renders a successful read past its freshness policy as stale', () => {
    const now = Date.parse('2026-09-01T12:00:00Z');
    const observedAt = new Date(now - FRESHNESS_LIMIT_MS - 1).toISOString();
    expect(resolveDataState(success({ observedAt, now }))).toBe('stale');
    // And an empty collection past its policy is stale, not a confirmed zero.
    expect(resolveDataState(success({ observedAt, isEmpty: true, now }))).toBe('stale');
  });

  it('leaves a read inside its freshness policy alone', () => {
    const now = Date.parse('2026-09-01T12:00:00Z');
    const observedAt = new Date(now - 1_000).toISOString();
    expect(resolveDataState(success({ observedAt, now }))).toBe('ready');
    expect(resolveDataState(success({ observedAt, isEmpty: true, now }))).toBe('empty');
  });

  it('maps a mixed read to partial', () => {
    expect(resolveDataState(success({ partialFailures: ['device health'] }))).toBe('partial');
  });

  it('classifies every error kind in the taxonomy', () => {
    const kinds: ConsoleErrorKind[] = [
      'unauthenticated',
      'reauthentication-required',
      'forbidden',
      'not-found',
      'validation',
      'conflict',
      'rate-limited',
      'unavailable',
      'timeout',
      'network',
      'unexpected',
    ];
    for (const kind of kinds) {
      for (const hasData of [true, false]) {
        const state = resolveDataState(failure(kind, { hasData }));
        expect(DATA_STATES, `${kind} with hasData=${String(hasData)}`).toContain(state);
      }
    }
  });
});
