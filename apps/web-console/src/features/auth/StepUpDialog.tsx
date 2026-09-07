import { useState, type FormEvent, type ReactNode } from 'react';
import { toConsoleError } from '@shared/api/error';
import { textField } from '@shared/forms/textField';
import { Button, Dialog, InlineMessage, TextField } from '@shared/ui';
import { t } from '@shared/text';
import styles from '@shared/styles/app.module.css';
import { useAuth } from './AuthContext';
import { MfaMethodField, useMfaMethod } from './MfaMethodField';

/**
 * The reauthentication prompt (WCX-09 section 9.2).
 *
 * Split from `useStepUp` so it can be loaded on demand. The prompt pulls
 * Radix's dialog — a focus trap, a dismissable layer, a portal, and scroll
 * locking — and `useStepUp` is reachable from the shell, so keeping the two
 * in one module put all of that into the chunk an unauthenticated visitor
 * downloads. It is needed at the moment an operator reaches for an
 * irreversible action and not one moment earlier.
 *
 * Password and a fresh MFA proof, every time: not the session cookie, not a
 * cached proof. The console re-runs the login exchange, which is the only path
 * the Control Plane offers that verifies both.
 */
export type StepUpDialogProps = {
  onSatisfied: () => void;
  onCancelled: () => void;
  onRateLimited: () => void;
};

/** Stable, so the three inputs can point at the one message covering them. */
const STEP_UP_ERROR_ID = 'step-up-error';

export function StepUpDialog({ onSatisfied, onCancelled, onRateLimited }: StepUpDialogProps) {
  const auth = useAuth();
  const method = useMfaMethod();
  const [error, setError] = useState<ReactNode>(null);
  const [submitting, setSubmitting] = useState(false);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const proof = textField(form, 'step_up_proof');
    setError(null);
    setSubmitting(true);
    try {
      await auth.login({
        username: textField(form, 'step_up_username'),
        password: textField(form, 'step_up_password'),
        ...(method.value === 'totp' ? { totp_code: proof } : { recovery_code: proof }),
      });
      event.currentTarget.reset();
      onSatisfied();
    } catch (caught) {
      // Section 9.2.4: a generic denial. The submitted values are never
      // reflected, and the reason never distinguishes a wrong password from an
      // unknown account. Rate limiting is the one case that closes the dialog,
      // because retrying is exactly what must not happen next.
      const limited = toConsoleError(caught).kind === 'rate-limited';
      setError(limited ? t('auth.rateLimited') : t('auth.denied'));
      setSubmitting(false);
      if (limited) onRateLimited();
    }
  }

  return (
    <Dialog
      open
      onClose={onCancelled}
      title={t('stepUp.title')}
      description={t('stepUp.description')}
      actions={
        <>
          <Button variant="quiet" onClick={onCancelled}>{t('common.cancel')}</Button>
          <Button variant="primary" type="submit" form="step-up-form" pending={submitting}>
            {t('stepUp.submit')}
          </Button>
        </>
      }
    >
      <form id="step-up-form" className={styles.stepUpForm} onSubmit={(event) => { void submit(event); }}>
        <TextField
          name="step_up_username"
          label={t('auth.username')}
          autoComplete="username"
          required
          {...(error === null ? {} : { invalidatedBy: STEP_UP_ERROR_ID })}
        />
        <TextField
          name="step_up_password"
          label={t('auth.password')}
          type="password"
          autoComplete="current-password"
          required
          {...(error === null ? {} : { invalidatedBy: STEP_UP_ERROR_ID })}
        />
        <MfaMethodField method={method} />
        {/*
          Remounted when the method changes, so an authenticator code never
          travels into a recovery-code submission.
        */}
        <TextField
          key={method.value}
          name="step_up_proof"
          label={method.value === 'totp' ? t('auth.totpLabel') : t('auth.recoveryLabel')}
          // A one-time proof is never remembered, so the browser is told not
          // to offer a stored one.
          autoComplete="one-time-code"
          required
          {...(error === null ? {} : { invalidatedBy: STEP_UP_ERROR_ID })}
        />
        {error !== null && <InlineMessage id={STEP_UP_ERROR_ID} tone="error">{error}</InlineMessage>}
      </form>
    </Dialog>
  );
}
