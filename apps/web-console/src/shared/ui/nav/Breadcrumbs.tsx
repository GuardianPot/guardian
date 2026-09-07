import { Link } from 'react-router';
import type { ReactNode } from 'react';
import styles from '@shared/styles/app.module.css';
import { t } from '@shared/text';

/**
 * The path back out of a nested screen (WCX-10 section 9.4.2).
 *
 * Replaces the single "← All environments" link each nested screen carried.
 * One link is enough to go up one level and says nothing about where the
 * operator is; on the device screen it left the environment unnamed, so an
 * operator two levels deep had no way to read their position off the page.
 *
 * The last entry is the current screen and is not a link — a link to where you
 * already are is a control that does nothing. Section 8.4: a backend display
 * name may appear here, and it arrives as a node the caller has already routed
 * through the untrusted components, never as a string this file interpolates.
 */
export type Crumb = {
  label: ReactNode;
  /** Absent on the last entry, which is the current screen. */
  to?: string;
};

export function Breadcrumbs({ trail }: { trail: readonly Crumb[] }) {
  return (
    <nav className={styles.breadcrumbs} aria-label={t('common.breadcrumbs')}>
      <ol>
        {trail.map((crumb, index) => {
          const last = index === trail.length - 1;
          return (
            <li key={index}>
              {crumb.to !== undefined && !last
                ? <Link to={crumb.to}>{crumb.label}</Link>
                : <span aria-current="page">{crumb.label}</span>}
            </li>
          );
        })}
      </ol>
    </nav>
  );
}
