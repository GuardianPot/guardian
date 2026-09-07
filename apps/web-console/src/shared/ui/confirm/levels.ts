import type { Capability } from '@shared/auth/capability';
import type { PlainCatalogueKey } from '@shared/text';

/**
 * The action-to-confirmation-level table (WCX-04 section 9.3, WC-D16).
 *
 * A table in code, not a per-call-site judgement. `WCX-09` extended it and
 * `WCX-11` will extend it again; neither invents a level, and a call site
 * cannot pass one.
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
  | 'zone.rename'
  | 'zone.delete'
  | 'enrollment.revoke'
  | 'device.revoke'
  | 'device.reenroll'
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
  /**
   * Names the effect. Becomes the confirm button label, never `OK`.
   *
   * A catalogue key rather than a string since `WCX-08`: this table is read by
   * components, so the words in it are components' words.
   */
  effect: PlainCatalogueKey;
};

export const ACTION_CONFIRMATION: Readonly<Record<ConfirmableAction, ActionConfirmation>> = {
  // L1 — reversible. No confirmation; the result is reported and, where the
  // backend supports it, an undo affordance is offered at the call site.
  // Enable shares the disable capability: it is the same gate, inverted, and
  // re-enabling is immediate. `WCX-14` adds disposition changes at this level.
  'device.enable': { level: 1, capability: 'device.disable', effect: 'confirm.effect.deviceEnable' },
  'zone.rename': { level: 1, capability: 'zone.update', effect: 'confirm.effect.zoneRename' },

  // L2 — destructive but recoverable. Modal, effect-named confirm, object named.
  //
  // `WCX-09` moved disable up from L1. It is reversible by re-enable, but it
  // stops a live Edge from opening new authenticated sessions, and an
  // operator who meant to disable a different device finds out from the
  // network rather than from the console.
  'device.disable': { level: 2, capability: 'device.disable', effect: 'confirm.effect.deviceDisable' },
  'zone.delete': { level: 2, capability: 'zone.delete', effect: 'confirm.effect.zoneDelete' },
  'enrollment.revoke': { level: 2, capability: 'enrollment.revoke', effect: 'confirm.effect.enrollmentRevoke' },

  // L3 — irreversible and security-relevant. Modal, typed object name, and
  // step-up reauthentication.
  //
  // Re-enrollment is L3 by Product Owner decision on 2026-09-04: it is the
  // inverse of revocation and re-establishes the trust revocation removed, so
  // it carries the same gate.
  'device.revoke': { level: 3, capability: 'device.revoke', effect: 'confirm.effect.deviceRevoke' },
  'device.reenroll': { level: 3, capability: 'device.reenroll', effect: 'confirm.effect.deviceReenroll' },
  'session.revoke': { level: 3, capability: 'session.revoke', effect: 'confirm.effect.sessionRevoke' },
  'account.password': { level: 3, capability: 'account.password', effect: 'confirm.effect.accountPassword' },
};

export function confirmationFor(action: ConfirmableAction): ActionConfirmation {
  return ACTION_CONFIRMATION[action];
}

/**
 * Step-up reauthentication (WCX-09 section 9.2, change proposal 0003).
 *
 * `WCX-04` declared this as a refusing interface. The implementation is
 * `useStepUp` in the auth feature; the seam stays here so `@shared/ui` never
 * imports a feature.
 *
 * Two operations rather than one, because the marker's lifetime is the point.
 * `request` reauthenticates and marks *one* action; `consume` spends that mark
 * and clears it. An action that never reaches `consume` leaves nothing behind,
 * and a second action finds nothing to spend — which is what "scoped to a
 * single action" has to mean if it is to be testable.
 */
export type StepUpOutcome =
  | { satisfied: true }
  | { satisfied: false; reason: 'not-implemented' | 'cancelled' | 'denied' | 'rate-limited' };

export type StepUpReauthentication = {
  /**
   * Reauthenticates, then marks this one action as stepped up.
   *
   * Declared as a property holding a function rather than as a method, so it
   * can be destructured and passed as an effect dependency without `this`
   * ever entering the picture.
   */
  request: (action: ConfirmableAction) => Promise<StepUpOutcome>;
  /** Spends the mark. True at most once per `request` that succeeded. */
  consume: (action: ConfirmableAction) => boolean;
};

export const stepUpUnavailable: StepUpReauthentication = {
  request: () => Promise.resolve({ satisfied: false, reason: 'not-implemented' }),
  consume: () => false,
};
