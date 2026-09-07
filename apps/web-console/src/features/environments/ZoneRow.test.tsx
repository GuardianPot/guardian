import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { csrfToken, environmentID, json, loginHandlers, mockApi, renderRoute, zone } from '@shared/testing/harness';
import { taintZone } from '@shared/api/taint';
import type { StepUp } from '@features/auth';
import { ZoneRow } from './ZoneRow';

/**
 * Zone edit and delete (WCX-09 sections 9.4, 9.8.1, tests 10.1.6 and 10.1.9).
 *
 * The conflict is the interesting case. Both mutations carry the revision the
 * operator was looking at, so a `412` means someone else changed the zone
 * first — and a console that retried without the header would overwrite a
 * change it never showed anyone.
 */
const ZONE_ID = zone().zone_id;
const ZONE_PATH = `/v1/environments/${environmentID}/zones/${ZONE_ID}`;

/** Level 2 delete needs no step-up, so a refusing seam is the honest stub. */
const noStepUp: StepUp = {
  request: () => Promise.resolve({ satisfied: false, reason: 'not-implemented' }),
  consume: () => false,
  element: null,
};

async function openRow(overrides: Parameters<typeof zone>[0] = {}, extra = {}) {
  const api = mockApi({ ...loginHandlers(), ...extra });
  const view = renderRoute(
    <ul><ZoneRow zone={taintZone(zone(overrides))} stepUp={noStepUp} /></ul>,
    { path: '/environments/:environmentId', entry: `/environments/${environmentID}` },
  );
  // Matched by prefix: a hostile fixture renames the row, and the helper
  // should wait for the row rather than for one particular name.
  await screen.findByRole('button', { name: /^Edit / });
  return { api, ...view };
}

describe('editing a zone', () => {
  it('sends the revision the operator was looking at', async () => {
    // Optimistic concurrency is the whole point: without `If-Match` a save
    // would silently overwrite whatever arrived in between.
    const { api } = await openRow({}, {
      [`PATCH ${ZONE_PATH}`]: () => json({ zone: zone({ display_name: 'Lab zone renamed', revision: 2 }) }),
    });

    await userEvent.click(screen.getByRole('button', { name: 'Edit Lab zone' }));
    const name = await screen.findByLabelText('Zone name');
    await userEvent.clear(name);
    await userEvent.type(name, 'Lab zone renamed');
    await userEvent.click(screen.getByRole('button', { name: 'Save zone' }));

    await waitFor(() => { expect(api.called(`PATCH ${ZONE_PATH}`)).toHaveLength(1); });
    expect(api.header(`PATCH ${ZONE_PATH}`, 'If-Match')).toBe('"1"');
    expect(api.header(`PATCH ${ZONE_PATH}`, 'X-CSRF-Token')).toBe(csrfToken);
    expect(api.called(`PATCH ${ZONE_PATH}`)[0]?.body).toEqual({
      display_name: 'Lab zone renamed',
      cidr: '10.20.0.0/24',
    });
  });

  it('renders a conflict as a conflict, not as a validation failure', async () => {
    // Test 10.1.6. A `412` means the value was fine and someone was faster;
    // presenting it as invalid input would send the operator to fix the wrong
    // thing.
    await openRow({}, { [`PATCH ${ZONE_PATH}`]: () => json({ error: 'precondition_failed' }, 412) });

    await userEvent.click(screen.getByRole('button', { name: 'Edit Lab zone' }));
    await userEvent.click(await screen.findByRole('button', { name: 'Save zone' }));

    const alert = await screen.findByRole('alert');
    expect(alert).toHaveTextContent(/Another change reached this zone first, so nothing was written/);
    expect(alert).not.toHaveTextContent(/RFC1918|canonical/);
    // And it offers to take the stored value rather than retrying blindly.
    expect(screen.getByRole('button', { name: 'Reload the current value' })).toBeVisible();
  });

  it('distinguishes a rejected value from a conflict', async () => {
    await openRow({}, { [`PATCH ${ZONE_PATH}`]: () => json({ error: 'invalid_request' }, 400) });

    await userEvent.click(screen.getByRole('button', { name: 'Edit Lab zone' }));
    await userEvent.click(await screen.findByRole('button', { name: 'Save zone' }));

    expect(await screen.findByText(/Nothing changed\. Use a canonical, non-overlapping RFC1918 CIDR\./)).toBeVisible();
    expect(screen.queryByRole('button', { name: 'Reload the current value' })).toBeNull();
  });
});

describe('deleting a zone', () => {
  it('confirms at level 2 and sends the revision', async () => {
    const { api } = await openRow({}, {
      [`DELETE ${ZONE_PATH}`]: () => new Response(null, { status: 204 }),
    });

    await userEvent.click(screen.getByRole('button', { name: 'Delete Lab zone' }));
    // No step-up: delete is recoverable, so it is L2.
    expect(screen.queryByRole('dialog', { name: 'Confirm it is you' })).toBeNull();
    await userEvent.click(await screen.findByRole('button', { name: 'Delete zone' }));

    await waitFor(() => { expect(api.called(`DELETE ${ZONE_PATH}`)).toHaveLength(1); });
    expect(api.header(`DELETE ${ZONE_PATH}`, 'If-Match')).toBe('"1"');
  });

  it('issues nothing when the confirmation is cancelled', async () => {
    const { api } = await openRow();
    await userEvent.click(screen.getByRole('button', { name: 'Delete Lab zone' }));
    await screen.findByRole('dialog');
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }));

    expect(api.called(`DELETE ${ZONE_PATH}`)).toHaveLength(0);
  });

  it('says nothing changed when the delete fails', async () => {
    await openRow({}, { [`DELETE ${ZONE_PATH}`]: () => json({ error: 'unavailable' }, 503) });

    await userEvent.click(screen.getByRole('button', { name: 'Delete Lab zone' }));
    await userEvent.click(await screen.findByRole('button', { name: 'Delete zone' }));

    expect(await screen.findByText('The zone could not be deleted. Nothing changed.')).toBeVisible();
  });

  it('renders a hostile zone name inert in the confirmation', async () => {
    // Test 10.1.9. The name reaches the modal and the typed prompt.
    const hostile = '<img src=x onerror=alert(1)>';
    const { baseElement } = await openRow({ display_name: hostile });

    await userEvent.click(screen.getByRole('button', { name: `Delete ${hostile}` }));
    const dialog = await screen.findByRole('dialog');

    expect(dialog).toHaveTextContent(hostile);
    expect(baseElement.querySelector('img')).toBeNull();
  });
});
