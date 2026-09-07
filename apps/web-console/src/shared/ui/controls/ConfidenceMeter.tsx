import { confidenceEncoding } from '@shared/theme/statusEncoding';
import styles from '@shared/styles/app.module.css';
import { CONFIDENCE_METER_TEXT } from './text';

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
      <span className={styles.confidenceLabel}>{CONFIDENCE_METER_TEXT.label}</span>
      <span className={styles.confidenceSteps} aria-hidden="true">
        {Array.from({ length: encoding.total }, (_, index) => (
          <span
            key={index}
            className={index < encoding.filled ? styles.confidenceStepFilled : styles.confidenceStep}
          />
        ))}
      </span>
      <span>{CONFIDENCE_METER_TEXT.value(encoding.label, encoding.filled, encoding.total)}</span>
    </p>
  );
}
