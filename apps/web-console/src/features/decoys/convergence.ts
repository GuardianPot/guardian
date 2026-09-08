import type { DecoyView } from '@shared/api/types';

/**
 * Desired versus observed, kept apart (WCX-11 sections 9.3.1, 9.5 and 8.7).
 *
 * Two questions that a single field would collapse into a lie:
 *
 * - *has the Edge caught up with what the operator asked for?* That is
 *   convergence, and it is answered by comparing revisions.
 * - *what is the decoy actually doing?* That is the observed state, and only an
 *   Edge can answer it.
 *
 * A converged decoy is not therefore a healthy one, and a pending decoy is not
 * therefore broken. The list renders them as separate columns for that reason,
 * and nothing here derives one from the other.
 */

/**
 * `DC-12` and the Phase 2 target: a cached artefact should be deployed within
 * sixty seconds. This is the point past which the console says convergence is
 * taking longer than expected.
 */
export const CONVERGENCE_WINDOW_MS = 60_000;

export type Convergence =
  /** The Edge has confirmed the revision the operator is looking at. */
  | { state: 'converged'; observedRevision: number }
  /** Asked for, not yet confirmed, and still inside the expected window. */
  | { state: 'pending'; desiredRevision: number; observedRevision: number | null; elapsedMs: number }
  /**
   * Still unconfirmed past the window. This is *not* a failure verdict: the
   * console has not been told anything went wrong, only that it has not been
   * told anything at all. Section 9.5 forbids inventing a timeout, so this
   * reports the wait and leaves the observed column to say what an Edge
   * actually reported.
   */
  | { state: 'overdue'; desiredRevision: number; observedRevision: number | null; elapsedMs: number }
  /**
   * Guardian cannot manage this decoy, so convergence is not a meaningful
   * question. Asking it would imply the Control Plane is still driving
   * something it has lost (SEC-06).
   */
  | { state: 'unmanaged' };

export function convergenceOf(view: DecoyView, now: number = Date.now()): Convergence {
  if (view.observed.observed_state === 'unmanaged') return { state: 'unmanaged' };

  const desiredRevision = view.decoy.revision;
  const observedRevision = view.observed.desired_revision ?? null;
  if (observedRevision !== null && observedRevision >= desiredRevision) {
    return { state: 'converged', observedRevision };
  }

  // Elapsed is measured from the configuration change the Edge has not yet
  // acknowledged, which is what the operator is waiting on.
  const changedAt = Date.parse(view.decoy.updated_at);
  const elapsedMs = Number.isNaN(changedAt) ? 0 : Math.max(0, now - changedAt);
  const pending = { desiredRevision, observedRevision, elapsedMs };
  return elapsedMs > CONVERGENCE_WINDOW_MS
    ? { state: 'overdue', ...pending }
    : { state: 'pending', ...pending };
}

/**
 * Whether an observation is old enough to distrust.
 *
 * Separate from convergence: an Edge can be perfectly converged and have
 * stopped reporting since. Section 9.9.3 wants that said out loud with the
 * observation's age rather than left to look current.
 */
export function observationAgeMs(view: DecoyView, now: number = Date.now()): number | null {
  const reportedAt = view.observed.reported_at;
  if (reportedAt === null || reportedAt === undefined) return null;
  const parsed = Date.parse(reportedAt);
  return Number.isNaN(parsed) ? null : Math.max(0, now - parsed);
}

/**
 * Whether anything has ever reported on this decoy.
 *
 * `reported_at` being absent is the honest "nobody has looked" case, and it is
 * why `observed_state` is `unknown` rather than `absent`: sections 8.7 and
 * 9.9.4 both turn on the difference between *no observation* and *an
 * observation that the decoy is missing*.
 */
export function hasBeenObserved(view: DecoyView): boolean {
  return view.observed.reported_at !== null && view.observed.reported_at !== undefined;
}
