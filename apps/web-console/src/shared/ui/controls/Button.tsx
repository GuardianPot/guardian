import { useId, type ReactNode, type RefObject } from 'react';
import styles from '@shared/styles/app.module.css';
import { BUTTON_TEXT } from './text';

/**
 * The single button (WCX-04 section 9.5).
 *
 * Two rules that a per-call-site button kept breaking:
 *
 * 1. A pending control keeps its accessible name. Swapping the label for
 *    "Creating…" renames the control mid-interaction, which breaks a screen
 *    reader's mental model and every name-based selector. The pending marker
 *    is a separate, `aria-hidden` element and `aria-busy` carries the state.
 * 2. A disabled control renders its reason and associates it with
 *    `aria-describedby`, so an operator learns *why* rather than finding a
 *    dead control (WCX-04 section 9.7.5, decision WC-D07). The control is
 *    disabled, never hidden.
 *
 * No `className` prop: a screen positions a button with its own container and
 * can never restyle one.
 */
export type ButtonVariant = 'primary' | 'secondary' | 'destructive' | 'quiet';

const VARIANT_CLASS: Readonly<Record<ButtonVariant, string>> = {
  primary: 'primaryButton',
  secondary: 'secondaryButton',
  destructive: 'destructiveButton',
  quiet: 'quietButton',
};

export type ButtonProps = {
  children: ReactNode;
  variant?: ButtonVariant;
  type?: 'button' | 'submit';
  pending?: boolean;
  /** Present only when the control is unavailable. Rendered and associated. */
  disabledReason?: string;
  onClick?: () => void;
  /**
   * Focus management only. A dialog names the control that should receive
   * focus on open; nothing else may reach into a button through this.
   */
  buttonRef?: RefObject<HTMLButtonElement | null>;
};

export function Button({
  children,
  variant = 'secondary',
  type = 'button',
  pending = false,
  disabledReason,
  onClick,
  buttonRef,
}: ButtonProps) {
  const reasonId = useId();
  const unavailable = disabledReason !== undefined;
  return (
    <span className={styles.control}>
      <button
        ref={buttonRef}
        className={styles[VARIANT_CLASS[variant]]}
        type={type}
        disabled={unavailable || pending}
        aria-busy={pending || undefined}
        aria-describedby={unavailable ? reasonId : undefined}
        {...(onClick ? { onClick } : {})}
      >
        {children}
        {pending && <span className={styles.buttonPending} aria-hidden="true">{BUTTON_TEXT.pending}</span>}
      </button>
      {unavailable && <span className={styles.disabledReason} id={reasonId}>{disabledReason}</span>}
    </span>
  );
}
