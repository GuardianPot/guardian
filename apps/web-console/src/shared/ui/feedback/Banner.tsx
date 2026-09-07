import type { ReactNode } from 'react';
import styles from '@shared/styles/app.module.css';

/**
 * Page or scope-level persistent condition (WCX-04 section 9.4).
 *
 * A banner stays until the condition clears — a read-only session, an impaired
 * dependency, stale data. It is one of the two surfaces that may carry an
 * error, because unlike a toast it cannot expire unseen.
 *
 * `tone` fixes both the treatment and the announcement, and a caller cannot
 * pass a role or a class:
 *
 * - `informational` — a condition about the data. `status`.
 * - `blocking` — a condition that stops the operator acting. `alert`.
 * - `restricted` — a condition about the *session* rather than the data, such
 *   as a reload-restored read-only session. It spans the workspace above the
 *   content because it applies to every screen, and it is announced as
 *   `status` because nothing has failed.
 */
export type BannerTone = 'informational' | 'blocking' | 'restricted';

const TONE_CLASS: Readonly<Record<BannerTone, string>> = {
  informational: 'banner',
  blocking: 'bannerBlocking',
  restricted: 'reauthBanner',
};

export function Banner({ tone, children }: { tone: BannerTone; children: ReactNode }) {
  return (
    <div className={styles[TONE_CLASS[tone]]} role={tone === 'blocking' ? 'alert' : 'status'}>
      {children}
    </div>
  );
}
