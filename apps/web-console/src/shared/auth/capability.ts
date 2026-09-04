/**
 * Presentation-only capability seam.
 *
 * This is NEVER a security authority. The Control Plane enforces every
 * authorization decision; this seam only decides whether a control renders as
 * available. A capability that returns `allowed` still fails server-side if
 * the session is not permitted, and the UI must render that rejection
 * truthfully.
 *
 * Phase 2 has one local owner and no role model (IA-06, AUTH-05), so every
 * capability resolves from whether the in-memory CSRF proof exists. The
 * `not-permitted` denial is declared but unreachable; callers must still
 * handle it exhaustively so a future role model needs no call-site changes.
 */
export type Capability =
  | 'environment.create'
  | 'environment.update'
  | 'zone.create'
  | 'zone.update'
  | 'zone.delete'
  | 'device.enroll'
  | 'device.disable'
  | 'device.revoke'
  | 'session.revoke'
  | 'account.password';

export type CapabilityDenial = 'reauthentication-required' | 'not-permitted';

export type CapabilityDecision =
  | { allowed: true }
  | { allowed: false; denial: CapabilityDenial };

const ALLOWED: CapabilityDecision = { allowed: true };
const REAUTHENTICATION_REQUIRED: CapabilityDecision = {
  allowed: false,
  denial: 'reauthentication-required',
};

/** Resolves a capability from the session's mutation readiness. */
export function resolveCapability(
  _capability: Capability,
  canMutate: boolean,
): CapabilityDecision {
  return canMutate ? ALLOWED : REAUTHENTICATION_REQUIRED;
}
