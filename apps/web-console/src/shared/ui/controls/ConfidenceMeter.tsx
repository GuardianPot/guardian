import { confidenceEncoding } from '@shared/theme/statusEncoding';
import styles from '@shared/styles/app.module.css';
import { t } from '@shared/text';

/**
 * Confidence as a neutral stepped indicator (WCX-04 section 9.5, WC-D11).
 *
 * Confidence is never colour-coded and never shares the severity ramp, so the
 * steps are neutral and the value is also rendered as text. An unrecognised
 * value resolves to zero filled steps labelled `Unknown` — never to a high
 * reading.
 */
export function ConfidenceMeter({ value }: { value: string }) {
  const encoding = confidenceEncoding(value);
  return (
    <p className={styles.confidence}>
      <span className={styles.confidenceLabel}>{t('common.confidence')}</span>
      <span className={styles.confidenceSteps} aria-hidden="true">
        {Array.from({ length: encoding.total }, (_, index) => (
          <span
            key={index}
            className={index < encoding.filled ? styles.confidenceStepFilled : styles.confidenceStep}
          />
        ))}
      </span>
      <span>{t('common.confidenceValue', { label: encoding.label, filled: encoding.filled, total: encoding.total })}</span>
    </p>
  );
}
