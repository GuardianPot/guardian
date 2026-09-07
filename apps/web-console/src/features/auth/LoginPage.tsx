import { useState, type FormEvent } from 'react';
import { Navigate, useNavigate } from 'react-router';
import { toConsoleError } from '@shared/api/error';
import { textField } from '@shared/forms/textField';
import { useAuth } from './AuthContext';
import { useCapability } from './useCapability';
import { MfaMethodField, useMfaMethod } from './MfaMethodField';
import { Button, InlineMessage, LoadingState, TextField } from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { t } from '@shared/text';

/** Stable so the three inputs can point at the one message that covers them. */
const LOGIN_ERROR_ID = 'login-error';

export function LoginPage() {
  const auth = useAuth();
  const navigate = useNavigate();
  // Shared with step-up since `WCX-09`: both screens must offer the same
  // proofs, so both read the same control.
  const method = useMfaMethod();
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const writeAccess = useCapability('environment.create');
  if (auth.loading) return <div className={styles.centered}><LoadingState activity={t('common.checkingSession')} /></div>;
  if (auth.session && writeAccess.allowed) return <Navigate to="/environments" replace />;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setError('');
    setSubmitting(true);
    const form = new FormData(event.currentTarget);
    const proof = textField(form, 'proof');
    try {
      await auth.login({
        username: textField(form, 'username'),
        password: textField(form, 'password'),
        ...(method.value === 'totp' ? { totp_code: proof } : { recovery_code: proof }),
      });
      event.currentTarget.reset();
      void navigate('/environments', { replace: true });
    } catch (caught) {
      setError(toConsoleError(caught).kind === 'rate-limited' ? t('auth.rateLimited') : t('auth.denied'));
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className={styles.loginPage} tabIndex={-1}>
      <section className={styles.loginIntro}>
        <div className={styles.brandMark} aria-hidden="true">{t('common.brandMark')}</div>
        <p className={styles.eyebrow}>{t('auth.brand')}</p>
        {/*
          A paragraph, not the page heading: this column is hidden below 900
          pixels, and the screen's heading has to exist at every width. See
          `.loginHeadline` in `app.module.css`.
        */}
        <p className={styles.loginHeadline}>{t('auth.headlineFirst')}<br />{t('auth.headlineSecond')}</p>
        <p>{t('auth.intro')}</p>
      </section>
      <section className={styles.loginCard} aria-labelledby="login-heading">
        <p className={styles.eyebrow}>{t(auth.session ? 'auth.restoreEyebrow' : 'auth.ownerEyebrow')}</p>
        <h1 id="login-heading" tabIndex={-1}>{t(auth.session ? 'auth.reauthenticateHeading' : 'auth.signInHeading')}</h1>
        <p>{t(auth.session ? 'auth.reauthenticateIntro' : 'auth.signInIntro')}</p>
        <MfaMethodField method={method} />
        <form onSubmit={(event) => { void submit(event); }} className={styles.form}>
          <TextField
            name="username"
            label={t('auth.username')}
            autoComplete="username"
            required
            minLength={3}
            maxLength={64}
            {...(error ? { invalidatedBy: LOGIN_ERROR_ID } : {})}
          />
          <TextField
            name="password"
            label={t('auth.password')}
            type="password"
            autoComplete="current-password"
            required
            maxLength={1024}
            {...(error ? { invalidatedBy: LOGIN_ERROR_ID } : {})}
          />
          {/*
            The proof field is remounted when the method changes, so the
            authenticator code an operator typed never travels into a recovery
            code submission. `key` is what forces that.
          */}
          <TextField
            key={method.value}
            name="proof"
            label={t(method.value === 'totp' ? 'auth.totpLabel' : 'auth.recoveryLabel')}
            autoComplete="one-time-code"
            required
            pattern={method.value === 'totp' ? '[0-9]{6}' : '[A-Za-z0-9_-]{22}'}
            {...(error ? { invalidatedBy: LOGIN_ERROR_ID } : {})}
          />
          {/*
            One form-scoped message rather than three field-scoped ones: the
            backend deliberately does not say which credential was wrong, so
            three copies would announce the same non-answer three times.
          */}
          {error && <InlineMessage tone="error" id={LOGIN_ERROR_ID}>{error}</InlineMessage>}
          <Button variant="primary" type="submit" pending={submitting}>{t('auth.submit')}</Button>
        </form>
      </section>
    </main>
  );
}
