import type { HealthView } from '@shared/api/types';
import { isEmptyUntrusted } from '@shared/api/untrusted';
import { healthEncoding } from '@shared/theme/statusEncoding';
import { Panel, StatusBadge, Timestamp, UntrustedText } from '@shared/ui';
import styles from '@shared/styles/app.module.css';
import { t, tx } from '@shared/text';
import { clockQualityIsDegraded } from './clockQuality';

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
/**
 * The operator label for a backend condition type.
 *
 * The type is a closed enum in the contract, so the catalogue can carry a key
 * per value and the compiler checks the mapping is total.
 */
function conditionLabel(type: HealthView['conditions'][number]['type']): string {
  return t(`health.condition.${type}`);
}

export function HealthPanel({ health }: { health: HealthView }) {
  const aggregate = health.aggregate.status;
  const degradedClock = clockQualityIsDegraded(health);
  return (
    <Panel
      heading={t('health.heading')}
      headingLevel={2}
      eyebrow={t('health.eyebrow')}
      aside={<StatusBadge encoding={healthEncoding(aggregate)} />}
    >
      {health.aggregate.blocking_type && (
        <p className={styles.blocking}>
          {t('health.blocking')}
          {conditionLabel(health.aggregate.blocking_type)}
          {health.aggregate.reason && <> — <UntrustedText value={health.aggregate.reason} /></>}
          {health.aggregate.blocking_device_id && (
            <> · {tx('health.observedSource', { device: <UntrustedText value={health.aggregate.blocking_device_id} /> })}</>
          )}
        </p>
      )}
      <ul className={styles.conditionGrid} aria-label={t('health.conditionsLabel')}>
        {health.conditions.map((condition) => (
          <li key={condition.type} className={styles.condition}>
            <span className={`${styles.conditionDot} ${styles[healthEncoding(condition.status).tone] ?? ''}`} aria-hidden="true" />
            <div>
              <strong>{conditionLabel(condition.type)}</strong>
              {/*
                Reason, message, and source device come from the device itself.
                A compromised or emulated Edge writes them, so they render
                through the untrusted contract (WCX-06 section 9.1).
              */}
              <span>{healthEncoding(condition.status).label} · <UntrustedText value={condition.reason} /></span>
              {!isEmptyUntrusted(condition.message) && <p><UntrustedText value={condition.message} /></p>}
              {condition.source_device_id && (
                <small>{t('health.reportedBy')} <UntrustedText value={condition.source_device_id} /></small>
              )}
            </div>
          </li>
        ))}
      </ul>
      {/*
        Seconds are mandatory here: a health projection is evidence, and its
        ordering is what an operator reasons about (WCX-08 section 9.3.3). The
        timestamp is interpolated into the sentence rather than appended, so
        the sentence stays one reviewable catalogue entry.
      */}
      <p className={styles.timestamp}>
        {tx('health.receivedAt', {
          time: <Timestamp value={health.received_at} precision="second" uncertainClock={degradedClock} />,
        })}
      </p>
    </Panel>
  );
}
