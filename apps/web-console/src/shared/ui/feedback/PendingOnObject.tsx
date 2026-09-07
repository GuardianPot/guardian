import styles from '@shared/styles/app.module.css';
import { formatAge } from '@shared/ui/state/freshness';
import { PENDING_TEXT } from './text';

/**
 * The pending-on-object pattern (WCX-04 section 9.4).
 *
 * Long-running work is shown on the object it affects — the row or the panel
 * for that device, zone, or environment — carrying the treatment, the age of
 * the request, and the reason. No blocking progress modal is permitted, and
 * this component deliberately offers no way to build one: it renders inline
 * and traps nothing.
 *
 * The age is derived on render from a timestamp the caller holds. Nothing here
 * subscribes to an interval (section 9.11); the value refreshes when the
 * object's own query does.
 */
export function PendingOnObject({
  startedAt,
  reason,
  now,
}: {
  startedAt: string;
  reason: string;
  now?: number;
}) {
  return (
    <p className={styles.pendingOnObject} role="status">
      <span className={styles.pendingLabel}>{PENDING_TEXT.label}</span>
      <span>{PENDING_TEXT.detail(formatAge(startedAt, now), reason)}</span>
    </p>
  );
}
