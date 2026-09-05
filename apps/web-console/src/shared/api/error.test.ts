import { describe, expect, it } from 'vitest';
import {
  CONSOLE_ERROR_TEXT,
  ConsoleRequestError,
  consoleError,
  kindForStatus,
  statusSlug,
  toConsoleError,
  type ConsoleErrorKind,
} from './error';

describe('console error taxonomy', () => {
  it('maps every row of the approved status table', () => {
    const rows: [number, boolean, ConsoleErrorKind][] = [
      [401, false, 'unauthenticated'],
      [401, true, 'reauthentication-required'],
      [403, false, 'forbidden'],
      [404, false, 'not-found'],
      [400, false, 'validation'],
      [422, false, 'validation'],
      [409, false, 'conflict'],
      [412, false, 'conflict'],
      [429, false, 'rate-limited'],
      [500, false, 'unavailable'],
      [502, false, 'unavailable'],
      [503, false, 'unavailable'],
      [504, false, 'unavailable'],
      [418, false, 'unexpected'],
    ];
    for (const [status, sessionActive, kind] of rows) {
      expect(kindForStatus(status, sessionActive)).toBe(kind);
    }
  });

  it('marks only transient kinds retryable', () => {
    for (const kind of ['rate-limited', 'unavailable', 'timeout', 'network'] as const) {
      expect(consoleError(kind).retryable).toBe(true);
    }
    for (const kind of ['unauthenticated', 'forbidden', 'not-found', 'validation', 'conflict', 'unexpected'] as const) {
      expect(consoleError(kind).retryable).toBe(false);
    }
  });

  it('classifies an abort as timeout and a fetch failure as network', () => {
    expect(toConsoleError(new DOMException('aborted', 'AbortError')).kind).toBe('timeout');
    expect(toConsoleError(new TypeError('Failed to fetch')).kind).toBe('network');
  });

  it('falls back to unexpected for anything unrecognised', () => {
    for (const value of [undefined, null, 'boom', 42, new Error('boom'), {}]) {
      expect(toConsoleError(value).kind).toBe('unexpected');
    }
  });

  it('keeps a hostile backend slug out of the operator-facing message', () => {
    const hostile = '<img src=x onerror=alert(1)>[31m';
    const error = consoleError('unavailable', { statusSlug: hostile, httpStatus: 503 });
    expect(error.statusSlug).toBe(hostile);
    // The rendered text comes from the fixed table, never from the backend.
    expect(new ConsoleRequestError(error).message).toBe(CONSOLE_ERROR_TEXT.unavailable);
    expect(Object.values(CONSOLE_ERROR_TEXT)).not.toContain(hostile);
  });

  it('reads a status slug only from a string field', () => {
    expect(statusSlug({ status: 'invalid_argument' })).toBe('invalid_argument');
    expect(statusSlug({ status: 42 })).toBeUndefined();
    expect(statusSlug(null)).toBeUndefined();
    expect(statusSlug('nope')).toBeUndefined();
  });

  it('gives every kind a catalogue key and text entry', () => {
    const kinds: ConsoleErrorKind[] = [
      'unauthenticated', 'reauthentication-required', 'forbidden', 'not-found',
      'validation', 'conflict', 'rate-limited', 'unavailable', 'timeout',
      'network', 'unexpected',
    ];
    for (const kind of kinds) {
      expect(consoleError(kind).messageKey).toBe(`errors.${kind}`);
      expect(CONSOLE_ERROR_TEXT[kind]).toBeTruthy();
    }
  });
});
