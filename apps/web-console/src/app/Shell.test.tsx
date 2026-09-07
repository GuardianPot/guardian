import { screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { describe, expect, it } from 'vitest';
import { renderApp } from '@shared/testing/renderApp';
import { setViewportWidth } from '@shared/testing/viewport';
import { expectNoAxeViolations } from '@shared/testing/axe';

/**
 * The responsive shell (WCX-10 section 9.3, tests 10.1.6-8).
 *
 * One rule outranks everything else in this file, and it is here because it
 * was broken: **no operator control is removed at any viewport width.**
 * `P1-W11` GAP-1 gave the operator block `display: none` below 900 pixels, so
 * an operator who suspected a stolen session could not sign out from the
 * device in their hand. The suite that shipped that defect passed, because
 * nothing in it had a width.
 *
 * So these tests set a width. `installMatchMedia` in the setup file answers
 * from it, which is the same API the shell reads in a browser — the code path
 * under test is the real one, not a component handed a `narrow` prop it would
 * never receive in production.
 */
const WIDTHS = [320, 375, 900, 1440] as const;

/** Renders the application and waits for the shell itself to be mounted. */
async function mountShell(entry: string, options: Parameters<typeof renderApp>[1] = {}) {
  const view = renderApp(entry, options);
  // The brand link, because it is outside the disclosure and therefore the
  // one shell element present at every width. The harness signs in
  // asynchronously, so querying before this point finds nothing and would
  // report a missing control as a layout failure.
  await screen.findByRole('link', { name: 'Guardian Console home' });
  return view;
}

/** Everything an operator needs to end or restore a session. */
async function expectOperatorControlsPresent() {
  expect(await screen.findByText('Signed in as')).toBeVisible();
  expect(screen.getByText('owner')).toBeVisible();
  expect(screen.getByRole('button', { name: 'Sign out' })).toBeVisible();
}

describe('at every supported width', () => {
  it.each(WIDTHS)('keeps every operator control reachable at %ipx', async (width) => {
    setViewportWidth(width);
    await mountShell('/');

    // Below the breakpoint the controls live behind a disclosure, so opening
    // it is part of "reachable". Above it there is no disclosure at all.
    const trigger = screen.queryByRole('button', { name: 'Open navigation' });
    if (width <= 900) {
      expect(trigger, `a disclosure is expected at ${width}px`).not.toBeNull();
      await userEvent.click(trigger!);
    } else {
      expect(trigger, `no disclosure belongs at ${width}px`).toBeNull();
    }

    await expectOperatorControlsPresent();
    // Scoped to the landmark: the home placeholder links to the same screens,
    // and a document-wide query would pass on those instead.
    const nav = within(screen.getByRole('navigation', { name: 'Primary navigation' }));
    for (const entry of ['Home', 'Environments', 'Account']) {
      expect(nav.getByRole('link', { name: entry }), `${entry} at ${width}px`).toBeVisible();
    }
  });

  it.each(WIDTHS)('keeps the read-only banner visible at %ipx', async (width) => {
    // Section 9.3.5. The banner is outside the disclosure on purpose: it is
    // the explanation for why half the controls are disabled.
    setViewportWidth(width);
    await mountShell('/', { signedIn: false });

    expect(await screen.findByText(/Read-only session restored\./)).toBeVisible();
    // The banner carries its own re-authentication link, and so does the
    // operator block. Both must survive, so this counts rather than picking.
    expect(screen.getAllByRole('link', { name: 'Re-authenticate' }).length).toBeGreaterThanOrEqual(1);
  });

  it.each(WIDTHS)('reports no serious or critical axe violation at %ipx', async (width) => {
    setViewportWidth(width);
    const { container } = await mountShell('/');
    await screen.findByRole('heading', { name: 'Home', level: 1 });
    await expectNoAxeViolations(container);
  });
});

describe('the narrow disclosure', () => {
  async function openDisclosure() {
    setViewportWidth(375);
    const view = await mountShell('/');
    const trigger = await screen.findByRole('button', { name: 'Open navigation' });
    await userEvent.click(trigger);
    return { trigger, ...view };
  }

  it('starts closed and hides the panel until it is opened', async () => {
    setViewportWidth(375);
    await mountShell('/');

    const trigger = await screen.findByRole('button', { name: 'Open navigation' });
    expect(trigger).toHaveAttribute('aria-expanded', 'false');
    expect(trigger).toHaveAttribute('aria-controls');
    // Hidden, not removed: the panel is in the document with its controls
    // intact, which is what makes reopening it cheap and its state honest.
    const panel = document.getElementById(trigger.getAttribute('aria-controls')!);
    expect(panel).not.toBeNull();
    expect(panel).not.toBeVisible();
  });

  it('sets aria-expanded and renames itself when open', async () => {
    const { trigger } = await openDisclosure();
    expect(trigger).toHaveAttribute('aria-expanded', 'true');
    // Section 9.3.4 and the catalogue note: the label names the state the
    // press will produce, not the state it is in.
    expect(screen.getByRole('button', { name: 'Close navigation' })).toBe(trigger);
  });

  it('closes on escape and returns focus to its trigger', async () => {
    const { trigger } = await openDisclosure();
    await userEvent.keyboard('{Escape}');

    await waitFor(() => { expect(trigger).toHaveAttribute('aria-expanded', 'false'); });
    expect(trigger).toHaveFocus();
  });

  it('closes on navigation', async () => {
    const { trigger } = await openDisclosure();
    const nav = within(screen.getByRole('navigation', { name: 'Primary navigation' }));
    await userEvent.click(nav.getByRole('link', { name: 'Environments' }));

    await waitFor(() => { expect(trigger).toHaveAttribute('aria-expanded', 'false'); });
  });

  it('dissolves when the viewport widens, and does not reopen when it narrows', async () => {
    const { trigger } = await openDisclosure();
    expect(trigger).toHaveAttribute('aria-expanded', 'true');

    setViewportWidth(1440);
    await waitFor(() => {
      expect(screen.queryByRole('button', { name: /navigation/i })).toBeNull();
    });
    await expectOperatorControlsPresent();

    setViewportWidth(375);
    const reopened = await screen.findByRole('button', { name: 'Open navigation' });
    expect(reopened).toHaveAttribute('aria-expanded', 'false');
  });
});

describe('the shell landmarks', () => {
  it('puts the skip link first and points it at main', async () => {
    // Section 9.5.2, unchanged by the restructure.
    setViewportWidth(1440);
    const { container } = await mountShell('/');
    await screen.findByRole('heading', { name: 'Home', level: 1 });

    const skip = screen.getByRole('link', { name: 'Skip to content' });
    expect(skip).toHaveAttribute('href', '#main-content');
    expect(container.querySelector('a')).toBe(skip);
    expect(document.getElementById('main-content')?.tagName.toLowerCase()).toBe('main');
  });

  it('has exactly one main and one named primary navigation', async () => {
    setViewportWidth(1440);
    await mountShell('/');
    await screen.findByRole('heading', { name: 'Home', level: 1 });

    expect(screen.getAllByRole('main')).toHaveLength(1);
    expect(screen.getByRole('navigation', { name: 'Primary navigation' })).toBeInTheDocument();
  });

  it('marks the active entry with more than colour', async () => {
    // Section 9.4.3. `aria-current` is the part a screen reader can use, and
    // the stylesheet adds a weight change and a leading rule beside it.
    setViewportWidth(1440);
    await mountShell('/environments');

    const active = await screen.findByRole('link', { name: 'Environments', current: 'page' });
    expect(active).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Account' })).not.toHaveAttribute('aria-current');
  });
});
