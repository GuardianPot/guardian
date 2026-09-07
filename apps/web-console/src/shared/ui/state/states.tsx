import type { ReactNode } from 'react';
import styles from '@shared/styles/app.module.css';
import { Button } from '@shared/ui/controls/Button';
import { Skeleton } from '@shared/ui/controls/Skeleton';
import { formatAge } from './freshness';
import {
  DEGRADED_TEXT,
  DENIED_TEXT,
  EMPTY_TEXT,
  ERROR_TEXT,
  LOADING_TEXT,
  PARTIAL_TEXT,
  STALE_TEXT,
  UNKNOWN_TEXT,
} from './text';

/**
 * The eight canonical data states (WCX-04 section 9.1, decision WC-D15).
 *
 * Each state is its own component with its own required content, so a screen
 * cannot collapse two of them into one message. Three distinctions carry the
 * security weight and are asserted per state:
 *
 * - `denied` is not `empty`. An authorization refusal rendered as an empty
 *   list tells an operator that nothing exists when access was refused.
 * - `unknown` is neither `empty` nor a failing state. No observation is not a
 *   negative observation, and it is not a healthy one either.
 * - none of the eight reads as healthy, complete, or successful.
 *
 * Announcement follows section 9.7.1: informational states use `status`,
 * blocking states use `alert`.
 */
function Informational({ children }: { children: ReactNode }) {
  return <div className={styles.stateBlock} role="status">{children}</div>;
}

function Blocking({ children }: { children: ReactNode }) {
  return <div className={`${styles.stateBlock} ${styles.stateBlocking}`} role="alert">{children}</div>;
}

/** First load in flight. Announces the activity, never a bare spinner. */
export function LoadingState({ activity, skeletonLines }: { activity: string; skeletonLines?: number }) {
  return (
    <div className={styles.stateBlock}>
      <p className={styles.state} role="status">{LOADING_TEXT.announce(activity)}</p>
      {skeletonLines !== undefined && <Skeleton lines={skeletonLines} />}
    </div>
  );
}

/**
 * The backend confirmed zero items.
 *
 * `action` is the control that creates the first item. It is supplied only
 * when the operator holds the capability; `WC-D07` keeps a control the
 * operator lacks visible-but-disabled at its own call site rather than here.
 */
export function EmptyState({ collection, action }: { collection: string; action?: ReactNode }) {
  return (
    <Informational>
      <p className={styles.stateHeading}>{EMPTY_TEXT.heading(collection)}</p>
      <p className={styles.stateDetail}>{EMPTY_TEXT.body(collection)}</p>
      {action}
    </Informational>
  );
}

/** No observation exists. Absence is neither a healthy nor a failing signal. */
export function UnknownState({ subject, observationSource }: { subject: string; observationSource?: string }) {
  return (
    <Informational>
      <p className={styles.stateHeading}>{UNKNOWN_TEXT.heading}</p>
      <p className={styles.stateDetail}>{UNKNOWN_TEXT.body(subject)}</p>
      {observationSource !== undefined && (
        <p className={styles.stateDetail}>{UNKNOWN_TEXT.source(observationSource)}</p>
      )}
    </Informational>
  );
}

/**
 * Last successful data, with its age and why the refresh is not current.
 *
 * `observedAt` is optional only so that an age is never invented. A caller
 * that cannot say when the data was observed says nothing about its age rather
 * than printing a fabricated one; `DataBoundary` falls back to the moment the
 * query cache last accepted the data, so in practice the age is always there.
 */
export function StaleState({
  observedAt,
  reason,
  now,
  children,
}: {
  observedAt?: string;
  reason: string;
  now?: number;
  children: ReactNode;
}) {
  return (
    <div className={styles.stateWrapper}>
      <div className={styles.stateBlock} role="status">
        <p className={styles.stateHeading}>{STALE_TEXT.heading}</p>
        {observedAt !== undefined && (
          <p className={styles.stateDetail}>{STALE_TEXT.age(formatAge(observedAt, now))}</p>
        )}
        <p className={styles.stateDetail}>{STALE_TEXT.reason(reason)}</p>
      </div>
      {children}
    </div>
  );
}

/** Some sources answered and some did not. Names the ones that did not. */
export function PartialState({
  unavailable,
  onRetry,
  children,
}: {
  unavailable: readonly string[];
  onRetry?: () => void;
  children: ReactNode;
}) {
  return (
    <div className={styles.stateWrapper}>
      <div className={styles.stateBlock} role="status">
        <p className={styles.stateHeading}>{PARTIAL_TEXT.heading}</p>
        <p className={styles.stateDetail}>{PARTIAL_TEXT.body}</p>
        <ul className={styles.stateList} aria-label={PARTIAL_TEXT.listLabel}>
          {unavailable.map((source) => <li key={source}>{source}</li>)}
        </ul>
        {onRetry && <Button variant="secondary" onClick={onRetry}>{PARTIAL_TEXT.retry}</Button>}
      </div>
      {children}
    </div>
  );
}

/** An upstream dependency is impaired: which one, what works, what does not. */
export function DegradedState({
  dependency,
  stillWorks,
  doesNotWork,
  onRetry,
}: {
  dependency: string;
  stillWorks: string;
  doesNotWork: string;
  onRetry?: () => void;
}) {
  return (
    <Blocking>
      <p className={styles.stateHeading}>{DEGRADED_TEXT.heading(dependency)}</p>
      <p className={styles.stateDetail}>{DEGRADED_TEXT.works(stillWorks)}</p>
      <p className={styles.stateDetail}>{DEGRADED_TEXT.broken(doesNotWork)}</p>
      {onRetry && <Button variant="secondary" onClick={onRetry}>{DEGRADED_TEXT.retry}</Button>}
    </Blocking>
  );
}

/**
 * Authorization refused.
 *
 * Takes no subject and renders no count, name, or identifier. The refusal
 * withheld what exists, so the state must not restate it.
 */
export function DeniedState({ reauthenticate = false }: { reauthenticate?: boolean }) {
  return (
    <Blocking>
      <p className={styles.stateHeading}>{DENIED_TEXT.heading}</p>
      <p className={styles.stateDetail}>{DENIED_TEXT.body}</p>
      {reauthenticate && <p className={styles.stateDetail}>{DENIED_TEXT.reauthenticate}</p>}
    </Blocking>
  );
}

/** Unexpected failure. A fixed message, and retry only when retryable. */
export function ErrorState({ retryable = false, onRetry }: { retryable?: boolean; onRetry?: () => void }) {
  return (
    <Blocking>
      <p className={styles.stateHeading}>{ERROR_TEXT.heading}</p>
      <p className={styles.stateDetail}>{ERROR_TEXT.body}</p>
      {retryable && onRetry && <Button variant="secondary" onClick={onRetry}>{ERROR_TEXT.retry}</Button>}
    </Blocking>
  );
}
