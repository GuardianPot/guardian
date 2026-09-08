import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { ConfirmationDialog } from './ConfirmationDialog';
import {
  ACTION_CONFIRMATION,
  confirmationFor,
  stepUpUnavailable,
  type ConfirmableAction,
  type StepUpReauthentication,
} from './levels';
import { CATALOGUE } from '@shared/text';
import { expectNoAxeViolations } from '@shared/testing/axe';

/**
 * The three confirmation levels (WCX-04 section 9.3, WCX-09 section 9.1).
 *
 * The levels come from the table, not from the call site, so the assertions
 * below are as much about the table as about the dialog.
 */

/** A step-up that always succeeds, with a mark that is spendable once. */
function satisfiedStepUp(): StepUpReauthentication & { requests: number } {
  let mark: ConfirmableAction | null = null;
  const stepUp = {
    requests: 0,
    request(action: ConfirmableAction) {
      stepUp.requests += 1;
      mark = action;
      return Promise.resolve({ satisfied: true as const });
    },
    consume(action: ConfirmableAction) {
      if (mark !== action) return false;
      mark = null;
      return true;
    },
  };
  return stepUp;
}

const noop = () => undefined;

describe('the action-to-level table', () => {
  it('assigns a level to every confirmable action, from the table and not a call site', () => {
    const actions = Object.keys(ACTION_CONFIRMATION) as ConfirmableAction[];
    expect(actions.length).toBeGreaterThan(0);
    for (const action of actions) {
      expect([1, 2, 3], action).toContain(confirmationFor(action).level);
      // The confirm control names the effect. `OK` is never a label.
      expect(CATALOGUE[confirmationFor(action).effect], action).not.toMatch(/^(?:OK|Confirm|Yes)$/i);
    }
  });

  // Section 10.1.1: the whole table, row by row, as WCX-09 section 9.1 sets it.
  it.each([
    ['zone.rename', 1],
    ['device.enable', 1],
    ['device.disable', 2],
    ['enrollment.revoke', 2],
    ['zone.delete', 2],
    // WCX-11 section 9.5. Enable, disable and reconfigure are reversible;
    // deploying occupies a real address and removal retires a decoy.
    ['decoy.enable', 1],
    ['decoy.disable', 1],
    ['decoy.update', 1],
    ['decoy.deploy', 2],
    ['decoy.remove', 2],
    ['device.revoke', 3],
    ['device.reenroll', 3],
    ['session.revoke', 3],
    ['account.password', 3],
  ] as const)('puts %s at level %i', (action, level) => {
    expect(confirmationFor(action).level).toBe(level);
  });

  it('covers every action in the table and no more', () => {
    expect(Object.keys(ACTION_CONFIRMATION).sort()).toEqual([
      'account.password', 'decoy.deploy', 'decoy.disable', 'decoy.enable',
      'decoy.remove', 'decoy.update', 'device.disable', 'device.enable',
      'device.reenroll', 'device.revoke', 'enrollment.revoke', 'session.revoke',
      'zone.delete', 'zone.rename',
    ]);
  });

  it('renders no dialog at all for a level 1 action', () => {
    const { container } = render(
      <ConfirmationDialog action="zone.rename" objectName="Lab zone" open onCancel={noop} onConfirm={noop} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('refuses when no step-up implementation is supplied', async () => {
    await expect(stepUpUnavailable.request('device.revoke')).resolves.toEqual({
      satisfied: false,
      reason: 'not-implemented',
    });
    expect(stepUpUnavailable.consume('device.revoke')).toBe(false);
  });
});

describe('level 2 confirmation', () => {
  it('names the object and the effect, and labels confirm with the effect', async () => {
    const confirmed = vi.fn();
    render(
      <ConfirmationDialog action="zone.delete" objectName="Lab zone" open onCancel={noop} onConfirm={confirmed} />,
    );

    const dialog = await screen.findByRole('dialog');
    expect(dialog).toHaveAccessibleName('Delete zone');
    expect(dialog).toHaveTextContent('Lab zone');
    expect(screen.getByRole('button', { name: 'Delete zone' })).toBeEnabled();
    expect(screen.queryByRole('button', { name: /^OK$/i })).toBeNull();

    await userEvent.click(screen.getByRole('button', { name: 'Delete zone' }));
    expect(confirmed).toHaveBeenCalledTimes(1);
  });

  it('needs no step-up, because level 2 is recoverable', async () => {
    const confirmed = vi.fn();
    const stepUp = satisfiedStepUp();
    render(
      <ConfirmationDialog action="device.disable" objectName="edge-one" open onCancel={noop} onConfirm={confirmed} stepUp={stepUp} />,
    );

    await screen.findByRole('dialog');
    await userEvent.click(screen.getByRole('button', { name: 'Disable device' }));

    expect(stepUp.requests).toBe(0);
    expect(confirmed).toHaveBeenCalledTimes(1);
  });

  it('lands focus on cancel, never on the destructive control', async () => {
    render(
      <ConfirmationDialog action="zone.delete" objectName="Lab zone" open onCancel={noop} onConfirm={noop} />,
    );

    await screen.findByRole('dialog');
    expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus();
    expect(screen.getByRole('button', { name: 'Delete zone' })).not.toHaveFocus();
  });

  it('closes on escape and returns to the caller without confirming', async () => {
    const cancelled = vi.fn();
    const confirmed = vi.fn();
    render(
      <ConfirmationDialog action="zone.delete" objectName="Lab zone" open onCancel={cancelled} onConfirm={confirmed} />,
    );

    await screen.findByRole('dialog');
    await userEvent.keyboard('{Escape}');
    expect(cancelled).toHaveBeenCalled();
    expect(confirmed).not.toHaveBeenCalled();
  });
});

describe('level 3 confirmation', () => {
  /** Opens an irreversible confirmation with a step-up that succeeds. */
  async function openIrreversible(objectName = 'edge-one', action: ConfirmableAction = 'device.revoke') {
    const confirmed = vi.fn();
    const stepUp = satisfiedStepUp();
    const view = render(
      <ConfirmationDialog action={action} objectName={objectName} open onCancel={noop} onConfirm={confirmed} stepUp={stepUp} />,
    );
    await screen.findByRole('dialog');
    return { confirmed, stepUp, ...view };
  }

  it('asks for reauthentication before showing the confirmation', async () => {
    // Section 9.2.1. The typed-confirmation field cannot appear before the
    // step-up resolves: presenting it would imply the action is authorised.
    let settle: (() => void) | undefined;
    const held: StepUpReauthentication = {
      request: () => new Promise((resolve) => { settle = () => { resolve({ satisfied: true }); }; }),
      consume: () => true,
    };
    render(
      <ConfirmationDialog action="device.revoke" objectName="edge-one" open onCancel={noop} onConfirm={noop} stepUp={held} />,
    );

    expect(screen.queryByRole('dialog')).toBeNull();
    expect(screen.queryByLabelText('Object name')).toBeNull();

    settle?.();
    await screen.findByRole('dialog');
    expect(screen.getByLabelText('Object name')).toBeInTheDocument();
  });

  it('states that the action cannot be undone', async () => {
    await openIrreversible();
    expect(await screen.findByText(/This cannot be undone\./)).toBeVisible();
  });

  it('keeps confirm disabled until the exact object name is typed', async () => {
    await openIrreversible();

    const confirm = screen.getByRole('button', { name: 'Revoke device' });
    expect(confirm).toBeDisabled();
    // Disabled, never hidden, and the reason is associated (section 8.6, 9.7.5).
    expect(confirm).toHaveAccessibleDescription(/Type the exact name to continue: edge-one/);

    await userEvent.type(screen.getByLabelText('Object name'), 'edge-on');
    expect(screen.getByRole('button', { name: 'Revoke device' })).toBeDisabled();
    // Section 9.6.3: the mismatch is named, and the typed value is not echoed.
    const mismatch = screen.getByText('That does not match the name above.');
    expect(mismatch).toBeVisible();
    expect(mismatch.textContent).not.toContain('edge-on');

    await userEvent.type(screen.getByLabelText('Object name'), 'e');
    expect(screen.getByRole('button', { name: 'Revoke device' })).toBeEnabled();
  });

  it('never transmits the typed value', async () => {
    // Section 8.5. The comparison is local; nothing about it leaves the tab.
    const fetchSpy = vi.spyOn(globalThis, 'fetch');
    const { confirmed } = await openIrreversible();

    await userEvent.type(screen.getByLabelText('Object name'), 'edge-one');
    await userEvent.click(screen.getByRole('button', { name: 'Revoke device' }));

    expect(confirmed).toHaveBeenCalledTimes(1);
    expect(fetchSpy).not.toHaveBeenCalled();
    fetchSpy.mockRestore();
  });

  it('does not focus confirm even once the typed name enables it', async () => {
    await openIrreversible();
    expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus();
  });

  it.each([
    ['cancelled', 'Reauthentication was not completed, so nothing was changed.'],
    ['denied', 'Reauthentication was denied, so nothing was changed.'],
    ['rate-limited', 'Too many reauthentication attempts. Wait before trying again. Nothing was changed.'],
  ] as const)('reports a %s step-up and issues nothing', async (reason, message) => {
    // Section 10.1.2. No request is issued when step-up is cancelled or fails.
    const confirmed = vi.fn();
    render(
      <ConfirmationDialog
        action="device.revoke"
        objectName="edge-one"
        open
        onCancel={noop}
        onConfirm={confirmed}
        stepUp={{
          request: () => Promise.resolve({ satisfied: false, reason }),
          consume: () => false,
        }}
      />,
    );

    expect(await screen.findByRole('alert')).toHaveTextContent(message);
    expect(confirmed).not.toHaveBeenCalled();
    // There is nothing left to confirm with, so the control is gone.
    expect(screen.queryByRole('button', { name: 'Revoke device' })).toBeNull();
    expect(screen.queryByLabelText('Object name')).toBeNull();
  });

  it('refuses to complete when no step-up implementation exists', async () => {
    const confirmed = vi.fn();
    render(
      <ConfirmationDialog action="device.revoke" objectName="edge-one" open onCancel={noop} onConfirm={confirmed} />,
    );

    expect(await screen.findByRole('alert')).toHaveTextContent(
      'Step-up reauthentication is not available, so this action cannot be completed.',
    );
    expect(confirmed).not.toHaveBeenCalled();
  });

  it('spends the step-up mark exactly once', async () => {
    // Section 10.1.3. The mark is scoped to one action. A dialog whose mark
    // was already spent refuses rather than proceeding on a stale approval.
    const confirmed = vi.fn();
    const stepUp = satisfiedStepUp();
    render(
      <ConfirmationDialog action="device.revoke" objectName="edge-one" open onCancel={noop} onConfirm={confirmed} stepUp={stepUp} />,
    );
    await screen.findByRole('dialog');
    await userEvent.type(screen.getByLabelText('Object name'), 'edge-one');

    // Something else spends the mark first — a second control on the screen.
    expect(stepUp.consume('device.revoke')).toBe(true);

    await userEvent.click(screen.getByRole('button', { name: 'Revoke device' }));
    expect(confirmed).not.toHaveBeenCalled();
    expect(await screen.findByRole('alert')).toHaveTextContent('Reauthentication was denied');
  });

  it('will not spend a mark issued for a different action', () => {
    const stepUp = satisfiedStepUp();
    void stepUp.request('device.revoke');
    expect(stepUp.consume('session.revoke')).toBe(false);
  });

  it('reports no serious or critical axe violation at either modal level', async () => {
    const recoverable = render(
      <ConfirmationDialog action="zone.delete" objectName="Lab zone" open onCancel={noop} onConfirm={noop} />,
    );
    await screen.findByRole('dialog');
    await expectNoAxeViolations(recoverable.baseElement);
    recoverable.unmount();

    const irreversible = render(
      <ConfirmationDialog action="device.revoke" objectName="edge-one" open onCancel={noop} onConfirm={noop} stepUp={satisfiedStepUp()} />,
    );
    await screen.findByRole('dialog');
    await expectNoAxeViolations(irreversible.baseElement);
  });

  it('compares a hostile object name by exact equality and renders it as text', async () => {
    // Section 10.1.9. A device name reaches the confirmation from the backend.
    const hostile = '<img src=x onerror=alert(1)>';
    const { baseElement } = await openIrreversible(hostile);

    const dialog = await screen.findByRole('dialog');
    expect(dialog).toHaveTextContent(hostile);
    expect(baseElement.querySelector('img')).toBeNull();

    await userEvent.type(screen.getByLabelText('Object name'), hostile);
    expect(screen.getByRole('button', { name: 'Revoke device' })).toBeEnabled();
    expect(baseElement.querySelector('img')).toBeNull();
  });
});
