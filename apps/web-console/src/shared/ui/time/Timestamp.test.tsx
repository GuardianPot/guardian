import { act, render, screen } from '@testing-library/react';
import { readFileSync } from 'node:fs';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { expectNoAxeViolations } from '@shared/testing/axe';
import { Timestamp, formatAge, parseInstant } from './Timestamp';

/**
 * The canonical timestamp (WCX-08 section 9.3, tests 10.1.6-12).
 *
 * The rules under test are all one rule seen from different sides: never claim
 * more about an instant than the data supports. A bare local time claims a
 * zone it does not state; a fabricated date claims an observation that never
 * happened; an unmarked timestamp from a device with a bad clock claims an
 * ordering the device itself could not establish.
 */
const INSTANT = '2026-09-01T14:23:45.678Z';

/** The `<time>` element, or a failure that says so rather than a null deref. */
function timeElement(): HTMLTimeElement {
  const element = document.querySelector('time');
  if (element === null) throw new Error('no <time> element was rendered');
  return element;
}

/**
 * Just the formatted instant.
 *
 * `textContent` also carries the visually-hidden UTC description, which
 * contains seconds by construction — asserting against it would pass whatever
 * the visible precision turned out to be.
 */
function visibleTime(): string {
  return timeElement().firstChild?.textContent ?? '';
}

describe('the element', () => {
  it('is a <time> carrying the original instant in dateTime', () => {
    render(<Timestamp value={INSTANT} />);
    // Section 10.1.6. Machine-readable and unrounded, whatever the display
    // precision does.
    expect(timeElement().dateTime).toBe(INSTANT);
  });

  it('keeps dateTime at full precision even when the display is to the minute', () => {
    render(<Timestamp value={INSTANT} precision="minute" />);
    expect(timeElement().dateTime).toBe(INSTANT);
    expect(visibleTime()).not.toContain('45');
  });

  it('normalises a non-UTC input to the same instant in UTC', () => {
    render(<Timestamp value="2026-09-01T16:23:45.678+02:00" />);
    expect(timeElement().dateTime).toBe(INSTANT);
  });

  it('has no accessibility violations', async () => {
    const { container } = render(<Timestamp value={INSTANT} mode="absoluteWithRelative" uncertainClock />);
    await expectNoAxeViolations(container);
  });
});

describe('the zone', () => {
  it.each([
    ['minute' as const],
    ['second' as const],
  ])('is named in the visible text at %s precision', (precision) => {
    render(<Timestamp value={INSTANT} precision={precision} />);
    // Section 10.1.7. A local time with no zone is not an instant, and an
    // operator reading it in an incident report cannot recover which clock it
    // was on. `Intl` renders the abbreviation as letters or as a `GMT±h`
    // offset depending on the locale data; both name the zone.
    expect(visibleTime()).toMatch(/[A-Z]{2,5}$|GMT[+-]\d/);
  });

  it('puts the UTC instant in the accessible description, not only in a title', () => {
    // Section 10.1.8. `title` is mouse-only; the description has to be
    // reachable by a screen reader and by keyboard.
    render(<Timestamp value={INSTANT} />);
    expect(screen.getByText(`UTC ${INSTANT}`)).toBeInTheDocument();
    expect(timeElement().title).toContain(INSTANT);
  });
});

describe('precision', () => {
  it('renders seconds when asked for them', () => {
    // Section 10.1.9. Evidence, journey, audit, and health-transition
    // surfaces reason about ordering, so seconds are not decoration there.
    render(<Timestamp value={INSTANT} precision="second" />);
    expect(visibleTime()).toMatch(/:\d\d:45/);
  });

  it('omits them when it is not', () => {
    render(<Timestamp value={INSTANT} precision="minute" />);
    expect(visibleTime()).not.toMatch(/:\d\d:\d\d/);
  });

  it('defaults to the minute, so seconds are a deliberate choice', () => {
    render(<Timestamp value={INSTANT} />);
    expect(visibleTime()).not.toMatch(/:\d\d:\d\d/);
  });
});

describe('an instant that is not known', () => {
  const NOT_A_TIME: readonly (string | null | undefined)[] = [
    undefined,
    null,
    '',
    'not-a-timestamp',
    '2026-13-45T99:99:99Z',
    '0',
  ];

  it.each(NOT_A_TIME)('renders the unknown treatment for %o and never a date', (value) => {
    // Section 10.1.10 and 9.8.4. Never a default date, never the epoch, never
    // now. A fabricated timestamp is worse than an admitted absence, because
    // an operator can act on the second and cannot detect the first.
    const { container } = render(<Timestamp value={value} />);

    expect(screen.getByText('No timestamp was recorded')).toBeInTheDocument();
    expect(container.querySelector('time')).toBeNull();
    expect(container.textContent).not.toMatch(/1970|2026|\d{1,2}:\d{2}/);
  });

  it('does not fall back to the current time', () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-09-07T09:00:00.000Z'));
    try {
      const { container } = render(<Timestamp value={undefined} />);
      expect(container.textContent).not.toContain('9:00');
      expect(container.textContent).not.toContain('2026');
    } finally {
      vi.useRealTimers();
    }
  });

  it('reads as unknown rather than as an empty cell', () => {
    const { container } = render(<Timestamp value={null} />);
    expect(container.textContent?.trim()).not.toBe('');
  });
});

describe('a source with degraded clock quality', () => {
  it('renders the marker and its description', () => {
    // Section 10.1.11 and 9.8.5. Saying nothing would present the device's
    // timestamp as authoritative, which overstates the evidence.
    render(<Timestamp value={INSTANT} uncertainClock />);

    expect(screen.getByText('clock quality degraded')).toBeInTheDocument();
    expect(
      screen.getByText(/The source device reported degraded clock quality/),
    ).toBeInTheDocument();
  });

  it('says nothing when the clock is not known to be bad', () => {
    render(<Timestamp value={INSTANT} />);
    expect(screen.queryByText('clock quality degraded')).not.toBeInTheDocument();
  });

  it('uses a neutral token, never a severity one', () => {
    // Section 9.9. A bad clock is a caveat about evidence quality, not an
    // incident, and colouring it as one would be a false severity signal.
    const css = readFileSync('src/shared/styles/app.module.css', 'utf8');
    const rule = /\.timestampUncertain\s*\{([^}]*)\}/.exec(css)?.[1] ?? '';
    expect(rule, '.timestampUncertain must exist').not.toBe('');
    expect(rule).not.toMatch(/--severity-|--status-|--health-/);
    expect(rule).toMatch(/--text-muted|--line-strong/);
  });

  it('introduces no new theme token', () => {
    const semantic = readFileSync('src/shared/theme/semantic.css', 'utf8');
    expect(semantic).not.toMatch(/--clock|--timestamp/);
  });
});

describe('relative time', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-09-01T14:38:45.678Z'));
  });
  afterEach(() => {
    vi.useRealTimers();
  });

  it('appears only beside the absolute value, never instead of it', () => {
    // Section 10.1.12 and 9.3.5. "15 minutes ago" alone cannot be put in a
    // report and cannot be compared against a device log.
    render(<Timestamp value={INSTANT} mode="absoluteWithRelative" />);

    expect(screen.getByText('15 minutes ago')).toBeInTheDocument();
    expect(timeElement().dateTime).toBe(INSTANT);
    expect(visibleTime()).toMatch(/\d{1,2}:\d{2}/);
  });

  it('is absent in the default mode', () => {
    render(<Timestamp value={INSTANT} />);
    expect(screen.queryByText(/ago/)).not.toBeInTheDocument();
  });

  it('announces nothing when it recomputes', () => {
    // A timestamp that quietly ages must not interrupt a screen reader every
    // minute. No live region, no status role, anywhere in the subtree.
    const { container } = render(<Timestamp value={INSTANT} mode="absoluteWithRelative" />);

    act(() => { vi.advanceTimersByTime(120_000); });

    expect(screen.getByText('17 minutes ago')).toBeInTheDocument();
    expect(container.querySelector('[aria-live]')).toBeNull();
    expect(container.querySelector('[role="status"], [role="alert"], [role="timer"]')).toBeNull();
    expect(container.querySelector('[aria-atomic]')).toBeNull();
  });

  it('starts no timer when it is not showing relative time', () => {
    const setInterval = vi.spyOn(globalThis, 'setInterval');
    render(<Timestamp value={INSTANT} />);
    expect(setInterval).not.toHaveBeenCalled();
    setInterval.mockRestore();
  });

  it('clears its timer on unmount', () => {
    // `controls.test.tsx` exempts this module from the layer's no-interval
    // rule on the strength of these two tests. A timer that outlived its
    // component would be exactly the global subscription that rule forbids.
    const clearInterval = vi.spyOn(globalThis, 'clearInterval');
    const { unmount } = render(<Timestamp value={INSTANT} mode="absoluteWithRelative" />);

    expect(clearInterval).not.toHaveBeenCalled();
    unmount();
    expect(clearInterval).toHaveBeenCalled();
    clearInterval.mockRestore();
  });
});

describe('formatAge', () => {
  const now = Date.parse('2026-09-01T14:23:45.678Z');
  const ago = (ms: number) => new Date(now - ms).toISOString();

  it.each([
    [0, '0 seconds'],
    [1_000, '1 second'],
    [45_000, '45 seconds'],
    [60_000, '1 minute'],
    [3_599_000, '59 minutes'],
    [3_600_000, '1 hour'],
    [86_399_000, '23 hours'],
    [86_400_000, '1 day'],
    [4 * 86_400_000, '4 days'],
  ])('renders %i ms as %s', (elapsed, expected) => {
    expect(formatAge(ago(elapsed), now)).toBe(expected);
  });

  it('never reports a negative age from a clock that is ahead', () => {
    // A device clock ahead of the console's is exactly the case
    // `clock_quality` exists to flag. "-3 minutes ago" is not an improvement
    // on "0 seconds", and it reads as a bug rather than as a caveat.
    expect(formatAge(ago(-180_000), now)).toBe('0 seconds');
  });

  it('admits an unparseable value rather than guessing', () => {
    expect(formatAge('not-a-timestamp', now)).toBe('an unknown age');
  });
});

describe('parseInstant', () => {
  it.each([undefined, null, '', 'not-a-timestamp'])('returns null for %o', (value) => {
    expect(parseInstant(value)).toBeNull();
  });

  it('returns the epoch milliseconds for a real instant', () => {
    expect(parseInstant(INSTANT)).toBe(Date.parse(INSTANT));
  });
});
