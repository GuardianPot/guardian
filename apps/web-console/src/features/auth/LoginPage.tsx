import { useState, type FormEvent } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { toConsoleError } from '@shared/api/error';
import { textField } from '@shared/forms/textField';
import { useAuth } from './AuthContext';
import { useCapability } from './useCapability';
import { Button, InlineMessage, LoadingState, TextField } from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { LOGIN_TEXT as TEXT } from './text';

/** Stable so the three inputs can point at the one message that covers them. */
const LOGIN_ERROR_ID = 'login-error';

export function LoginPage() {
  const auth = useAuth();
  const navigate = useNavigate();
  const [method, setMethod] = useState<'totp' | 'recovery'>('totp');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const writeAccess = useCapability('environment.create');
  if (auth.loading) return <div className={styles.centered}><LoadingState activity={TEXT.checkingSession} /></div>;
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
        ...(method === 'totp' ? { totp_code: proof } : { recovery_code: proof }),
      });
      event.currentTarget.reset();
      void navigate('/environments', { replace: true });
    } catch (caught) {
      setError(toConsoleError(caught).kind === 'rate-limited' ? TEXT.rateLimited : TEXT.denied);
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className={styles.loginPage}>
      <section className={styles.loginIntro}>
        <div className={styles.brandMark} aria-hidden="true">G</div>
        <p className={styles.eyebrow}>{TEXT.brandEyebrow}</p>
        <h1>{TEXT.headlineFirst}<br />{TEXT.headlineSecond}</h1>
        <p>{TEXT.intro}</p>
      </section>
      <section className={styles.loginCard} aria-labelledby="login-heading">
        <p className={styles.eyebrow}>{auth.session ? TEXT.restoreEyebrow : TEXT.ownerEyebrow}</p>
        <h2 id="login-heading">{auth.session ? TEXT.reauthenticateHeading : TEXT.signInHeading}</h2>
        <p>{auth.session ? TEXT.reauthenticateIntro : TEXT.signInIntro}</p>
        <div className={styles.segmented} role="group" aria-label={TEXT.methodGroup}>
          <button type="button" aria-pressed={method === 'totp'} onClick={() => setMethod('totp')}>{TEXT.authenticator}</button>
          <button type="button" aria-pressed={method === 'recovery'} onClick={() => setMethod('recovery')}>{TEXT.recovery}</button>
        </div>
        <form onSubmit={(event) => { void submit(event); }} className={styles.form}>
          <TextField
            name="username"
            label={TEXT.username}
            autoComplete="username"
            required
            minLength={3}
            maxLength={64}
            {...(error ? { invalidatedBy: LOGIN_ERROR_ID } : {})}
          />
          <TextField
            name="password"
            label={TEXT.password}
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
            label={method === 'totp' ? TEXT.totpLabel : TEXT.recoveryLabel}
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
          <Button variant="primary" type="submit" pending={submitting}>{TEXT.submit}</Button>
        </form>
      </section>
    </main>
  );
}
