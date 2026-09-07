import { useRef, useState, type FormEvent, type KeyboardEvent } from 'react';
import { Navigate, useNavigate } from 'react-router';
import { toConsoleError } from '@shared/api/error';
import { textField } from '@shared/forms/textField';
import { useAuth } from './AuthContext';
import { useCapability } from './useCapability';
import { Button, InlineMessage, LoadingState, TextField } from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { t } from '@shared/text';

/** Stable so the three inputs can point at the one message that covers them. */
const LOGIN_ERROR_ID = 'login-error';

/** The MFA methods, in the order the segmented control presents them. */
const METHODS = ['totp', 'recovery'] as const;
type Method = (typeof METHODS)[number];

/** Arrow, Home, and End behaviour for the segmented control (section 9.5.3). */
function methodForKey(key: string, current: Method): Method | undefined {
  const index = METHODS.indexOf(current);
  if (key === 'ArrowLeft' || key === 'ArrowUp') return METHODS[(index + METHODS.length - 1) % METHODS.length];
  if (key === 'ArrowRight' || key === 'ArrowDown') return METHODS[(index + 1) % METHODS.length];
  if (key === 'Home') return METHODS[0];
  if (key === 'End') return METHODS[METHODS.length - 1];
  return undefined;
}

export function LoginPage() {
  const auth = useAuth();
  const navigate = useNavigate();
  const [method, setMethod] = useState<Method>('totp');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const totpRef = useRef<HTMLButtonElement>(null);
  const recoveryRef = useRef<HTMLButtonElement>(null);
  const writeAccess = useCapability('environment.create');
  if (auth.loading) return <div className={styles.centered}><LoadingState activity={t('common.checkingSession')} /></div>;
  if (auth.session && writeAccess.allowed) return <Navigate to="/environments" replace />;

  function onMethodKeyDown(event: KeyboardEvent<HTMLButtonElement>) {
    const next = methodForKey(event.key, method);
    if (next === undefined) return;
    event.preventDefault();
    setMethod(next);
    (next === 'totp' ? totpRef : recoveryRef).current?.focus();
  }

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
        ...(method === 'totp' ? { totp_code: proof } : { recovery_code: proof }),
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
        {/*
          A two-option segmented control (WCX-05 section 9.5.3). It stays a
          group of toggle buttons rather than becoming a radiogroup — the
          browser suite drives it by button role — and gains the arrow-key
          movement an operator expects from a segmented control. Each option
          exposes its state through `aria-pressed`.
        */}
        <div className={styles.segmented} role="group" aria-label={t('auth.methodGroup')}>
          {/*
            The key handler sits on each option rather than on the group.
            Focus is always on an option when an arrow is pressed, so the
            behaviour is identical, and a `group` is not an interactive
            element that should be carrying keyboard listeners.
          */}
          <button ref={totpRef} type="button" aria-pressed={method === 'totp'} onClick={() => setMethod('totp')} onKeyDown={onMethodKeyDown}>{t('auth.methodAuthenticator')}</button>
          <button ref={recoveryRef} type="button" aria-pressed={method === 'recovery'} onClick={() => setMethod('recovery')} onKeyDown={onMethodKeyDown}>{t('auth.methodRecovery')}</button>
        </div>
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
            key={method}
            name="proof"
            label={t(method === 'totp' ? 'auth.totpLabel' : 'auth.recoveryLabel')}
            autoComplete="one-time-code"
            required
            pattern={method === 'totp' ? '[0-9]{6}' : '[A-Za-z0-9_-]{22}'}
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
