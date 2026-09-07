import { useEffect, useRef, useState } from 'react';
import { Button } from '@shared/ui/controls/Button';
import { Dialog } from '@shared/ui/controls/Dialog';
import { TextField } from '@shared/ui/controls/TextField';
import { InlineMessage } from '@shared/ui/feedback/InlineMessage';
import {
  confirmationFor,
  stepUpUnavailable,
  type ConfirmableAction,
  type StepUpOutcome,
  type StepUpReauthentication,
} from './levels';
import { t, type PlainCatalogueKey } from '@shared/text';

/**
 * Levels 2 and 3 of the confirmation model (WCX-04 section 9.3, WC-D16).
 *
 * The level comes from the action table, never from the call site, so a screen
 * cannot downgrade a device revoke to a level 2 prompt. Level 1 is reversible
 * and has no dialog at all; passing a level 1 action here renders nothing, on
 * purpose, so a screen that adds a modal to a reversible action gets no modal
 * rather than a wrong one.
 *
 * Focus lands on cancel. Radix would otherwise focus the first tabbable
 * element, and the confirm control must never be the default (rule 3).
 *
 * `objectName` is attacker-influenced — a device or zone display name — and is
 * rendered as text and compared with exact string equality. It is never parsed,
 * never interpolated into markup, and never used as a selector. The typed
 * value is compared here and is never sent anywhere (`WCX-09` section 8.5).
 *
 * For level 3 the reauthentication comes **first** (`WCX-09` section 9.2.1).
 * Opening the confirmation runs `stepUp.request`; the typed-confirmation body
 * appears only after that succeeds. Asking an operator to type a device name
 * and only then telling them to reauthenticate wastes the typing, and worse,
 * presents the confirmation as if the action were already authorised.
 */
export type ConfirmationDialogProps = {
  action: ConfirmableAction;
  /** The affected object, named in the dialog and typed back for level 3. */
  objectName: string;
  open: boolean;
  onCancel: () => void;
  onConfirm: () => void;
  stepUp?: StepUpReauthentication;
};

/** The step-up messages, one per refusal reason. Never a submitted value. */
const REFUSAL: Record<Exclude<StepUpOutcome, { satisfied: true }>['reason'], PlainCatalogueKey> = {
  'not-implemented': 'confirm.stepUpUnavailable',
  cancelled: 'confirm.stepUpCancelled',
  denied: 'confirm.stepUpDenied',
  'rate-limited': 'confirm.stepUpRateLimited',
};

export function ConfirmationDialog({
  action,
  objectName,
  open,
  onCancel,
  onConfirm,
  stepUp = stepUpUnavailable,
}: ConfirmationDialogProps) {
  const confirmation = confirmationFor(action);
  // The effect below depends on this one function, not on the whole seam: an
  // implementation that rebuilds its object each render would otherwise
  // re-request reauthentication on every render.
  const { request: requestStepUp } = stepUp;
  const cancelRef = useRef<HTMLButtonElement>(null);
  const [typed, setTyped] = useState('');
  const [refusal, setRefusal] = useState<keyof typeof REFUSAL | null>(null);
  const [steppedUp, setSteppedUp] = useState(false);

  const irreversible = confirmation.level === 3;

  // Reset when the dialog opens or closes, during render rather than in an
  // effect. A typed name and a step-up approval must not survive a cancel and
  // be waiting for the next open; doing it in an effect would leave one frame
  // where they still are.
  const [lastOpen, setLastOpen] = useState(open);
  if (lastOpen !== open) {
    setLastOpen(open);
    setTyped('');
    setRefusal(null);
    setSteppedUp(false);
  }

  const needsStepUp = open && lastOpen === open && irreversible && !steppedUp && refusal === null;

  useEffect(() => {
    if (!needsStepUp) return;
    let current = true;
    void requestStepUp(action).then((outcome) => {
      if (!current) return;
      if (outcome.satisfied) setSteppedUp(true);
      else setRefusal(outcome.reason);
    });
    return () => { current = false; };
  }, [action, needsStepUp, requestStepUp]);

  if (confirmation.level === 1) return null;

  const nameMatches = typed === objectName;
  const confirmBlocked = irreversible && !nameMatches;
  const effect = t(confirmation.effect);

  function confirm() {
    // Spending the mark is what makes it single-use. A second irreversible
    // action finds nothing to spend and reauthenticates again.
    if (irreversible && !stepUp.consume(action)) {
      setSteppedUp(false);
      setRefusal('denied');
      return;
    }
    onConfirm();
  }

  return (
    <Dialog
      // While reauthentication is in flight the step-up dialog owns the
      // screen. Two stacked modals would fight over the focus trap.
      open={open && (!irreversible || steppedUp || refusal !== null)}
      onClose={onCancel}
      title={effect}
      description={
        irreversible
          ? t('confirm.irreversible', { effect, object: objectName })
          : t('confirm.recoverable', { effect, object: objectName })
      }
      initialFocus={cancelRef}
      actions={
        <>
          <Button variant="quiet" buttonRef={cancelRef} onClick={onCancel}>
            {t('common.cancel')}
          </Button>
          {refusal === null && (
            <Button
              variant="destructive"
              onClick={confirm}
              {...(confirmBlocked ? { disabledReason: t('confirm.typeToConfirm', { object: objectName }) } : {})}
            >
              {effect}
            </Button>
          )}
        </>
      }
    >
      {irreversible && steppedUp && refusal === null && (
        <TextField
          name="confirm_object_name"
          label={t('confirm.typeLabel')}
          description={t('confirm.typeToConfirm', { object: objectName })}
          value={typed}
          onChange={setTyped}
          {...(typed !== '' && !nameMatches ? { error: t('confirm.typeMismatch') } : {})}
        />
      )}
      {refusal !== null && <InlineMessage tone="error">{t(REFUSAL[refusal])}</InlineMessage>}
    </Dialog>
  );
}
