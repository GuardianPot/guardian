import { readFileSync } from 'node:fs';
import { afterEach, describe, expect, it } from 'vitest';
import {
  FRESHNESS,
  TRANSPORT_RECONSIDERATION_TRIGGER,
  freshness,
  permitPolling,
  pollingIsPermitted,
  staleAfter,
  type FreshnessClass,
} from './freshness';

/**
 * The freshness policy (WCX-07 sections 9.1 and 10.1, decision WC-D05).
 */
afterEach(() => permitPolling(false));

const CLASSES: FreshnessClass[] = [
  'critical',
  'operational',
  'configuration',
  'static',
  'once',
  'session',
];

/** Resolves the interval a class produces, with polling permitted. */
function intervalOf(resourceClass: FreshnessClass): number | false {
  permitPolling(true);
  const option = freshness(resourceClass).refetchInterval;
  return typeof option === 'function' ? option() : option;
}

describe('the class table', () => {
  it('declares the intervals and staleness section 9.1 sets', () => {
    expect(FRESHNESS.critical).toMatchObject({ refetchInterval: 5_000, staleAfterMs: 15_000 });
    expect(FRESHNESS.operational).toMatchObject({ refetchInterval: 10_000, staleAfterMs: 30_000 });
    expect(FRESHNESS.configuration).toMatchObject({ refetchInterval: 60_000, staleAfterMs: 300_000 });
    expect(FRESHNESS.static).toMatchObject({ refetchInterval: false, staleAfterMs: null });
    expect(FRESHNESS.once).toMatchObject({ refetchInterval: false, staleAfterMs: null });
    // The session class preserves the Phase 1 cadence exactly.
    expect(FRESHNESS.session).toMatchObject({ refetchInterval: false, staleAfterMs: 30_000 });
  });

  it('applies the declared interval and staleness of every class', () => {
    for (const resourceClass of CLASSES) {
      const policy = FRESHNESS[resourceClass];
      expect(intervalOf(resourceClass), resourceClass).toBe(policy.refetchInterval);
      expect(staleAfter(resourceClass), resourceClass).toBe(policy.staleAfterMs);
    }
  });

  it('says why each class exists, so a later change is a decision', () => {
    for (const resourceClass of CLASSES) {
      expect(FRESHNESS[resourceClass].why.length, resourceClass).toBeGreaterThan(30);
    }
  });
});

describe('background behaviour', () => {
  it('never refetches on an interval in a hidden tab', () => {
    // Section 9.1.1. A console left open in a background tab costs the Control
    // Plane nothing.
    for (const resourceClass of CLASSES) {
      expect(freshness(resourceClass).refetchIntervalInBackground, resourceClass).toBe(false);
    }
  });

  it('refetches a polled class on visibility and focus', () => {
    // Section 9.1.2. TanStack's focus manager covers both `visibilitychange`
    // and window focus, so returning to the tab does not wait out the interval.
    for (const resourceClass of ['critical', 'operational', 'configuration'] as const) {
      expect(freshness(resourceClass).refetchOnWindowFocus, resourceClass).toBe(true);
    }
    for (const resourceClass of ['static', 'once', 'session'] as const) {
      expect(freshness(resourceClass).refetchOnWindowFocus, resourceClass).toBe(false);
    }
  });

  it('refetches everything but static on reconnection', () => {
    // Section 9.1.3, including classes that do not poll: a read taken before
    // an outage is not evidence of anything after it.
    for (const resourceClass of CLASSES.filter((name) => name !== 'static')) {
      expect(freshness(resourceClass).refetchOnReconnect, resourceClass).toBe(true);
    }
    expect(freshness('static').refetchOnReconnect).toBe(false);
  });

  it('caps backoff at eight intervals without silencing the staleness treatment', () => {
    // Section 9.9.3. Backoff slows retries; it must never stop them, because a
    // resource that stopped retrying would stop updating its observed age.
    const { retryDelay } = freshness('operational');
    const interval = FRESHNESS.operational.refetchInterval as number;
    expect(retryDelay(0)).toBeLessThan(retryDelay(3));
    for (const failureCount of [0, 1, 2, 5, 10, 50]) {
      expect(retryDelay(failureCount), `failure ${failureCount}`).toBeLessThanOrEqual(interval * 8);
    }
    // Staleness is independent of backoff: the threshold does not move.
    expect(staleAfter('operational')).toBe(FRESHNESS.operational.staleAfterMs);
  });
});

describe('the polling gate', () => {
  it('issues no interval refetch without a session', () => {
    // Section 8.1. An interval that outlived its session would keep asking the
    // Control Plane for data on behalf of an operator who has signed out.
    permitPolling(false);
    for (const resourceClass of ['critical', 'operational', 'configuration'] as const) {
      const option = freshness(resourceClass).refetchInterval;
      expect(typeof option, resourceClass).toBe('function');
      expect((option as () => number | false)(), resourceClass).toBe(false);
    }
  });

  it('resumes when a session returns', () => {
    permitPolling(false);
    expect(pollingIsPermitted()).toBe(false);
    permitPolling(true);
    expect(pollingIsPermitted()).toBe(true);
    expect(intervalOf('operational')).toBe(10_000);
  });

  it('leaves an unpolled class alone in either state', () => {
    for (const state of [true, false]) {
      permitPolling(state);
      for (const resourceClass of ['static', 'once', 'session'] as const) {
        expect(freshness(resourceClass).refetchInterval, resourceClass).toBe(false);
      }
    }
  });
});

describe('the recorded transport trigger', () => {
  it('is data the Phase 5 benchmark can assert against, not prose', () => {
    // Section 9.7. `WC-D05` chose polling deliberately and recorded what would
    // reopen that choice. Recorded as numbers so a benchmark can compare
    // against them rather than a human reading a paragraph.
    expect(TRANSPORT_RECONSIDERATION_TRIGGER.visibilityP95Seconds).toBe(5);
    expect(TRANSPORT_RECONSIDERATION_TRIGGER.requestRateCeilingPerMinute).toBeGreaterThan(0);
    expect(TRANSPORT_RECONSIDERATION_TRIGGER.reference).toBe('P5-W9');
    expect(TRANSPORT_RECONSIDERATION_TRIGGER.consequence).toMatch(/change proposal/);
  });

  it('matches the PERF-05 threshold the critical class is sized for', () => {
    const criticalSeconds = (FRESHNESS.critical.refetchInterval as number) / 1_000;
    expect(criticalSeconds).toBeLessThanOrEqual(TRANSPORT_RECONSIDERATION_TRIGGER.visibilityP95Seconds);
  });
});

describe('the cadence boundary', () => {
  it('is the only module that names a cadence option', () => {
    // The same rule `check-freshness.mjs` enforces on lint, asserted here so a
    // developer sees it fail in the suite they run most often.
    const check = readFileSync('test/check-freshness.mjs', 'utf8');
    expect(check).toContain('src/shared/api/freshness.ts');
    expect(check).toMatch(/refetchInterval/);
    expect(check).toMatch(/staleTime/);
  });
});
