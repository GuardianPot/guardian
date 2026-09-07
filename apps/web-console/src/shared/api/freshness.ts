import { retryDelay, retryRead } from './query';

/**
 * The one freshness policy (WCX-07 section 9.1, decision WC-D05).
 *
 * Before this, four queries carried a hard-coded five-second interval and
 * nothing stopped the next screen inventing a fifth value. That matters more
 * than tidiness: `WC-D05` chose polling over a server-driven channel
 * *deliberately*, and recorded a measured condition for reconsidering that
 * choice. A trigger cannot be evaluated against constants scattered through
 * feature modules, so the cadence has to be one object.
 *
 * A resource picks a class. It never picks an interval. `check-freshness.mjs`
 * fails the lint if `refetchInterval` or `staleTime` appears outside this
 * module.
 */
export type FreshnessClass =
  | 'critical'
  | 'operational'
  | 'configuration'
  | 'static'
  | 'once'
  | 'session';

export type FreshnessParameters = {
  /** Milliseconds between background refetches, or `false` for none. */
  readonly refetchInterval: number | false;
  /**
   * Age past which the surface must render `stale`, even after a successful
   * fetch. `null` means the resource does not go stale.
   */
  readonly staleAfterMs: number | null;
  readonly why: string;
};

export const FRESHNESS: Readonly<Record<FreshnessClass, FreshnessParameters>> = {
  critical: {
    refetchInterval: 5_000,
    staleAfterMs: 15_000,
    why: 'An operator is watching for an intrusion. PERF-05 requires visibility within five seconds.',
  },
  operational: {
    refetchInterval: 10_000,
    staleAfterMs: 30_000,
    why: 'Health and inventory change on the order of seconds and are read continuously.',
  },
  configuration: {
    refetchInterval: 60_000,
    staleAfterMs: 300_000,
    why: 'Only an operator changes these, and the console already invalidates them on write.',
  },
  static: {
    refetchInterval: false,
    staleAfterMs: null,
    why: 'Enumerations and singletons do not change within a session.',
  },
  once: {
    refetchInterval: false,
    staleAfterMs: null,
    why: 'A one-time read such as an enrollment secret. Refetching it would be a second issue.',
  },
  session: {
    refetchInterval: false,
    staleAfterMs: 30_000,
    why: 'Preserves the Phase 1 cadence: no interval, and a probe at most twice a minute.',
  },
} as const;

/**
 * The measured condition for reconsidering the transport (section 9.7).
 *
 * Recorded as data rather than prose so the `P5-W9` benchmark can assert
 * against it. Until one of these is *measured*, polling stands — the decision
 * was made deliberately and is not reopened by opinion.
 */
export const TRANSPORT_RECONSIDERATION_TRIGGER = {
  reference: 'P5-W9',
  /** Seconds from decoy interaction to console visibility, p95, `critical`. */
  visibilityP95Seconds: 5,
  /** Aggregate console requests per minute across a three-tab session. */
  requestRateCeilingPerMinute: 180,
  tabsAssumed: 3,
  consequence:
    'Either threshold being exceeded opens a change proposal for a server-driven invalidation channel. Neither being exceeded means polling stands.',
} as const;

/**
 * Whether background polling is permitted at all (section 8.1).
 *
 * A console with no session must issue no request. Signing out already removes
 * non-session queries, which stops their intervals, but this is a second,
 * explicit gate: an interval that outlived its session would keep asking a
 * Control Plane for data on behalf of an operator who has left.
 *
 * A module-level flag rather than context, because TanStack consults
 * `refetchInterval` outside React's render tree.
 */
let pollingPermitted = false;

export function permitPolling(permitted: boolean): void {
  pollingPermitted = permitted;
}

export function pollingIsPermitted(): boolean {
  return pollingPermitted;
}

/**
 * Query options for a resource class.
 *
 * Spread into `queryOptions()` at the call site. Everything cadence-related
 * comes from here, so a feature module contains no timing decision at all.
 */
export function freshness(resourceClass: FreshnessClass) {
  const policy = FRESHNESS[resourceClass];
  const interval = policy.refetchInterval;
  const polled = interval !== false;

  return {
    // A resource is fresh for as long as its class says, so a remount inside
    // that window reuses the cache instead of issuing a request.
    staleTime: policy.staleAfterMs ?? Number.POSITIVE_INFINITY,
    /*
     * Consulted by TanStack after every fetch, so a session that ends between
     * ticks stops the next one. Returning `false` suspends the interval; it
     * resumes when a session returns.
     */
    refetchInterval: polled ? () => (pollingIsPermitted() ? interval : (false as const)) : (false as const),
    // Section 9.1.1. A hidden tab costs the Control Plane nothing.
    refetchIntervalInBackground: false,
    // Section 9.1.2. TanStack's focus manager covers both `visibilitychange`
    // and window focus, so returning to the tab refetches immediately rather
    // than waiting out the remainder of an interval.
    refetchOnWindowFocus: polled,
    // Section 9.1.3. Everything but `static` is worth re-reading after a
    // network outage, including resources that do not poll.
    refetchOnReconnect: resourceClass !== 'static',
    retry: resourceClass === 'session' || resourceClass === 'once' ? false : retryRead,
    retryDelay: (failureCount: number) =>
      // Section 9.9.3: backoff grows but never exceeds eight intervals, so a
      // failing resource keeps being retried rather than going quiet forever.
      polled ? Math.min(retryDelay(failureCount), interval * 8) : retryDelay(failureCount),
  } as const;
}

/** The age past which a class must render `stale`, for the boundary to use. */
export function staleAfter(resourceClass: FreshnessClass): number | null {
  return FRESHNESS[resourceClass].staleAfterMs;
}
