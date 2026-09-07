import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import {
  DegradedState,
  DeniedState,
  EmptyState,
  ErrorState,
  LoadingState,
  PartialState,
  StaleState,
  UnknownState,
} from './states';
import { FRESHNESS_LIMIT_MS } from './freshness';

/**
 * The negative assertions that carry `WCX-04` section 8.2, 8.3, and 8.4.
 *
 * Each is written as a *negative* on purpose. A positive assertion that the
 * `unknown` state says "no observation exists" would still pass if the same
 * component also said "all conditions healthy" somewhere else on the block.
 */
const AFFIRMATIVE =
  /\b(?:complete|completed|success|successful|succeeded|up to date|all clear|no issues|nothing to report|operational|protected|working normally|as expected)\b/i;

/** The health-true and health-false labels from the WCX-03 encoding table. */
const HEALTH_LABELS = ['Healthy', 'Action required'];

const EVERY_STATE: readonly { name: string; render: () => void }[] = [
  { name: 'loading', render: () => { render(<LoadingState activity="Loading the device inventory" />); } },
  { name: 'empty', render: () => { render(<EmptyState collection="Edge devices" />); } },
  { name: 'unknown', render: () => { render(<UnknownState subject="the health of this device" observationSource="an enrolled Edge reports its conditions" />); } },
  {
    name: 'stale',
    render: () => {
      render(
        <StaleState observedAt={new Date(Date.now() - 5 * 60_000).toISOString()} reason="the channel is closed">
          <p>cached device list</p>
        </StaleState>,
      );
    },
  },
  {
    name: 'partial',
    render: () => {
      render(
        <PartialState unavailable={['device health']} onRetry={() => undefined}>
          <p>cached device list</p>
        </PartialState>,
      );
    },
  },
  { name: 'degraded', render: () => { render(<DegradedState dependency="The health projection" stillWorks="inventory" doesNotWork="every health condition" />); } },
  { name: 'denied', render: () => { render(<DeniedState />); } },
  { name: 'error', render: () => { render(<ErrorState retryable onRetry={() => undefined} />); } },
];

function bodyText(): string {
  return document.body.textContent ?? '';
}

describe('the eight data states', () => {
  it.each(EVERY_STATE)('never renders $name as healthy, complete, or successful', ({ render: mount }) => {
    mount();
    const text = bodyText();
    expect(text.length).toBeGreaterThan(0);
    expect(text).not.toMatch(AFFIRMATIVE);
    // "healthy" is permitted in exactly one place: the sentence that denies it.
    expect(text.replace(/not a healthy signal/gi, '')).not.toMatch(/healthy/i);
    for (const label of HEALTH_LABELS) {
      expect(screen.queryByText(label, { exact: true }), label).toBeNull();
    }
  });

  it('announces informational states as status and blocking states as alert', () => {
    render(<LoadingState activity="Loading the device inventory" />);
    expect(screen.getByRole('status')).toBeVisible();
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('distinguishes denied from empty', () => {
    // Section 8.3. A refusal that read as an empty list would tell an operator
    // that no devices exist when in fact access was refused.
    const empty = render(<EmptyState collection="Edge devices" />);
    const emptyText = empty.container.textContent ?? '';
    empty.unmount();

    const denied = render(<DeniedState />);
    const deniedText = denied.container.textContent ?? '';

    expect(deniedText).not.toBe(emptyText);
    expect(deniedText).toMatch(/refused/i);
    expect(deniedText).not.toMatch(/\bzero\b/i);
    expect(deniedText).not.toMatch(/Edge devices/);
    // A refusal is a blocking condition, and an empty collection is not.
    expect(screen.getByRole('alert')).toBeVisible();
  });

  it('says nothing about what exists when access was refused', () => {
    render(<DeniedState />);
    const text = bodyText();
    expect(text).not.toMatch(/\b\d+\b/);
    expect(text).toMatch(/reports nothing about what is or is not here/i);
  });

  it('distinguishes unknown from empty and from a failing state', () => {
    // Section 8.4. Absence of a health projection is neither a healthy signal
    // nor a negative observation.
    const unknown = render(<UnknownState subject="the health of this device" observationSource="an enrolled Edge reports its conditions" />);
    const unknownText = unknown.container.textContent ?? '';
    expect(unknownText).toMatch(/not a healthy signal/i);
    expect(unknownText).toMatch(/not a failure signal/i);
    expect(unknownText).not.toMatch(/\bzero\b/i);
    expect(screen.queryByRole('alert')).toBeNull();
    expect(screen.getByRole('status')).toBeVisible();
    unknown.unmount();

    const empty = render(<EmptyState collection="Edge devices" />);
    expect(empty.container.textContent).not.toBe(unknownText);
  });

  it('states what would produce an observation', () => {
    render(<UnknownState subject="the health of this device" observationSource="an enrolled Edge reports its conditions" />);
    expect(screen.getByText(/an enrolled Edge reports its conditions/)).toBeVisible();
  });

  it('renders stale data with its observation age and the reason refresh is not current', () => {
    const observedAt = new Date(Date.now() - 3 * 60_000 - 1_000).toISOString();
    render(
      <StaleState observedAt={observedAt} reason="the channel is closed">
        <p>cached device list</p>
      </StaleState>,
    );
    expect(screen.getByText('cached device list')).toBeVisible();
    expect(screen.getByText(/Observed 3 minutes ago/)).toBeVisible();
    expect(screen.getByText(/the channel is closed/)).toBeVisible();
    expect(FRESHNESS_LIMIT_MS).toBeGreaterThan(0);
  });

  it('lists what a partial result could not load and offers a retry', () => {
    const retried: string[] = [];
    render(
      <PartialState unavailable={['device health', 'zone list']} onRetry={() => retried.push('retry')}>
        <p>cached device list</p>
      </PartialState>,
    );
    expect(screen.getByText('cached device list')).toBeVisible();
    const list = screen.getByRole('list', { name: 'Sources that could not be read' });
    expect(list.querySelectorAll('li')).toHaveLength(2);
    expect(list).toHaveTextContent('device health');
    expect(list).toHaveTextContent('zone list');
    screen.getByRole('button', { name: 'Retry the missing sources' }).click();
    expect(retried).toEqual(['retry']);
  });

  it('names the impaired dependency and what does and does not still answer', () => {
    render(<DegradedState dependency="The health projection" stillWorks="inventory and configuration" doesNotWork="every health condition" />);
    expect(screen.getByText('The health projection is impaired')).toBeVisible();
    expect(screen.getByText(/Still answering: inventory and configuration/)).toBeVisible();
    expect(screen.getByText(/Not answering: every health condition/)).toBeVisible();
  });

  it('offers a retry on a retryable error and none on an unretryable one', () => {
    const retryable = render(<ErrorState retryable onRetry={() => undefined} />);
    expect(screen.getByRole('button', { name: 'Try again' })).toBeVisible();
    retryable.unmount();

    render(<ErrorState onRetry={() => undefined} />);
    expect(screen.queryByRole('button', { name: 'Try again' })).toBeNull();
  });

  it('renders no diagnostic detail on an error', () => {
    render(<ErrorState />);
    const text = bodyText();
    expect(text).not.toMatch(/\b[45]\d\d\b/);
    expect(text).not.toMatch(/stack|exception|http|fetch/i);
  });
});
