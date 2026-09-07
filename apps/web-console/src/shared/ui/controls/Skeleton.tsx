import styles from '@shared/styles/app.module.css';

/**
 * A placeholder shape for a first load (WCX-04 section 9.5).
 *
 * Deliberately silent. `LoadingState` owns the `status` announcement, so a
 * skeleton beside it would announce the same activity twice. It is also
 * deliberately static: `WC-D13` forbids motion that encodes state, and a
 * shimmer that keeps running is indistinguishable from progress.
 */
export function Skeleton({ lines = 3 }: { lines?: number }) {
  return (
    <div className={styles.skeleton} role="presentation" aria-hidden="true">
      {Array.from({ length: lines }, (_, index) => (
        <span key={index} className={styles.skeletonLine} />
      ))}
    </div>
  );
}
