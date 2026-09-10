import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { environment, environmentID, json, type MockResponder } from '@shared/testing/harness';
import { renderApp } from '@shared/testing/renderApp';
import { readScope } from './scope';

/**
 * The environment scope parameter (WCX-10 sections 8.2-8.3 and 9.2, tests
 * 10.1.2-5 and 11).
 *
 * Every test here is about a refusal. The parameter arrives from a link
 * someone else wrote, and the failure this package is guarding against is not
 * a crash — it is an operator who believes they are looking at environment A
 * while seeing environment B, acts on the wrong network, and is never told.
 * So: no fallback, ever, and nothing auto-chosen when there is a choice.
 */
const SECOND_ID = '018f1f7e-6d31-7cc5-8db8-17547f78e6ff';

const second = () => environment({ environment_id: SECOND_ID, display_name: 'Staging' });

function listing(...environments: ReturnType<typeof environment>[]): Record<string, MockResponder> {
  return {
    'GET /v1/environments?limit=200': () => json({ environments }),
    // Each listed environment is readable by id, so a scope change has
    // somewhere real to go.
    ...Object.fromEntries(environments.map((record) => [
      `GET /v1/environments/${record.environment_id}`,
      () => json({ environment: record }),
    ])),
  };
}

async function openHome(entry: string, handlers: Record<string, MockResponder> = listing(environment(), second())) {
  const view = renderApp(entry, { handlers });
  await screen.findByRole('link', { name: 'Guardian Console home' });
  return view;
}

describe('parsing', () => {
  it.each([
    ['absent', '', 'absent'],
    ['empty', '?env=', 'absent'],
    ['a valid uuid', `?env=${environmentID}`, 'selected'],
    ['a non-uuid', '?env=not-a-uuid', 'malformed'],
    // The shapes an attacker reaches for first. None of them is a UUID, so
    // none of them ever reaches a request path (section 8.2).
    ['a traversal', '?env=../../admin', 'malformed'],
    ['a url', '?env=https://elsewhere.example/', 'malformed'],
    ['markup', '?env=<img src=x onerror=alert(1)>', 'malformed'],
    ['a uuid with trailing junk', `?env=${environmentID}/x`, 'malformed'],
  ] as const)('reads %s as %s', (_name, query, state) => {
    expect(readScope(new URLSearchParams(query)).state).toBe(state);
  });
});

describe('with several environments and no parameter', () => {
  it('chooses nothing', async () => {
    // Section 9.2.3 and test 10.1.3. Picking the first would put an operator
    // in front of a network they did not ask for, and every screen after
    // that would look correct.
    const { router } = await openHome('/');

    const selector = await screen.findByLabelText('Environment scope');
    await waitFor(() => { expect(selector).toBeEnabled(); });
    expect(selector).toHaveValue('');
    expect(screen.getByRole('option', { name: 'No environment selected' })).toBeInTheDocument();
    expect(router.state.location.search).toBe('');
    // And nothing claims a scope on the screen.
    expect(screen.queryByRole('heading', { name: 'Current environment' })).toBeNull();
  });

  it('writes the parameter when the operator picks one', async () => {
    const { router } = await openHome('/');
    const selector = await screen.findByLabelText('Environment scope');
    await waitFor(() => { expect(selector).toBeEnabled(); });

    await userEvent.selectOptions(selector, SECOND_ID);

    await waitFor(() => { expect(router.state.location.search).toBe(`?env=${SECOND_ID}`); });
  });
});

describe('with exactly one environment', () => {
  it('preselects it and writes it to the URL', async () => {
    // Section 9.2.4 and test 10.1.4. There is no ambiguity to resolve, and a
    // link that resolves differently depending on how many environments the
    // reader can see is not a deterministic link.
    const { router } = await openHome('/', listing(environment()));

    await waitFor(() => {
      expect(router.state.location.search).toBe(`?env=${environmentID}`);
    });
    // The control's value is asserted inside `waitFor` as well, not just its
    // existence. The selector is in the tree before the scope resolves, so
    // `findByLabelText` returns immediately and the value can still be a render
    // behind the router state that was just awaited — which is exactly how this
    // failed intermittently under a full-suite run and never in isolation.
    const selector = await screen.findByLabelText('Environment scope');
    await waitFor(() => {
      expect(selector).toHaveValue(environmentID);
    });
  });
});

describe('the selector when there is nothing to choose', () => {
  it.each([
    ['the list is still loading', { 'GET /v1/environments?limit=200': () => new Promise<Response>(() => undefined) }, /Loading the environment list/],
    // A refusal rather than a 503: `freshness('configuration')` retries a
    // retryable read twice with backoff, so a 503 would still be in flight
    // when the assertion ran. What is under test is the disabled reason, not
    // the retry policy.
    ['the list was refused', { 'GET /v1/environments?limit=200': () => json({ error: 'forbidden' }, 403) }, /could not be read, so no scope can be chosen/],
    ['there are none', listing(), /No environments exist yet/],
  ] as const)('is disabled with a reason when %s', async (_name, handlers, reason) => {
    // Section 9.2.5 and WC-D07: disabled with a reason, never hidden.
    await openHome('/', handlers);

    // The reason, not the disabled state: the selector is disabled while
    // loading too, so waiting on `disabled` would pass before the list
    // resolved and assert against the loading message.
    const selector = await screen.findByLabelText('Environment scope');
    await waitFor(() => { expect(selector).toHaveAccessibleDescription(reason); });
    expect(selector).toBeDisabled();
  });
});

describe('a scope the operator cannot use', () => {
  it('renders not-found for a malformed value and looks nothing up', async () => {
    // Section 9.7.1 and test 10.1.2.
    const { api } = await openHome('/?env=not-a-uuid');

    expect(await screen.findByRole('heading', { name: 'Page not found', level: 1 })).toBeVisible();
    expect(screen.getByText(/deliberately not fallen back to a different environment/)).toBeVisible();
    // Nothing was requested with it: an invalid value never reaches a path.
    expect(api.calls.some((call) => call.key.includes('not-a-uuid'))).toBe(false);
  });

  it('renders denied for a forbidden environment without substituting another', async () => {
    await openHome(`/?env=${SECOND_ID}`, {
      ...listing(environment(), second()),
      [`GET /v1/environments/${SECOND_ID}`]: () => json({ error: 'forbidden' }, 403),
    });

    expect(await screen.findByText('Access was refused')).toBeVisible();
    // Section 8.3: the environment the operator *can* read is not shown here.
    const panel = screen.getByRole('region', { name: 'Current environment' });
    expect(within(panel).queryByText('Lab')).toBeNull();
  });

  it('renders not-found for an unknown environment without substituting another', async () => {
    await openHome(`/?env=${SECOND_ID}`, {
      ...listing(environment(), second()),
      [`GET /v1/environments/${SECOND_ID}`]: () => json({ error: 'not_found' }, 404),
    });

    expect(await screen.findByText('This data could not be loaded')).toBeVisible();
    const panel = screen.getByRole('region', { name: 'Current environment' });
    expect(within(panel).queryByText('Lab')).toBeNull();
  });
});

describe('changing scope', () => {
  it('refetches the scoped read once, under a key carrying the environment', async () => {
    // Test 10.1.11. The key change is what makes the refetch happen, and
    // "once" is what makes it a key change rather than a render loop.
    const { api, router } = await openHome(`/?env=${environmentID}`);
    await screen.findByRole('region', { name: 'Current environment' });
    await waitFor(() => {
      expect(api.called(`GET /v1/environments/${environmentID}`).length).toBeGreaterThanOrEqual(1);
    });
    const before = api.called(`GET /v1/environments/${environmentID}`).length;

    await userEvent.selectOptions(screen.getByLabelText('Environment scope'), SECOND_ID);

    await waitFor(() => { expect(router.state.location.search).toBe(`?env=${SECOND_ID}`); });
    await waitFor(() => { expect(api.called(`GET /v1/environments/${SECOND_ID}`)).toHaveLength(1); });
    // The previous environment is not read again: the key moved, it did not
    // fan out.
    expect(api.called(`GET /v1/environments/${environmentID}`)).toHaveLength(before);
  });

  it('reads the environment list once however many consumers ask', async () => {
    // Section 9.9. The selector and the list screen share one `queryOptions`
    // helper, so the list is fetched once per freshness interval.
    const { api } = await openHome('/environments');
    await screen.findByRole('heading', { name: 'Environments', level: 1 });

    await waitFor(() => {
      expect(api.called('GET /v1/environments?limit=200').length).toBeGreaterThanOrEqual(1);
    });
    expect(api.called('GET /v1/environments?limit=200')).toHaveLength(1);
  });
});
