import { useRef, useState, type KeyboardEvent } from 'react';
import styles from '@shared/styles/app.module.css';
import { t } from '@shared/text';

/**
 * The MFA method chooser, shared by sign-in and step-up (WCX-09 section 9.2.2).
 *
 * Extracted from `LoginPage` when step-up needed the same control. Sharing it
 * is not only about duplication: the two screens must offer the *same* proofs.
 * A step-up that quietly accepted only TOTP would lock out an operator who is
 * holding recovery codes precisely because they lost the authenticator.
 *
 * A group of toggle buttons rather than a radiogroup — the browser suite
 * drives it by button role — with the arrow-key movement an operator expects
 * from a segmented control (`WCX-05` section 9.5.3). Each option exposes its
 * state through `aria-pressed`.
 *
 * The hook returns only the selection. The focus refs stay inside the
 * component: they exist to move focus between two buttons this file owns, and
 * a caller that could reach them would be reaching into the control.
 */
export const METHODS = ['totp', 'recovery'] as const;
export type MfaMethod = (typeof METHODS)[number];

export type MfaMethodState = {
  value: MfaMethod;
  set: (method: MfaMethod) => void;
};

export function useMfaMethod(): MfaMethodState {
  const [value, set] = useState<MfaMethod>('totp');
  return { value, set };
}

/** Arrow, Home, and End behaviour for the segmented control. */
function methodForKey(key: string, current: MfaMethod): MfaMethod | undefined {
  const index = METHODS.indexOf(current);
  if (key === 'ArrowLeft' || key === 'ArrowUp') return METHODS[(index + METHODS.length - 1) % METHODS.length];
  if (key === 'ArrowRight' || key === 'ArrowDown') return METHODS[(index + 1) % METHODS.length];
  if (key === 'Home') return METHODS[0];
  if (key === 'End') return METHODS[METHODS.length - 1];
  return undefined;
}

export function MfaMethodField({ method }: { method: MfaMethodState }) {
  const totpRef = useRef<HTMLButtonElement>(null);
  const recoveryRef = useRef<HTMLButtonElement>(null);
  const selected = method.value;

  function onKeyDown(event: KeyboardEvent<HTMLButtonElement>) {
    const next = methodForKey(event.key, selected);
    if (next === undefined) return;
    event.preventDefault();
    method.set(next);
    (next === 'totp' ? totpRef : recoveryRef).current?.focus();
  }

  return (
    <div className={styles.segmented} role="group" aria-label={t('auth.methodGroup')}>
      {/*
        The key handler sits on each option rather than on the group. Focus is
        always on an option when an arrow is pressed, so the behaviour is
        identical, and a `group` is not an interactive element that should be
        carrying keyboard listeners.
      */}
      <button
        ref={totpRef}
        type="button"
        aria-pressed={selected === 'totp'}
        onClick={() => { method.set('totp'); }}
        onKeyDown={onKeyDown}
      >
        {t('auth.methodAuthenticator')}
      </button>
      <button
        ref={recoveryRef}
        type="button"
        aria-pressed={selected === 'recovery'}
        onClick={() => { method.set('recovery'); }}
        onKeyDown={onKeyDown}
      >
        {t('auth.methodRecovery')}
      </button>
    </div>
  );
}
