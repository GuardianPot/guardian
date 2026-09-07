import { useMemo } from 'react';
import { reveal, type Untrusted } from '@shared/api/untrusted';
import styles from '@shared/styles/app.module.css';
import { TEXT_LIMIT, transformUntrusted, type Segment } from './transform';
import { t } from '@shared/text';

/**
 * A short attacker-influenced value (WCX-06 section 9.1).
 *
 * Display names, reasons, usernames, filenames, source identifiers. Every
 * control character is escaped — including newline and tab, because a single
 * line that contains a newline is not a single line and would break the layout
 * it was placed in.
 *
 * The component emits a `span` and nothing else. There is no prop that could
 * make it an anchor, and no attribute here is derived from the value, so a
 * `javascript:` URL, a filename, and a payload that looks like markup all
 * render identically: as characters.
 */
export function UntrustedText({ value }: { value: Untrusted }) {
  const transformed = useMemo(
    () => transformUntrusted(reveal(value), { limit: TEXT_LIMIT, allowLineBreaks: false }),
    [value],
  );

  return (
    <span className={styles.untrusted}>
      {transformed.segments.map(renderSegment)}
      {transformed.truncated && (
        <span className={styles.untrustedTruncation}>
          {' '}
          {t('untrusted.truncatedText', { shown: TEXT_LIMIT, original: transformed.originalLength })}
        </span>
      )}
    </span>
  );
}

/**
 * One segment.
 *
 * An escape is `role="img"` with a name: it is a glyph standing in for a
 * character, and section 9.7.1 requires a screen-reader user to learn that a
 * control character was present rather than hearing the escape spelled out or
 * hearing nothing at all.
 */
export function renderSegment(segment: Segment, index: number) {
  if (segment.kind === 'text') return <span key={index}>{segment.value}</span>;
  return (
    <span
      key={index}
      className={styles.untrustedEscape}
      role="img"
      aria-label={t('untrusted.escapeLabel', { description: segment.description })}
    >
      {segment.value}
    </span>
  );
}
