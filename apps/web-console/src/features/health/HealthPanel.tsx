import type { HealthView } from '@shared/api/types';
import { isEmptyUntrusted } from '@shared/api/untrusted';
import { healthEncoding } from '@shared/theme/statusEncoding';
import { Panel, StatusBadge, UntrustedText } from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { HEALTH_TEXT } from './text';

/**
 * The backend health projection, rendered as text (P1-W11, WCX-04 section 9.6).
 *
 * Every condition keeps its own status word, so `False` and `Unknown` never
 * collapse into one another and neither reads as healthy. The status words come
 * from the `WCX-03` encoding table rather than from a local map, so a value the
 * backend adds later resolves to the unknown treatment instead of falling
 * through to a healthy-looking blank.
 *
 * The condition list deliberately carries no glyph: the aggregate badge above
 * it does, and the browser suite asserts that nothing inside this list is an
 * embedded element, because hostile condition text travels through it.
 */
export function HealthPanel({ health }: { health: HealthView }) {
  const aggregate = health.aggregate.status;
  return (
    <Panel
      heading={HEALTH_TEXT.heading}
      headingLevel={2}
      eyebrow={HEALTH_TEXT.eyebrow}
      aside={<StatusBadge encoding={healthEncoding(aggregate)} />}
    >
      {health.aggregate.blocking_type && (
        <p className={styles.blocking}>
          {HEALTH_TEXT.blocking}
          {HEALTH_TEXT.conditions[health.aggregate.blocking_type] ?? health.aggregate.blocking_type}
          {health.aggregate.reason && <> — <UntrustedText value={health.aggregate.reason} /></>}
          {health.aggregate.blocking_device_id && (
            <> · source <UntrustedText value={health.aggregate.blocking_device_id} /></>
          )}
        </p>
      )}
      <ul className={styles.conditionGrid} aria-label={HEALTH_TEXT.conditionsLabel}>
        {health.conditions.map((condition) => (
          <li key={condition.type} className={styles.condition}>
            <span className={`${styles.conditionDot} ${styles[healthEncoding(condition.status).tone] ?? ''}`} aria-hidden="true" />
            <div>
              <strong>{HEALTH_TEXT.conditions[condition.type] ?? condition.type}</strong>
              {/*
                Reason, message, and source device come from the device itself.
                A compromised or emulated Edge writes them, so they render
                through the untrusted contract (WCX-06 section 9.1).
              */}
              <span>{healthEncoding(condition.status).label} · <UntrustedText value={condition.reason} /></span>
              {!isEmptyUntrusted(condition.message) && <p><UntrustedText value={condition.message} /></p>}
              {condition.source_device_id && (
                <small>{HEALTH_TEXT.sourceDevice} <UntrustedText value={condition.source_device_id} /></small>
              )}
            </div>
          </li>
        ))}
      </ul>
      <p className={styles.timestamp}>{HEALTH_TEXT.receivedAt(formatTime(health.received_at))}</p>
    </Panel>
  );
}

export function formatTime(value: string) {
  return new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value));
}
