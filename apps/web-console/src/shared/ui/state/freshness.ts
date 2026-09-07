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

// `WCX-04` shipped an age formatter here with its unit words inline. `WCX-08`
// moved both to `time/Timestamp`, so the `stale` state and a relative
// timestamp read from one implementation and one catalogue.
