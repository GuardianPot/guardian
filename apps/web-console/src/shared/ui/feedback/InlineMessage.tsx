import type { ReactNode } from 'react';
import styles from '@shared/styles/app.module.css';

/**
 * Field and form-scoped feedback (WCX-04 section 9.4).
 *
 * An error is an `alert`, so it is announced the moment a submission is
 * rejected. A success carries no role: it is already visible beside the
 * control the operator just used, and announcing it would interrupt them.
 *
 * Inline is one of the two surfaces that may carry an error. A toast may
 * accompany an error but may never replace it, because a toast expires.
 */
export type InlineTone = 'error' | 'success';

export function InlineMessage({ tone, id, children }: { tone: InlineTone; id?: string; children: ReactNode }) {
  return tone === 'error' ? (
    <p className={styles.formError} role="alert" {...(id === undefined ? {} : { id })}>{children}</p>
  ) : (
    <p className={styles.inlineSuccess} {...(id === undefined ? {} : { id })}>{children}</p>
  );
}
