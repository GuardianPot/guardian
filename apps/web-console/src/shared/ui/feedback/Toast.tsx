import { useCallback, useEffect, useRef, useState } from 'react';
import styles from '@shared/styles/app.module.css';
import { TOAST_TEXT } from './text';

/**
 * Short confirmation of a completed action (WCX-04 section 9.4).
 *
 * Two structural guarantees, not conventions:
 *
 * 1. **A toast cannot carry an error.** There is no error tone and no way to
 *    add one at a call site. Section 8.5 forbids a toast being the only
 *    surface carrying an error, because a toast expires unseen; making the
 *    error tone unrepresentable is stronger than a rule reviewers must
 *    remember. Errors go to `InlineMessage` or `Banner`.
 * 2. **A toast never steals focus.** It is announced through `status` and
 *    reached by keyboard through the dismiss button in the tab order.
 *
 * The region is `pointer-events: none` and only the dismiss button takes
 * pointer events, so a toast can never intercept a click on the content it
 * floats over.
 *
 * The timer is per toast and is cleared on unmount. Section 9.11 forbids a
 * global interval, and nothing here subscribes to one.
 */
export const TOAST_MINIMUM_MS = 6_000;

export type ToastMessage = { id: string; text: string };

/** Screen-local toast state. Nothing is stored outside this component tree. */
export function useToasts() {
  const [toasts, setToasts] = useState<readonly ToastMessage[]>([]);
  const nextId = useRef(0);
  const dismiss = useCallback((id: string) => {
    setToasts((current) => current.filter((toast) => toast.id !== id));
  }, []);
  const show = useCallback((text: string) => {
    nextId.current += 1;
    const id = `toast-${nextId.current}`;
    setToasts((current) => [...current, { id, text }]);
  }, []);
  return { toasts, show, dismiss };
}

export function ToastRegion({
  toasts,
  onDismiss,
  lifetimeMs = TOAST_MINIMUM_MS,
}: {
  toasts: readonly ToastMessage[];
  onDismiss: (id: string) => void;
  lifetimeMs?: number;
}) {
  return (
    // Deliberately unlabelled and roleless: each toast is its own `status`
    // live region, and a named wrapper would add an empty landmark to every
    // screen for the seconds a toast is not showing.
    <div className={styles.toastRegion}>
      {toasts.map((toast) => (
        <Toast key={toast.id} toast={toast} onDismiss={onDismiss} lifetimeMs={lifetimeMs} />
      ))}
    </div>
  );
}

function Toast({
  toast,
  onDismiss,
  lifetimeMs,
}: {
  toast: ToastMessage;
  onDismiss: (id: string) => void;
  lifetimeMs: number;
}) {
  const [paused, setPaused] = useState(false);
  useEffect(() => {
    if (paused) return;
    const timer = setTimeout(() => onDismiss(toast.id), Math.max(lifetimeMs, TOAST_MINIMUM_MS));
    return () => clearTimeout(timer);
  }, [paused, lifetimeMs, onDismiss, toast.id]);

  return (
    <div
      className={styles.toast}
      role="status"
      onMouseEnter={() => setPaused(true)}
      onMouseLeave={() => setPaused(false)}
      onFocusCapture={() => setPaused(true)}
      onBlurCapture={() => setPaused(false)}
    >
      <span>{toast.text}</span>
      <button className={styles.toastDismiss} type="button" onClick={() => onDismiss(toast.id)}>
        {TOAST_TEXT.dismiss}
      </button>
    </div>
  );
}
