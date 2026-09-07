import { useId, type ReactNode } from 'react';
import styles from '@shared/styles/app.module.css';

/**
 * A titled region (WCX-04 sections 9.5 and 9.7.3).
 *
 * The heading level is an explicit prop rather than a fixed `h2`, so a screen
 * that nests regions keeps a valid heading order instead of skipping a level.
 * The region is labelled by its own heading, so assistive technology can list
 * the page's regions without the screen wiring `aria-labelledby` by hand.
 */
export type PanelProps = {
  heading: string;
  headingLevel: 2 | 3 | 4;
  /** Category above the heading, such as `Inventory truth`. */
  eyebrow?: string;
  /** A count or a status indicator rendered opposite the heading. */
  aside?: ReactNode;
  children: ReactNode;
};

export function Panel({ heading, headingLevel, eyebrow, aside, children }: PanelProps) {
  const headingId = useId();
  const Heading = `h${headingLevel}` as const;
  return (
    <section className={styles.panel} aria-labelledby={headingId}>
      <div className={styles.panelHeading}>
        <div>
          {eyebrow !== undefined && <p className={styles.eyebrow}>{eyebrow}</p>}
          <Heading id={headingId}>{heading}</Heading>
        </div>
        {aside}
      </div>
      {children}
    </section>
  );
}
