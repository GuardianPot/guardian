import { Dialog as RadixDialog } from 'radix-ui';
import { useRef, type ReactNode, type RefObject } from 'react';
import styles from '@shared/styles/app.module.css';

/**
 * The single modal (WCX-04 sections 9.5 and 9.7.2).
 *
 * Radix supplies the focus trap and the escape handler. What this wrapper adds
 * is what a call site kept getting wrong:
 *
 * - a name and a description are required props, so a dialog cannot ship
 *   unlabelled;
 * - `initialFocus` is explicit. Radix would otherwise focus the first tabbable
 *   element, which for a confirmation is whichever control happens to come
 *   first in the DOM. `WCX-04` section 9.3 requires focus on cancel, so the
 *   caller names the control rather than relying on source order;
 * - focus return is handled here. Radix returns focus to its own
 *   `Dialog.Trigger`, and every dialog in this console is opened from state —
 *   a one-time secret appears when a mutation resolves, not when a button is
 *   pressed — so there is no trigger to return to. The invoking control is
 *   captured on open instead, while focus is still on it.
 *
 * There is no `className` prop and no `open`-less mode: a dialog is always a
 * modal, and a screen cannot restyle it into something that is not one.
 */
export type DialogProps = {
  open: boolean;
  onClose: () => void;
  title: string;
  description: string;
  /** Receives focus when the dialog opens. */
  initialFocus?: RefObject<HTMLElement | null>;
  children?: ReactNode;
  /** The action row. Render cancel first so the tab order matches the DOM. */
  actions: ReactNode;
};

export function Dialog({
  open,
  onClose,
  title,
  description,
  initialFocus,
  children,
  actions,
}: DialogProps) {
  const invoker = useRef<HTMLElement | null>(null);
  return (
    <RadixDialog.Root open={open} onOpenChange={(next) => { if (!next) onClose(); }}>
      <RadixDialog.Portal>
        <RadixDialog.Overlay className={styles.dialogOverlay} />
        <RadixDialog.Content
          className={styles.dialog}
          onEscapeKeyDown={onClose}
          onOpenAutoFocus={(event) => {
            // Radix dispatches this before it moves focus, so the active
            // element here is still the control the operator was on.
            invoker.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
            if (!initialFocus?.current) return;
            event.preventDefault();
            initialFocus.current.focus();
          }}
          onCloseAutoFocus={(event) => {
            event.preventDefault();
            if (invoker.current?.isConnected) invoker.current.focus();
          }}
        >
          <RadixDialog.Title>{title}</RadixDialog.Title>
          <RadixDialog.Description>{description}</RadixDialog.Description>
          {children}
          <div className={styles.dialogActions}>{actions}</div>
        </RadixDialog.Content>
      </RadixDialog.Portal>
    </RadixDialog.Root>
  );
}
