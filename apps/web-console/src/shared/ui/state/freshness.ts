import { staleAfter, type FreshnessClass } from '@shared/api/freshness';

/**
 * Observation age and the staleness threshold (WCX-04 section 9.1, OPS-03,
 * WCX-07 section 9.1.4).
 *
 * `WCX-04` shipped this as one interim constant beside the rule that read it.
 * `WCX-07` replaced the constant with the per-class policy: a surface now
 * declares which freshness class its read belongs to, and the threshold comes
 * from the same object that sets the refetch interval. One place decides both,
 * so a cadence change cannot leave the staleness treatment behind.
 *
 * A read that succeeded but is older than its class allows is rendered
 * `stale`, not fresh. Staleness is a product concern, not a transport detail:
 * the alternative is presenting an old observation as the current state of a
 * network.
 */
export const DEFAULT_FRESHNESS_CLASS: FreshnessClass = 'operational';

/** True when an observation is older than its class allows. */
export function isBeyondFreshness(
  observedAt: string,
  now: number = Date.now(),
  resourceClass: FreshnessClass = DEFAULT_FRESHNESS_CLASS,
): boolean {
  const limit = staleAfter(resourceClass);
  // A class that never goes stale never goes stale.
  if (limit === null) return false;
  const observed = Date.parse(observedAt);
  // An unparsable timestamp cannot be shown to be fresh, so it is not.
  if (Number.isNaN(observed)) return true;
  return now - observed > limit;
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
