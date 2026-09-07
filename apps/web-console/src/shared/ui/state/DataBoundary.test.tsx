import { render, screen } from '@testing-library/react';
import { describe, expect, it } from 'vitest';
import { ConsoleRequestError, consoleError } from '@shared/api/error';
import { expectNoAxeViolations } from '@shared/testing/axe';
import { DataBoundary, type QueryLike } from './DataBoundary';
import { DEFAULT_FRESHNESS_CLASS } from './freshness';
import { staleAfter } from '@shared/api/freshness';

/** The staleness threshold of the class these states are rendered for. */
const STALE_AFTER_MS = staleAfter(DEFAULT_FRESHNESS_CLASS) ?? 0;

/**
 * `DataBoundary` end to end: a query result in, exactly one state out.
 *
 * `resolveDataState.test.ts` asserts the mapping table itself. This asserts
 * that the component renders the state the table chose, with the screen's own
 * words in it — the split that stops a route re-deriving `denied` or `unknown`
 * for itself the way `P1-W11` did.
 */
const subject = {
  name: 'Edge devices',
  observationSource: 'an enrolled Edge reports its conditions',
  dependency: 'The Control Plane',
  stillWorks: 'navigation',
  doesNotWork: 'the device inventory',
  staleReason: 'the last refresh did not return',
};

const query = <T,>(overrides: Partial<QueryLike<T>> = {}): QueryLike<T> => ({
  isPending: false,
  data: undefined,
  error: null,
  ...overrides,
});

const failing = <T,>(kind: Parameters<typeof consoleError>[0], data?: T): QueryLike<T> =>
  query<T>({ error: new ConsoleRequestError(consoleError(kind)), ...(data === undefined ? {} : { data }) });

function renderBoundary(result: QueryLike<string[]>, extra: Record<string, unknown> = {}) {
  return render(
    <DataBoundary query={result} subject={subject} {...extra}>
      {(devices) => <ul>{devices.map((device) => <li key={device}>{device}</li>)}</ul>}
    </DataBoundary>,
  );
}

describe('DataBoundary', () => {
  it('renders the data when the read is ready', () => {
    renderBoundary(query({ data: ['edge-one'] }));
    expect(screen.getByText('edge-one')).toBeVisible();
  });

  it('names the activity while loading', () => {
    renderBoundary(query({ isPending: true }));
    expect(screen.getByRole('status')).toHaveTextContent('Loading Edge devices');
  });

  it('names the collection and offers the creating action when empty', () => {
    renderBoundary(query({ data: [] }), { emptyAction: <button type="button">Enroll the first Edge</button> });
    expect(screen.getByText('No Edge devices recorded')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Enroll the first Edge' })).toBeVisible();
  });

  it('renders a refused read as denied, never as an empty collection', () => {
    renderBoundary(failing('forbidden', []));
    const alert = screen.getByRole('alert');
    expect(alert).toHaveTextContent('Access was refused');
    expect(screen.queryByText('No Edge devices recorded')).toBeNull();
    expect(alert).not.toHaveTextContent('Edge devices');
  });

  it('renders a missing observation as unknown when the resource is observation-shaped', () => {
    renderBoundary(failing('not-found'), { observationShaped: true });
    expect(screen.getByText('No observation exists')).toBeVisible();
    expect(screen.getByText(/an enrolled Edge reports its conditions/)).toBeVisible();
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('keeps the cached data visible and dates it when a refresh fails', () => {
    const observedAt = new Date(Date.now() - 2 * 60_000).toISOString();
    renderBoundary(failing('unavailable', ['edge-one']), { observedAt });
    expect(screen.getByText('edge-one')).toBeVisible();
    expect(screen.getByText(/Observed 2 minutes ago/)).toBeVisible();
    expect(screen.getByText(/the last refresh did not return/)).toBeVisible();
  });

  it('names the impaired dependency when there is nothing cached to show', () => {
    renderBoundary(failing('unavailable'));
    expect(screen.getByText('The Control Plane is impaired')).toBeVisible();
    expect(screen.getByText(/Still answering: navigation/)).toBeVisible();
    expect(screen.getByText(/Not answering: the device inventory/)).toBeVisible();
  });

  it('lists the sources a partial read could not load beside the data it could', () => {
    renderBoundary(query({ data: ['edge-one'] }), { partialFailures: ['device health'] });
    expect(screen.getByText('edge-one')).toBeVisible();
    expect(screen.getByRole('list', { name: 'Sources that could not be read' })).toHaveTextContent('device health');
  });

  it('renders a successful read past its freshness policy as stale', () => {
    const observedAt = new Date(Date.now() - STALE_AFTER_MS - 60_000).toISOString();
    renderBoundary(query({ data: ['edge-one'] }), { observedAt });
    expect(screen.getByText('edge-one')).toBeVisible();
    expect(screen.getByText('Showing the last data Guardian received')).toBeVisible();
  });

  it('offers a retry only when the classified error is retryable', () => {
    const retryable = renderBoundary(failing('network'), { onRetry: () => undefined });
    expect(screen.getByRole('button', { name: 'Try again' })).toBeVisible();
    retryable.unmount();

    renderBoundary(failing('conflict'), { onRetry: () => undefined });
    expect(screen.queryByRole('button', { name: 'Try again' })).toBeNull();
  });

  it('leaks no diagnostic detail from the classified error', () => {
    renderBoundary(failing('unexpected'));
    const text = document.body.textContent ?? '';
    expect(text).not.toMatch(/unexpected|errors\.|\b5\d\d\b/i);
  });

  it('reports no serious or critical axe violation in any outcome it renders', async () => {
    for (const result of [
      query<string[]>({ isPending: true }),
      query<string[]>({ data: [] }),
      query<string[]>({ data: ['edge-one'] }),
      failing<string[]>('forbidden'),
      failing<string[]>('unavailable'),
      failing<string[]>('network'),
      failing<string[]>('not-found'),
    ]) {
      const view = renderBoundary(result, { observationShaped: true, onRetry: () => undefined });
      await expectNoAxeViolations(view.container);
      view.unmount();
    }
  });

  it('treats an absent projection as unknown rather than as a successful empty read', () => {
    render(
      <DataBoundary
        query={query<{ aggregate: null } | null>({ data: null })}
        subject={subject}
        isAbsent={(value) => value === null}
      >
        {() => <p>projection</p>}
      </DataBoundary>,
    );
    expect(screen.getByText('No observation exists')).toBeVisible();
    expect(screen.queryByText('projection')).toBeNull();
  });
});
