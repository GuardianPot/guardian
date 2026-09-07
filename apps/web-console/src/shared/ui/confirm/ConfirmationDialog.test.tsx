import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it, vi } from 'vitest';
import { ConfirmationDialog } from './ConfirmationDialog';
import { ACTION_CONFIRMATION, confirmationFor, stepUpUnavailable, type ConfirmableAction } from './levels';

/**
 * The three confirmation levels (WCX-04 section 9.3, WC-D16).
 *
 * The levels come from the table, not from the call site, so the assertions
 * below are as much about the table as about the dialog.
 */
describe('the action-to-level table', () => {
  it('assigns a level to every confirmable action, from the table and not a call site', () => {
    const actions = Object.keys(ACTION_CONFIRMATION) as ConfirmableAction[];
    expect(actions.length).toBeGreaterThan(0);
    for (const action of actions) {
      expect([1, 2, 3], action).toContain(confirmationFor(action).level);
      // The confirm control names the effect. `OK` is never a label.
      expect(confirmationFor(action).effect, action).not.toMatch(/^(?:OK|Confirm|Yes)$/i);
    }
  });

  it('places every irreversible security action at level 3', () => {
    for (const action of ['device.revoke', 'session.revoke', 'account.password'] as const) {
      expect(confirmationFor(action).level, action).toBe(3);
    }
  });

  it('places destructive but recoverable actions at level 2', () => {
    for (const action of ['zone.delete', 'enrollment.revoke'] as const) {
      expect(confirmationFor(action).level, action).toBe(2);
    }
  });

  it('places reversible actions at level 1, with no confirmation at all', () => {
    for (const action of ['device.enable', 'device.disable'] as const) {
      expect(confirmationFor(action).level, action).toBe(1);
    }
    const { container } = render(
      <ConfirmationDialog action="device.disable" objectName="edge-one" open onCancel={() => undefined} onConfirm={() => undefined} />,
    );
    expect(container).toBeEmptyDOMElement();
  });

  it('keeps step-up reauthentication an interface only until WCX-09 implements it', async () => {
    await expect(stepUpUnavailable('device.revoke')).resolves.toEqual({
      satisfied: false,
      reason: 'not-implemented',
    });
  });
});

describe('level 2 confirmation', () => {
  it('names the object and the effect, and labels confirm with the effect', async () => {
    const confirmed = vi.fn();
    render(
      <ConfirmationDialog action="zone.delete" objectName="Lab zone" open onCancel={() => undefined} onConfirm={confirmed} />,
    );

    const dialog = await screen.findByRole('dialog');
    expect(dialog).toHaveAccessibleName('Delete zone');
    expect(dialog).toHaveTextContent('Lab zone');
    expect(screen.getByRole('button', { name: 'Delete zone' })).toBeEnabled();
    expect(screen.queryByRole('button', { name: /^OK$/i })).toBeNull();

    await userEvent.click(screen.getByRole('button', { name: 'Delete zone' }));
    expect(confirmed).toHaveBeenCalledTimes(1);
  });

  it('lands focus on cancel, never on the destructive control', async () => {
    render(
      <ConfirmationDialog action="zone.delete" objectName="Lab zone" open onCancel={() => undefined} onConfirm={() => undefined} />,
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
  it('states that the action cannot be undone', async () => {
    render(
      <ConfirmationDialog action="device.revoke" objectName="edge-one" open onCancel={() => undefined} onConfirm={() => undefined} />,
    );
    expect(await screen.findByText(/This cannot be undone\./)).toBeVisible();
  });

  it('keeps confirm disabled until the exact object name is typed', async () => {
    render(
      <ConfirmationDialog action="device.revoke" objectName="edge-one" open onCancel={() => undefined} onConfirm={() => undefined} />,
    );

    await screen.findByRole('dialog');
    const confirm = screen.getByRole('button', { name: 'Revoke device' });
    expect(confirm).toBeDisabled();
    // Disabled, never hidden, and the reason is associated (section 8.6, 9.7.5).
    expect(confirm).toHaveAccessibleDescription(/Type the exact name to continue: edge-one/);

    await userEvent.type(screen.getByLabelText('Object name'), 'edge-on');
    expect(screen.getByRole('button', { name: 'Revoke device' })).toBeDisabled();

    await userEvent.type(screen.getByLabelText('Object name'), 'e');
    expect(screen.getByRole('button', { name: 'Revoke device' })).toBeEnabled();
  });

  it('does not focus confirm even once the typed name enables it', async () => {
    render(
      <ConfirmationDialog action="device.revoke" objectName="edge-one" open onCancel={() => undefined} onConfirm={() => undefined} />,
    );
    await screen.findByRole('dialog');
    expect(screen.getByRole('button', { name: 'Cancel' })).toHaveFocus();
  });

  it('refuses to complete while step-up reauthentication is unimplemented', async () => {
    const confirmed = vi.fn();
    render(
      <ConfirmationDialog action="device.revoke" objectName="edge-one" open onCancel={() => undefined} onConfirm={confirmed} />,
    );

    await screen.findByRole('dialog');
    await userEvent.type(screen.getByLabelText('Object name'), 'edge-one');
    await userEvent.click(screen.getByRole('button', { name: 'Revoke device' }));

    expect(confirmed).not.toHaveBeenCalled();
    expect(await screen.findByRole('alert')).toHaveTextContent(/Step-up reauthentication is required/);
  });

  it('completes once WCX-09 supplies a satisfied step-up', async () => {
    const confirmed = vi.fn();
    render(
      <ConfirmationDialog
        action="device.revoke"
        objectName="edge-one"
        open
        onCancel={() => undefined}
        onConfirm={confirmed}
        stepUp={() => Promise.resolve({ satisfied: true })}
      />,
    );

    await screen.findByRole('dialog');
    await userEvent.type(screen.getByLabelText('Object name'), 'edge-one');
    await userEvent.click(screen.getByRole('button', { name: 'Revoke device' }));

    expect(confirmed).toHaveBeenCalledTimes(1);
  });

  it('compares a hostile object name by exact equality and renders it as text', async () => {
    const hostile = '<img src=x onerror=alert(1)>';
    const { baseElement } = render(
      <ConfirmationDialog action="device.revoke" objectName={hostile} open onCancel={() => undefined} onConfirm={() => undefined} />,
    );

    const dialog = await screen.findByRole('dialog');
    expect(dialog).toHaveTextContent(hostile);
    expect(baseElement.querySelector('img')).toBeNull();

    await userEvent.type(screen.getByLabelText('Object name'), hostile);
    expect(screen.getByRole('button', { name: 'Revoke device' })).toBeEnabled();
    expect(baseElement.querySelector('img')).toBeNull();
  });
});
