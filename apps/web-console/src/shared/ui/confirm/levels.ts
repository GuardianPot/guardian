import type { Capability } from '@shared/auth/capability';

/**
 * The action-to-confirmation-level table (WCX-04 section 9.3, WC-D16).
 *
 * A table in code, not a per-call-site judgement. `WCX-09` and `WCX-11` extend
 * this table; neither invents a level, and a call site cannot pass one.
 *
 * | Level | Meaning | Interaction |
 * |---|---|---|
 * | L1 | reversible | no confirmation; result feedback, undo where the backend supports it |
 * | L2 | destructive-recoverable | modal; the confirm button names the effect; the object is named |
 * | L3 | irreversible-security | modal; the operator types the exact object name; step-up reauthentication |
 */
export type ConfirmationLevel = 1 | 2 | 3;

export type ConfirmableAction =
  | 'device.enable'
  | 'device.disable'
  | 'zone.delete'
  | 'enrollment.revoke'
  | 'device.revoke'
  | 'session.revoke'
  | 'account.password';

export type ActionConfirmation = {
  level: ConfirmationLevel;
  /**
   * The capability that gates the control. A level 3 control stays visible and
   * disabled until its capability allows it (section 8.6), so the mapping
   * carries the capability rather than leaving each call site to guess.
   */
  capability: Capability;
  /** Names the effect. Becomes the confirm button label, never `OK`. */
  effect: string;
};

export const ACTION_CONFIRMATION: Readonly<Record<ConfirmableAction, ActionConfirmation>> = {
  // L1 — reversible. No confirmation; the result is reported and, where the
  // backend supports it, an undo affordance is offered at the call site.
  // Enable shares the disable capability: it is the same gate, inverted, and
  // re-enabling is immediate. `WCX-14` adds disposition changes at this level.
  'device.enable': { level: 1, capability: 'device.disable', effect: 'Enable device' },
  'device.disable': { level: 1, capability: 'device.disable', effect: 'Disable device' },

  // L2 — destructive but recoverable. Modal, effect-named confirm, object named.
  'zone.delete': { level: 2, capability: 'zone.delete', effect: 'Delete zone' },
  'enrollment.revoke': { level: 2, capability: 'enrollment.revoke', effect: 'Revoke enrollment token' },

  // L3 — irreversible and security-relevant. Modal, typed object name, and
  // step-up reauthentication. `WCX-09` implements the step-up; until then no
  // screen exposes an L3 action, so none is reachable.
  'device.revoke': { level: 3, capability: 'device.revoke', effect: 'Revoke device' },
  'session.revoke': { level: 3, capability: 'session.revoke', effect: 'Revoke session' },
  'account.password': { level: 3, capability: 'account.password', effect: 'Change password' },
};

export function confirmationFor(action: ConfirmableAction): ActionConfirmation {
  return ACTION_CONFIRMATION[action];
}

/**
 * Step-up reauthentication, as an interface only (section 9.3 rule 4).
 *
 * `WCX-09` implements this against approved change proposal `0003`. Until
 * then the only implementation is `stepUpUnavailable`, which refuses. A level
 * 3 confirmation therefore cannot complete, which is the intended state: no
 * screen exposes a level 3 action in this package.
 */
export type StepUpOutcome = { satisfied: true } | { satisfied: false; reason: 'not-implemented' };

export type StepUpReauthentication = (action: ConfirmableAction) => Promise<StepUpOutcome>;

export const stepUpUnavailable: StepUpReauthentication = () =>
  Promise.resolve({ satisfied: false, reason: 'not-implemented' });
