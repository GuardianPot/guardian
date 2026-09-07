/**
 * Observation age and the interim freshness threshold (WCX-04 section 9.1,
 * OPS-03).
 *
 * `WCX-07` owns the real per-read freshness policy. Until it lands the
 * threshold is one constant declared beside the rule that reads it, so there
 * is exactly one place to change and no screen can pick its own number.
 *
 * A read that succeeded but is older than this is rendered `stale`, not fresh.
 * Staleness is a product concern, not a transport detail: the alternative is
 * presenting an old observation as the current state of a network.
 */
export const FRESHNESS_LIMIT_MS = 60_000;

/** True when an observation is older than the policy allows. */
export function isBeyondFreshness(observedAt: string, now: number = Date.now()): boolean {
  const observed = Date.parse(observedAt);
  // An unparsable timestamp cannot be shown to be fresh, so it is not.
  if (Number.isNaN(observed)) return true;
  return now - observed > FRESHNESS_LIMIT_MS;
}

const UNITS: readonly { limit: number; size: number; one: string; many: string }[] = [
  { limit: 60_000, size: 1_000, one: 'second', many: 'seconds' },
  { limit: 3_600_000, size: 60_000, one: 'minute', many: 'minutes' },
  { limit: 86_400_000, size: 3_600_000, one: 'hour', many: 'hours' },
];

/**
 * Renders an observation age in whole units.
 *
 * `WCX-08` replaces this with the canonical timestamp presentation, which
 * pairs relative time with an absolute value. Until then this is only ever
 * shown beside the absolute time the caller already renders.
 */
export function formatAge(observedAt: string, now: number = Date.now()): string {
  const observed = Date.parse(observedAt);
  if (Number.isNaN(observed)) return 'an unknown age';
  const elapsed = Math.max(0, now - observed);
  const unit = UNITS.find((candidate) => elapsed < candidate.limit);
  if (!unit) {
    const days = Math.floor(elapsed / 86_400_000);
    return `${days} ${days === 1 ? 'day' : 'days'}`;
  }
  const count = Math.floor(elapsed / unit.size);
  return `${count} ${count === 1 ? unit.one : unit.many}`;
}
