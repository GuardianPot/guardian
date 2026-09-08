import { useEffect, useState, type ReactNode } from 'react';
import { toConsoleError } from '@shared/api/error';
import { schemaFor, useConsoleForm } from '@shared/forms';
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

/*
 * The same contract-derived validator sign-in uses, on the same three fields.
 * One proof control satisfies either `totp_code` or `recovery_code`; the
 * operator's method choice decides which, and both bounds come from the
 * contract rather than from a pattern typed here.
 */
const stepUpSchema = schemaFor<'AuthLoginRequest', { step_up_username: string; step_up_password: string; step_up_proof: string }>(
  'AuthLoginRequest',
  [],
  {
    step_up_username: ['username'],
    step_up_password: ['password'],
    step_up_proof: ['totp_code', 'recovery_code'],
  },
);

export function StepUpDialog({ onSatisfied, onCancelled, onRateLimited }: StepUpDialogProps) {
  const auth = useAuth();
  const method = useMfaMethod();
  const [error, setError] = useState<ReactNode>(null);
  const [submitting, setSubmitting] = useState(false);

  const form = useConsoleForm<{ step_up_username: string; step_up_password: string; step_up_proof: string }>({
    schema: stepUpSchema,
    defaultValues: { step_up_username: '', step_up_password: '', step_up_proof: '' },
    onSubmit: async (values) => {
      setError(null);
      setSubmitting(true);
      try {
        await auth.login({
          username: values.step_up_username,
          password: values.step_up_password,
          ...(method.value === 'totp'
            ? { totp_code: values.step_up_proof }
            : { recovery_code: values.step_up_proof }),
        });
        // No form reset: the dialog unmounts on success, which destroys the
        // fields and everything typed into them.
        onSatisfied();
      } catch (caught) {
        // Section 9.2.4: a generic denial. The submitted values are never
        // reflected, and the reason never distinguishes a wrong password from
        // an unknown account. Rate limiting is the one case that closes the
        // dialog, because retrying is exactly what must not happen next.
        const limited = toConsoleError(caught).kind === 'rate-limited';
        setError(limited ? t('auth.rateLimited') : t('auth.denied'));
        setSubmitting(false);
        if (limited) onRateLimited();
      }
    },
  });

  /*
   * The proof the stack holds is cleared when the method changes, for the same
   * reason sign-in does it: the control is remounted on `key`, and React Hook
   * Form would otherwise carry an authenticator code into a recovery-code
   * submission while the input looked empty.
   */
  useEffect(() => {
    form.form.setValue('step_up_proof', '');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [method.value]);

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
      <form id="step-up-form" className={styles.stepUpForm} onSubmit={(event) => { void form.submit(event); }}>
        <TextField
          name="step_up_username"
          registration={form.form.register('step_up_username')}
          label={t('auth.username')}
          autoComplete="username"
          required
          {...(error === null ? {} : { invalidatedBy: STEP_UP_ERROR_ID })}
        />
        <TextField
          name="step_up_password"
          registration={form.form.register('step_up_password')}
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
          registration={form.form.register('step_up_proof')}
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
