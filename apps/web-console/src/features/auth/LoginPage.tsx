import * as Label from '@radix-ui/react-label';
import { useState, type FormEvent } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';
import { toConsoleError } from '@shared/api/error';
import { textField } from '@shared/forms/textField';
import { useAuth } from './AuthContext';
import { useCapability } from './useCapability';
import { LoadingState } from '@shared/ui/Status';
import styles from '@shared/styles/app.module.css';

export function LoginPage() {
  const auth = useAuth();
  const navigate = useNavigate();
  const [method, setMethod] = useState<'totp' | 'recovery'>('totp');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const writeAccess = useCapability('environment.create');
  if (auth.loading) return <div className={styles.centered}><LoadingState label="Checking your session…" /></div>;
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
      setError(toConsoleError(caught).kind === 'rate-limited' ? 'Too many attempts. Wait before trying again.' : 'Sign-in was denied. Check your credentials and MFA proof.');
    } finally {
      setSubmitting(false);
    }
  }

  return (
    <main className={styles.loginPage}>
      <section className={styles.loginIntro}>
        <div className={styles.brandMark} aria-hidden="true">G</div>
        <p className={styles.eyebrow}>Guardian Control Plane</p>
        <h1>Know what is protected.<br />Know what needs attention.</h1>
        <p>Device inventory and health remain separate, explicit signals—never an inferred green light.</p>
      </section>
      <section className={styles.loginCard} aria-labelledby="login-heading">
        <p className={styles.eyebrow}>{auth.session ? 'Restore mutation access' : 'Owner access'}</p>
        <h2 id="login-heading">{auth.session ? 'Re-authenticate' : 'Sign in'}</h2>
        <p>{auth.session ? 'Your cookie session is active, but its CSRF proof was intentionally not persisted.' : 'Use your local owner credentials and one MFA method.'}</p>
        <div className={styles.segmented} role="group" aria-label="MFA method">
          <button type="button" aria-pressed={method === 'totp'} onClick={() => setMethod('totp')}>Authenticator</button>
          <button type="button" aria-pressed={method === 'recovery'} onClick={() => setMethod('recovery')}>Recovery code</button>
        </div>
        <form onSubmit={(event) => { void submit(event); }} className={styles.form}>
          <label>Username<input name="username" autoComplete="username" required minLength={3} maxLength={64} aria-invalid={Boolean(error)} aria-describedby={error ? 'login-error' : undefined} /></label>
          <label>Password<input name="password" type="password" autoComplete="current-password" required maxLength={1024} aria-invalid={Boolean(error)} aria-describedby={error ? 'login-error' : undefined} /></label>
          <Label.Root htmlFor="proof">{method === 'totp' ? '6-digit authenticator code' : 'Recovery code'}</Label.Root>
          <input id="proof" name="proof" autoComplete="one-time-code" required pattern={method === 'totp' ? '[0-9]{6}' : '[A-Za-z0-9_-]{22}'} aria-invalid={Boolean(error)} aria-describedby={error ? 'login-error' : undefined} />
          {error && <p id="login-error" className={styles.formError} role="alert">{error}</p>}
          <button className={styles.primaryButton} disabled={submitting}>{submitting ? 'Signing in…' : 'Continue securely'}</button>
        </form>
      </section>
    </main>
  );
}
