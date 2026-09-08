import { useEffect, useState } from 'react';
import { Navigate, useNavigate } from 'react-router';
import { toConsoleError } from '@shared/api/error';
import { maxLengthOf, schemaFor, useConsoleForm } from '@shared/forms';
import { useAuth } from './AuthContext';
import { useCapability } from './useCapability';
import { MfaMethodField, useMfaMethod } from './MfaMethodField';
import { Button, InlineMessage, LoadingState, TextField } from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { t } from '@shared/text';

/** Stable so the three inputs can point at the one message that covers them. */
const LOGIN_ERROR_ID = 'login-error';

/*
 * Sign-in bounds come from the contract (WCX-11 section 8.2).
 *
 * They were literals here — 3, 64, 1024 — copied from `AuthLoginRequest`. The
 * copy was correct and would have stayed correct only until the contract moved.
 */
const loginSchema = schemaFor<'AuthLoginRequest', { username: string; password: string; proof: string }>(
  'AuthLoginRequest',
  ['username', 'password'],
  // One control, either proof. The contract's `oneOf` says the request carries
  // a TOTP code or a recovery code, and the operator's method choice decides
  // which; both bounds come from the contract rather than from a literal here.
  { proof: ['totp_code', 'recovery_code'] },
);
const USERNAME_LIMIT = maxLengthOf('AuthLoginRequest', 'username') ?? 64;
const PASSWORD_LIMIT = maxLengthOf('AuthLoginRequest', 'password') ?? 1024;

export function LoginPage() {
  const auth = useAuth();
  const navigate = useNavigate();
  // Shared with step-up since `WCX-09`: both screens must offer the same
  // proofs, so both read the same control.
  const method = useMfaMethod();
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const writeAccess = useCapability('environment.create');
  const form = useConsoleForm<{ username: string; password: string; proof: string }>({
    schema: loginSchema,
    defaultValues: { username: '', password: '', proof: '' },
    onSubmit: async (values) => {
      setError('');
      setSubmitting(true);
      try {
        await auth.login({
          username: values.username,
          password: values.password,
          ...(method.value === 'totp'
            ? { totp_code: values.proof }
            : { recovery_code: values.proof }),
        });
        form.form.reset({ username: '', password: '', proof: '' });
        void navigate('/environments', { replace: true });
      } catch (caught) {
        setError(toConsoleError(caught).kind === 'rate-limited' ? t('auth.rateLimited') : t('auth.denied'));
      } finally {
        setSubmitting(false);
      }
    },
  });

  /*
   * Clearing the proof when the method changes is a security behaviour, not a
   * convenience, and moving to the form stack nearly lost it.
   *
   * The field is remounted on `key={method.value}` so an authenticator code
   * never travels into a recovery-code submission. React Hook Form keeps a
   * registered field's value across a remount, which would have quietly undone
   * that: the input would look empty and the form state would still hold the
   * code. This resets the value the stack holds, which is the one that is sent.
   */
  useEffect(() => {
    form.form.setValue('proof', '');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [method.value]);

  // After every hook, so the hook order is identical on the render that
  // redirects and the render that shows the form.
  if (auth.loading) return <div className={styles.centered}><LoadingState activity={t('common.checkingSession')} /></div>;
  if (auth.session && writeAccess.allowed) return <Navigate to="/environments" replace />;

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
        <form onSubmit={(event) => { void form.submit(event); }} className={styles.form}>
          <TextField
            name="username"
            label={t('auth.username')}
            autoComplete="username"
            required
            maxLength={USERNAME_LIMIT}
            registration={form.form.register('username')}
            {...(error ? { invalidatedBy: LOGIN_ERROR_ID } : {})}
          />
          <TextField
            name="password"
            label={t('auth.password')}
            type="password"
            autoComplete="current-password"
            required
            maxLength={PASSWORD_LIMIT}
            registration={form.form.register('password')}
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
            registration={form.form.register('proof')}
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
