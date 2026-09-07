import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter, Link, Route, Routes } from 'react-router-dom';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { RootErrorBoundary, RouteErrorBoundary } from './ErrorBoundary';
import { clearLastRenderError, readLastRenderError } from './lastRenderError';
import { expectNoAxeViolations } from '@shared/testing/axe';

/**
 * `P1-W11` GAP-2: no error boundary existed, so an unexpected exception
 * blanked the whole console. In a security product a blank page is
 * indistinguishable from "nothing is wrong", which is why the fallback must
 * appear — and why it must never repeat what the exception said.
 */
const SECRET = 'operator-token-9f3c2a1b leaked through an exception message';

function Explode(): never {
  throw new Error(SECRET);
}

beforeEach(() => {
  clearLastRenderError();
  // React reports a caught render error on the console. That is React's own
  // output, not a sink this package writes to, and it would otherwise drown
  // the run.
  vi.spyOn(console, 'error').mockImplementation(() => undefined);
});

afterEach(() => {
  vi.restoreAllMocks();
  clearLastRenderError();
});

describe('RootErrorBoundary', () => {
  it('renders the fixed fallback and never the thrown message', () => {
    render(<RootErrorBoundary><Explode /></RootErrorBoundary>);

    const alert = screen.getByRole('alert');
    expect(alert).toHaveTextContent('The console stopped unexpectedly');
    expect(screen.getByText('Guardian')).toBeVisible();
    expect(screen.getByRole('button', { name: 'Reload the console' })).toBeVisible();
    expect(document.body.innerHTML).not.toContain(SECRET);
    expect(document.body.innerHTML).not.toContain('operator-token-9f3c2a1b');
    expect(document.body.textContent).not.toMatch(/Error|stack|at Explode/);
  });

  it('keeps the children mounted while nothing throws', () => {
    render(<RootErrorBoundary><p>console content</p></RootErrorBoundary>);
    expect(screen.getByText('console content')).toBeVisible();
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('retains the last error in memory only, behind the WCX-15 interface', () => {
    render(<RootErrorBoundary><Explode /></RootErrorBoundary>);

    expect(readLastRenderError()?.message).toBe(SECRET);
    // In memory means in memory. Nothing reaches a storage area.
    expect(localStorage).toHaveLength(0);
    expect(sessionStorage).toHaveLength(0);
  });
});

function ShellHarness({ initial }: { initial: string }) {
  return (
    <MemoryRouter initialEntries={[initial]}>
      <nav aria-label="Primary navigation">
        <Link to="/environments">Environments</Link>
      </nav>
      <button type="button">Sign out</button>
      <main>
        <RouteErrorBoundary>
          <Routes>
            <Route path="/environments" element={<h1>Environment workspace</h1>} />
            <Route path="/broken" element={<Explode />} />
          </Routes>
        </RouteErrorBoundary>
      </main>
    </MemoryRouter>
  );
}

describe('RouteErrorBoundary', () => {
  it('keeps navigation and sign-out reachable while a screen is failing', () => {
    render(<ShellHarness initial="/broken" />);

    expect(screen.getByRole('alert')).toHaveTextContent('This screen stopped unexpectedly');
    expect(screen.getByRole('link', { name: 'Environments' })).toBeVisible();
    expect(screen.getByRole('button', { name: 'Sign out' })).toBeVisible();
    expect(document.body.innerHTML).not.toContain(SECRET);
  });

  it('resets when the operator navigates away from the failed screen', async () => {
    render(<ShellHarness initial="/broken" />);
    expect(screen.getByRole('alert')).toBeVisible();

    await userEvent.click(screen.getByRole('link', { name: 'Environments' }));

    expect(await screen.findByRole('heading', { name: 'Environment workspace' })).toBeVisible();
    expect(screen.queryByRole('alert')).toBeNull();
  });

  it('reports no serious or critical axe violation in either fallback', async () => {
    // A fallback is what an operator is left with when everything else failed,
    // so it is the last place an unreachable control is acceptable.
    const root = render(<RootErrorBoundary><Explode /></RootErrorBoundary>);
    await expectNoAxeViolations(root.container);
    root.unmount();

    const route = render(<ShellHarness initial="/broken" />);
    await expectNoAxeViolations(route.container);
  });

  it('does not leave the document without its language or theme', () => {
    // The fallback renders inside the tree rather than replacing the document,
    // so `WCX-03`'s stylesheet and the document language survive a failure.
    document.documentElement.lang = 'en';
    render(<RootErrorBoundary><Explode /></RootErrorBoundary>);
    expect(document.documentElement.lang).toBe('en');
    expect(document.documentElement.isConnected).toBe(true);
  });
});
