import { useQueryClient } from '@tanstack/react-query';
import { useState, type FormEvent } from 'react';
import { toConsoleError } from '@shared/api/error';
import { textField } from '@shared/forms/textField';
import { useAuth, useCapability, useStepUp } from '@features/auth';
import {
  Button,
  ConfirmationDialog,
  InlineMessage,
  Panel,
  TextField,
  confirmationFor,
} from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { t } from '@shared/text';
import { accountInvalidation, changePassword } from './api';

/**
 * Password change (WCX-09 sections 8.7, 9.4, and 9.5.6).
 *
 * Level 3, so step-up reauthentication runs before the confirmation. That is
 * not redundant with the current-password field: the field proves the current
 * password, and the step-up proves possession of the MFA factor too. A
 * password change on a session someone else is holding is exactly the case
 * both are for.
 *
 * **What is reported is only what the response confirmed** (section 8.7). The
 * Control Plane revokes every session for this owner and issues a new one, so
 * the response carries fresh credentials — which the console installs, because
 * the proof it held a moment ago is now invalid. What the response does *not*
 * carry is a list of what it revoked, so the message says the session list
 * below now shows the outcome rather than narrating a count nobody sent.
 */
export function PasswordPanel() {
  const auth = useAuth();
  const client = useQueryClient();
  const stepUp = useStepUp();
  const capability = useCapability(confirmationFor('account.password').capability);
  const [confirming, setConfirming] = useState<{ current: string; next: string } | null>(null);
  const [busy, setBusy] = useState(false);
  const [failed, setFailed] = useState('');
  const [changed, setChanged] = useState(false);

  const username = auth.session?.username ?? '';
  const blocked = capability.allowed ? undefined : t('account.password.reauthenticate');

  function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    setFailed('');
    setChanged(false);
    // Held in component state only for the moment between the form and the
    // confirmation, and cleared on every exit below.
    setConfirming({
      current: textField(form, 'current_password'),
      next: textField(form, 'new_password'),
    });
  }

  async function apply(input: { current: string; next: string }) {
    setConfirming(null);
    if (!auth.csrf) {
      setFailed(t('account.password.reauthenticate'));
      return;
    }
    setBusy(true);
    try {
      const credentials = await changePassword(
        { current_password: input.current, new_password: input.next },
        auth.csrf,
      );
      // The old proof died with the old session. Install the new one before
      // anything else tries to use it.
      auth.adopt(credentials);
      await accountInvalidation.afterSessionChange(client);
      setChanged(true);
    } catch (caught) {
      const error = toConsoleError(caught);
      setFailed(
        error.kind === 'validation'
          ? t('account.password.rejected')
          : error.kind === 'rate-limited'
            ? t('auth.rateLimited')
            : t('account.password.failed'),
      );
    } finally {
      setBusy(false);
    }
  }

  return (
    <Panel
      heading={t('account.password.heading')}
      headingLevel={2}
      eyebrow={t('account.password.eyebrow')}
    >
      <p>{t('account.password.intro')}</p>
      <form className={styles.form} onSubmit={submit}>
        {/*
          A hidden username field is what lets a password manager attach the
          new password to the right account. It is the operator's own name
          from the session, never typed here.
        */}
        <input type="hidden" name="username" autoComplete="username" value={username} readOnly />
        <TextField
          name="current_password"
          label={t('account.password.current')}
          type="password"
          autoComplete="current-password"
          required
          maxLength={1024}
          {...(blocked === undefined ? {} : { disabledReason: blocked })}
        />
        <TextField
          name="new_password"
          label={t('account.password.next')}
          type="password"
          autoComplete="new-password"
          required
          minLength={12}
          maxLength={1024}
          description={t('account.password.policy')}
          {...(blocked === undefined ? {} : { disabledReason: blocked })}
        />
        <Button
          variant="primary"
          type="submit"
          pending={busy}
          {...(blocked === undefined ? {} : { disabledReason: blocked })}
        >
          {t(confirmationFor('account.password').effect)}
        </Button>
      </form>

      {changed && <InlineMessage tone="success">{t('account.password.changed')}</InlineMessage>}
      {failed !== '' && <InlineMessage tone="error">{failed}</InlineMessage>}

      {confirming !== null && (
        <ConfirmationDialog
          action="account.password"
          // The object is the account, named by the operator's own username.
          objectName={username}
          open
          stepUp={stepUp}
          onCancel={() => { setConfirming(null); }}
          onConfirm={() => { void apply(confirming); }}
        />
      )}
      {stepUp.element}
    </Panel>
  );
}
