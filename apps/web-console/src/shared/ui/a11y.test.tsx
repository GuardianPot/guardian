import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { ExerciseEveryComponent } from '@shared/testing/exerciseUi';
import { advisoryAxeFindings, expectNoAxeViolations } from '@shared/testing/axe';

/**
 * The whole shared layer, scanned (WCX-05 sections 9.4 and 10.1.1).
 *
 * `storage.test.tsx` already proves this page renders every component the
 * layer exports, so scanning it here is a scan of every shared component
 * rather than of a sample. The individual component tests scan their own
 * markup too; this catches what only appears when they are composed — a
 * duplicated landmark name, a heading order that skips a level, an id
 * collision between two instances of the same control.
 */
describe('the shared layer', () => {
  it('reports no serious or critical violation in its resting state', async () => {
    const { baseElement } = render(<ExerciseEveryComponent text="edge-one" />);
    await expectNoAxeViolations(baseElement);
  });

  it('reports no serious or critical violation with a toast showing', async () => {
    const { baseElement } = render(<ExerciseEveryComponent text="edge-one" />);
    await userEvent.click(screen.getByRole('button', { name: 'Show a confirmation' }));
    await expectNoAxeViolations(baseElement);
  });

  it('reports no serious or critical violation with a dialog open', async () => {
    const { baseElement } = render(<ExerciseEveryComponent text="edge-one" />);
    await userEvent.click(screen.getByRole('button', { name: 'Open the dialog' }));
    await screen.findByRole('dialog');
    await expectNoAxeViolations(baseElement);
  });

  it('reports no serious or critical violation with a level 3 confirmation open', async () => {
    const { baseElement } = render(<ExerciseEveryComponent text="edge-one" />);
    await userEvent.click(screen.getByRole('button', { name: 'Revoke the device' }));
    await screen.findByRole('dialog');
    await expectNoAxeViolations(baseElement);
  });

  it('reports no serious or critical violation when every string is hostile', async () => {
    // A hostile display name must not be able to turn a valid name into an
    // invalid one — an unclosed tag swallowing a label, for instance.
    const { baseElement } = render(<ExerciseEveryComponent text='"><img src=x onerror=alert(1)>' />);
    await expectNoAxeViolations(baseElement);
  });

  it('leaves the advisory findings visible rather than silent', () => {
    // Section 9.4: moderate and minor findings are reported, not failed on.
    // They are legitimate here — a component rendered outside a page has no
    // landmark to sit in and no `h1` above it — but a new one appearing is
    // worth seeing, so the list is printed rather than discarded.
    const findings = advisoryAxeFindings();
    if (findings.length > 0) {
      // The reporting channel section 9.4 asks for: visible in the run, not a
      // build break.
      console.info(`axe advisory: ${findings.map((finding) => `${finding.id}/${finding.impact ?? '?'}`).join(', ')}`);
    }
    expect(findings.every((finding) => finding.impact !== 'serious' && finding.impact !== 'critical')).toBe(true);
  });
});
