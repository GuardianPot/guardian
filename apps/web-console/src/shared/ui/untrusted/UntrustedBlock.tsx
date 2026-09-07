import { useId, useMemo, useState } from 'react';
import { reveal, type Untrusted } from '@shared/api/untrusted';
import { Button } from '@shared/ui/controls/Button';
import { InlineMessage } from '@shared/ui/feedback/InlineMessage';
import styles from '@shared/styles/app.module.css';
import { BLOCK_LIMIT, transformUntrusted } from './transform';
import { renderSegment } from './UntrustedText';
import { t } from '@shared/text';

/**
 * A multi-line captured payload (WCX-06 section 9.1).
 *
 * HTTP bodies, terminal transcripts, anything an attacker typed. Newline and
 * tab survive because they are structure an operator reads; every other
 * control character is escaped, so an ANSI sequence appears as its source
 * instead of repainting the block.
 *
 * The block is a named, keyboard-reachable scroll container (section 9.7.3):
 * a region that scrolls but cannot be focused is unreachable for anyone not
 * using a pointer.
 *
 * Copy hands over the *original* value, not the escaped rendering. An operator
 * pasting into an analysis tool needs the bytes that arrived. The control says
 * so in its name, so nobody copies attacker input believing it is sanitised.
 */
export function UntrustedBlock({ value, label }: { value: Untrusted; label?: string }) {
  const [copyState, setCopyState] = useState<'idle' | 'copied' | 'failed'>('idle');
  const legendId = useId();
  const transformed = useMemo(
    () => transformUntrusted(reveal(value), { limit: BLOCK_LIMIT, allowLineBreaks: true }),
    [value],
  );

  async function copy(): Promise<void> {
    try {
      // Reading the original is the one legitimate use of `reveal` outside the
      // renderer, and it never passes through the DOM on the way out.
      await navigator.clipboard.writeText(reveal(value));
      setCopyState('copied');
    } catch {
      // Section 9.9.3: a failed write says so. Silence would look like success.
      setCopyState('failed');
    }
  }

  return (
    <div className={styles.untrustedBlock}>
      {transformed.escaped && (
        <p className={styles.untrustedLegend} id={legendId}>{t('untrusted.legend')}</p>
      )}
      {/* eslint-disable jsx-a11y/no-noninteractive-tabindex -- a scrollable region must be focusable or a keyboard user cannot scroll it (WCAG 2.1.1, WCX-06 section 9.7.3); the rule sees a non-interactive role and cannot see that this container overflows */}
      <pre
        className={styles.untrustedPayload}
        tabIndex={0}
        role="group"
        aria-label={label ?? t('untrusted.blockLabel')}
        {...(transformed.escaped ? { 'aria-describedby': legendId } : {})}
      >
        {transformed.segments.map(renderSegment)}
      </pre>
      {/* eslint-enable jsx-a11y/no-noninteractive-tabindex */}
      {transformed.truncated && (
        <p className={styles.untrustedTruncation}>
          {t('untrusted.truncatedBlock', { shown: BLOCK_LIMIT, original: transformed.originalLength })}
        </p>
      )}
      <Button variant="quiet" onClick={() => { void copy(); }}>{t('untrusted.copy')}</Button>
      {copyState === 'copied' && <InlineMessage tone="success">{t('untrusted.copied')}</InlineMessage>}
      {copyState === 'failed' && <InlineMessage tone="error">{t('untrusted.copyFailed')}</InlineMessage>}
    </div>
  );
}
