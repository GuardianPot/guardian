import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { EXERCISED_COMPONENTS, ExerciseEveryComponent } from '@shared/testing/exerciseUi';
import * as layer from './index';

/**
 * Section 8.7 and standing constraint 2: no component may place any value into
 * browser storage.
 *
 * The console's session proof is memory-only by design, and a component that
 * quietly cached a display name, a filter, or a "last seen" marker would put
 * operator data on disk where the threat model says nothing is written. This
 * asserts both halves: the storage areas end empty, and the write paths were
 * never called — a value written and deleted again would pass the first check
 * alone.
 */
const setItem = vi.spyOn(Storage.prototype, 'setItem');
const removeItem = vi.spyOn(Storage.prototype, 'removeItem');
const indexedDBOpen = vi.fn();

beforeEach(() => {
  localStorage.clear();
  sessionStorage.clear();
  setItem.mockClear();
  removeItem.mockClear();
  indexedDBOpen.mockClear();
  // jsdom implements no IndexedDB, so a component reaching for one would throw
  // rather than be observed. Standing in a spy makes the attempt visible.
  vi.stubGlobal('indexedDB', {
    open: indexedDBOpen,
    deleteDatabase: indexedDBOpen,
    databases: () => Promise.resolve([]),
  });
});

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('browser storage', () => {
  it('is untouched after every component in the layer has been exercised', async () => {
    render(<ExerciseEveryComponent text="edge-one" />);

    // Interact, not just render: a component that writes on click rather than
    // on mount would pass a render-only assertion.
    await userEvent.click(screen.getByRole('button', { name: 'Show a confirmation' }));
    await userEvent.click(screen.getByRole('button', { name: 'Dismiss' }));

    await userEvent.click(screen.getByRole('button', { name: 'Open the dialog' }));
    await userEvent.click(await screen.findByRole('button', { name: 'Close the dialog' }));

    await userEvent.click(screen.getByRole('button', { name: 'Revoke the device' }));
    await userEvent.type(await screen.findByLabelText('Object name'), 'edge-one');
    // Confirming closes the dialog, so this is the whole level 3 path: step-up,
    // typed name, and the confirm that spends the mark.
    await userEvent.click(screen.getByRole('button', { name: 'Revoke device' }));

    // One-time material is the case that most invites a "remember it for me"
    // convenience, so it is exercised through display and dismissal.
    await userEvent.click(screen.getByRole('button', { name: 'Show the one-time secret' }));
    await screen.findByTestId('one-time-secret');
    await userEvent.click(screen.getByRole('button', { name: 'I have stored it securely' }));

    expect(localStorage).toHaveLength(0);
    expect(sessionStorage).toHaveLength(0);
    expect(await indexedDB.databases()).toHaveLength(0);

    expect(setItem).not.toHaveBeenCalled();
    expect(removeItem).not.toHaveBeenCalled();
    expect(indexedDBOpen).not.toHaveBeenCalled();
  });

  it('exercises every component the layer exports', () => {
    // A component added to the layer but not to the exercise would otherwise
    // be silently exempt from this test and from the hostile-content test.
    const exported = Object.entries(layer)
      .filter(([name, value]) => typeof value === 'function' && /^[A-Z]/.test(name))
      .map(([name]) => name);
    const exercised = new Set<string>(EXERCISED_COMPONENTS);
    const missing = exported.filter((name) => !exercised.has(name));
    // `RootErrorBoundary` is exercised by `boundary/ErrorBoundary.test.tsx`,
    // which needs a component that throws; it cannot share this page.
    expect(missing).toEqual(['RootErrorBoundary']);
  });
});
