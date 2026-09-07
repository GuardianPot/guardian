import { Suspense, lazy, useCallback, useRef, useState, type ReactNode } from 'react';
import type { ConfirmableAction, StepUpOutcome, StepUpReauthentication } from '@shared/ui';

/**
 * Step-up reauthentication for irreversible actions (WCX-09 section 9.2).
 *
 * `IA-06` and `AUTH-05` keep role-based access out of the MVP, so there is no
 * authorization gradient available: every mutation is equally reachable once a
 * CSRF proof exists. Reauthentication is the only mechanism left that can
 * distinguish revoking a device from renaming a zone, which is why change
 * proposal `0003` introduced it.
 *
 * Three properties this hook exists to hold:
 *
 * 1. **Password and a fresh MFA proof, every time.** Enforced by
 *    `StepUpDialog`, which re-runs the login exchange.
 * 2. **The mark is single-use and scoped to one action.** `request` sets it,
 *    `consume` spends it, and nothing else can read it. A second irreversible
 *    action finds nothing and reauthenticates again. The mark is a ref rather
 *    than state because `consume` runs in the event handler that follows the
 *    resolved promise, and a re-render is not guaranteed between them.
 * 3. **Nothing is kept.** The credentials live in the dialog's form state for
 *    the duration of one submission, and the dialog unmounts after it. The
 *    re-issued CSRF proof goes to the auth context, in memory, exactly as the
 *    sign-in screen's does.
 *
 * The prompt itself is loaded on demand. This hook is reachable from the
 * shell; the dialog is not needed until an operator reaches for an
 * irreversible action, and it brings Radix's dialog with it.
 */
const StepUpDialog = lazy(() => import('./StepUpDialog').then((module) => ({ default: module.StepUpDialog })));

export type StepUp = StepUpReauthentication & {
  /** Render this once, near the action it guards. */
  element: ReactNode;
};

/** A resolver waiting on the dialog, held while it is open. */
type Pending = {
  action: ConfirmableAction;
  settle: (outcome: StepUpOutcome) => void;
};

export function useStepUp(): StepUp {
  const [pending, setPending] = useState<Pending | null>(null);
  const mark = useRef<ConfirmableAction | null>(null);

  const request = useCallback(
    (action: ConfirmableAction) =>
      new Promise<StepUpOutcome>((resolve) => {
        // Any mark left from an abandoned action is dropped here rather than
        // carried into this one.
        mark.current = null;
        setPending({ action, settle: resolve });
      }),
    [],
  );

  const consume = useCallback((action: ConfirmableAction) => {
    if (mark.current !== action) return false;
    mark.current = null;
    return true;
  }, []);

  const close = useCallback((outcome: StepUpOutcome) => {
    setPending((open) => {
      open?.settle(outcome);
      return null;
    });
  }, []);

  const element = pending === null ? null : (
    // No fallback: the prompt is a modal, and a spinner where a modal is about
    // to appear reads as a second thing happening.
    <Suspense fallback={null}>
      <StepUpDialog
        onSatisfied={() => {
          mark.current = pending.action;
          close({ satisfied: true });
        }}
        onCancelled={() => { close({ satisfied: false, reason: 'cancelled' }); }}
        onRateLimited={() => { close({ satisfied: false, reason: 'rate-limited' }); }}
      />
    </Suspense>
  );

  // Not memoised: `element` is new every render, so a memo over the whole
  // object would be a memo that never hits. `request` and `consume` are
  // `useCallback`-stable on their own, and `ConfirmationDialog` depends on
  // `request` rather than on this object for exactly that reason.
  return { request, consume, element };
}
