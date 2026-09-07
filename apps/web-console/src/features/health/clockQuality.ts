import type { HealthView } from '@shared/api/types';

/**
 * Whether the source device reported a degraded clock (WCX-08 section 8.5).
 *
 * `clock_quality` is one of the eight conditions the Edge reports. When it is
 * not `True`, every timestamp attributed to that device is of uncertain
 * quality, and displaying one as authoritative would overstate the evidence —
 * in a product whose incident correctness *is* ordering, that is the wrong
 * direction to be wrong in.
 *
 * `Unknown` counts as degraded here on purpose. A device that has not reported
 * its clock quality has not established that its clock is good, and absence of
 * an observation is not a positive one.
 */
export function clockQualityIsDegraded(health: HealthView | undefined): boolean {
  if (health === undefined) return false;
  const clock = health.conditions.find((condition) => condition.type === 'clock_quality');
  return clock !== undefined && clock.status !== 'True';
}
