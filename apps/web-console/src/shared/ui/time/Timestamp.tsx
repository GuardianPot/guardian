import { useEffect, useState } from 'react';
import { plural, t } from '@shared/text';
import styles from '@shared/styles/app.module.css';

/**
 * The canonical timestamp (WCX-08 section 9.3, decision WC-D18).
 *
 * `P1-W11` formatted every time as a local medium date and short time: no
 * timezone, no seconds, no absolute value. For an attacker journey whose
 * correctness *is* ordering, that is not enough — and it sits oddly beside
 * `clock_quality` being a health condition the product already tracks.
 *
 * Four rules this component exists to hold:
 *
 * 1. **Never a bare local time.** The zone abbreviation is always visible, so
 *    an operator reading a timestamp in a report knows which clock it is on.
 * 2. **The absolute instant is always retrievable.** The full UTC ISO value is
 *    in the accessible description, not only in a hover title, so it is
 *    reachable without a mouse.
 * 3. **Never fabricate precision.** An absent, empty, or unparseable value
 *    renders the unknown treatment. Never a default date, never the epoch,
 *    never now.
 * 4. **Degraded clock quality is visible.** When the source device reported
 *    poor clock quality, saying nothing would present its timestamp as
 *    authoritative. The marker is text plus a neutral token, never a severity
 *    colour: a bad clock is not an incident.
 */
export type TimestampPrecision = 'minute' | 'second';
export type TimestampMode = 'absolute' | 'absoluteWithRelative';

export type TimestampProps = {
  /** ISO-8601 from the backend. */
  value: string | null | undefined;
  /**
   * `second` is mandatory in evidence, journey, audit, and health transition
   * contexts, where ordering is the thing being established.
   */
  precision?: TimestampPrecision;
  mode?: TimestampMode;
  /** True when the source device reported degraded `clock_quality`. */
  uncertainClock?: boolean;
};

/** `Intl` formatters are expensive to construct, so each is built once. */
const FORMATTERS = new Map<string, Intl.DateTimeFormat>();

function formatter(precision: TimestampPrecision): Intl.DateTimeFormat {
  const existing = FORMATTERS.get(precision);
  if (existing) return existing;
  const created = new Intl.DateTimeFormat(undefined, {
    // Spelled out component by component rather than as `dateStyle` plus
    // `timeStyle`: `Intl` rejects those alongside `timeZoneName`, and the zone
    // is the point. A local time with no zone is not an instant.
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
    ...(precision === 'second' ? { second: '2-digit' as const } : {}),
    timeZoneName: 'short',
  });
  FORMATTERS.set(precision, created);
  return created;
}

/**
 * An ISO-8601 instant with a time of day.
 *
 * Date-only and other shapes are rejected rather than parsed. `Date.parse` is
 * permissive by design — it reads `'0'` as the first of January 2000 — and a
 * permissive parse here is precisely the fabricated precision section 9.8.4
 * forbids. The contract says ISO-8601, so anything else is unknown.
 */
const ISO_INSTANT = /^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}(:\d{2}(\.\d+)?)?(Z|[+-]\d{2}:?\d{2})?$/;

/** Milliseconds since the epoch, or `null` when the value is not an instant. */
export function parseInstant(value: string | null | undefined): number | null {
  if (value === null || value === undefined || !ISO_INSTANT.test(value)) return null;
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? null : parsed;
}

const UNITS: readonly { limit: number; size: number; base: 'time.age.second' | 'time.age.minute' | 'time.age.hour' }[] = [
  { limit: 60_000, size: 1_000, base: 'time.age.second' },
  { limit: 3_600_000, size: 60_000, base: 'time.age.minute' },
  { limit: 86_400_000, size: 3_600_000, base: 'time.age.hour' },
];

/**
 * An elapsed duration in whole units.
 *
 * The single implementation: `WCX-04`'s `formatAge` and `WCX-07`'s helper both
 * delegate here, so the `stale` state and a relative timestamp cannot disagree
 * about how long ago something was.
 */
export function formatAge(observedAt: string, now: number = Date.now()): string {
  const observed = parseInstant(observedAt);
  if (observed === null) return t('time.age.unknown');
  const elapsed = Math.max(0, now - observed);
  const unit = UNITS.find((candidate) => elapsed < candidate.limit);
  if (!unit) return plural('time.age.day', Math.floor(elapsed / 86_400_000));
  return plural(unit.base, Math.floor(elapsed / unit.size));
}

/** Recomputes once a minute, which is the finest granularity ever displayed. */
function useMinuteTick(active: boolean): number {
  const [tick, setTick] = useState(() => Date.now());
  useEffect(() => {
    if (!active) return;
    const timer = setInterval(() => setTick(Date.now()), 60_000);
    return () => clearInterval(timer);
  }, [active]);
  return tick;
}

export function Timestamp({
  value,
  precision = 'minute',
  mode = 'absolute',
  uncertainClock = false,
}: TimestampProps) {
  const relative = mode === 'absoluteWithRelative';
  const now = useMinuteTick(relative);
  const instant = parseInstant(value);

  // Section 9.3.6. No fallback date: an unreadable timestamp is unknown, and
  // a made-up one would be worse than none at all.
  if (instant === null) {
    return <span className={styles.timestampUnknown}>{t('time.unknown')}</span>;
  }

  const iso = new Date(instant).toISOString();
  const description = [
    t('time.utcDescription', { iso }),
    uncertainClock ? t('time.degradedClockDescription') : '',
  ].filter(Boolean).join('. ');

  return (
    <time className={styles.timestamp} dateTime={iso} title={description}>
      {formatter(precision).format(instant)}
      {/*
        Relative time never replaces the absolute value (section 9.3.5). It is
        a convenience beside it, not a substitute for it.
      */}
      {relative && <span className={styles.timestampRelative}>{t('time.relative', { age: formatAge(iso, now) })}</span>}
      {uncertainClock && (
        <span className={styles.timestampUncertain}>{t('time.degradedClock')}</span>
      )}
      {/*
        The UTC instant, and the clock caveat, reachable without a pointer.
        `title` alone is mouse-only.
      */}
      <span className={styles.visuallyHidden}>{description}</span>
    </time>
  );
}
