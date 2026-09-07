import { useRef, useState } from 'react';
import { Button } from '@shared/ui/controls/Button';
import { Dialog } from '@shared/ui/controls/Dialog';
import { TextField } from '@shared/ui/controls/TextField';
import { InlineMessage } from '@shared/ui/feedback/InlineMessage';
import { confirmationFor, stepUpUnavailable, type ConfirmableAction, type StepUpReauthentication } from './levels';
import { t } from '@shared/text';

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
 * never interpolated into markup, and never used as a selector.
 */
export type ConfirmationDialogProps = {
  action: ConfirmableAction;
  /** The affected object, named in the dialog and typed back for level 3. */
  objectName: string;
  open: boolean;
  onCancel: () => void;
  onConfirm: () => void;
  /** `WCX-09` supplies the real implementation. */
  stepUp?: StepUpReauthentication;
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
  const cancelRef = useRef<HTMLButtonElement>(null);
  const [typed, setTyped] = useState('');
  const [stepUpRefused, setStepUpRefused] = useState(false);

  if (confirmation.level === 1) return null;

  const irreversible = confirmation.level === 3;
  const nameMatches = typed === objectName;
  const confirmBlocked = irreversible && !nameMatches;

  async function confirm() {
    if (irreversible) {
      const outcome = await stepUp(action);
      if (!outcome.satisfied) {
        setStepUpRefused(true);
        return;
      }
    }
    onConfirm();
  }

  return (
    <Dialog
      open={open}
      onClose={onCancel}
      title={confirmation.effect}
      description={
        irreversible
          ? t('confirm.irreversible', { effect: confirmation.effect, object: objectName })
          : t('confirm.recoverable', { effect: confirmation.effect, object: objectName })
      }
      initialFocus={cancelRef}
      actions={
        <>
          <Button variant="quiet" buttonRef={cancelRef} onClick={onCancel}>
            {t('common.cancel')}
          </Button>
          <Button
            variant="destructive"
            onClick={() => { void confirm(); }}
            {...(confirmBlocked ? { disabledReason: t('confirm.typeToConfirm', { object: objectName }) } : {})}
          >
            {confirmation.effect}
          </Button>
        </>
      }
    >
      {irreversible && (
        <TextField
          name="confirm_object_name"
          label={t('confirm.typeLabel')}
          description={t('confirm.typeToConfirm', { object: objectName })}
          value={typed}
          onChange={setTyped}
        />
      )}
      {stepUpRefused && <InlineMessage tone="error">{t('confirm.stepUpRequired')}</InlineMessage>}
    </Dialog>
  );
}
