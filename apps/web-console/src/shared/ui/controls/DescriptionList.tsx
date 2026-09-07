import type { ReactNode } from 'react';
import styles from '@shared/styles/app.module.css';

/**
 * Name/value facts (WCX-04 section 9.5).
 *
 * Each fact is a real `dl` pair, so the association survives a screen reader
 * instead of relying on visual proximity. A value is always rendered: a fact
 * the backend did not supply is stated by the caller as a fact ("No active
 * certificate"), never omitted, because an omitted row reads as an answered
 * question.
 */
export type DescriptionEntry = { term: string; value: ReactNode };

export function DescriptionList({
  label,
  entries,
}: {
  label: string;
  entries: readonly DescriptionEntry[];
}) {
  return (
    <section className={styles.factGrid} aria-label={label}>
      {entries.map((entry) => (
        <dl key={entry.term}>
          <dt>{entry.term}</dt>
          <dd>{entry.value}</dd>
        </dl>
      ))}
    </section>
  );
}
