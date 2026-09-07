/**
 * A viewport width that tests can set (WCX-10 section 10.1.6).
 *
 * jsdom has no layout and no `matchMedia`, so a responsive rule is invisible
 * to it. That matters more here than usual: `P1-W11` GAP-1 was a media query
 * that removed sign-out below 900 pixels, and it survived a full suite because
 * nothing in that suite had a width at all.
 *
 * So the shell reads the viewport through `matchMedia` — the same API a
 * browser answers — and this installs an implementation that answers from a
 * width a test controls. What the suite asserts is then the real code path,
 * not a component rendered with a `narrow` prop it would never receive in
 * production.
 *
 * Only `(max-width: Npx)` and `(min-width: Npx)` are understood, because those
 * are the forms this console's stylesheet and shell use. An unrecognised query
 * throws rather than quietly reporting `false`, which would make a broken test
 * look like a passing one.
 */
export const DEFAULT_TEST_WIDTH = 1440;

let width = DEFAULT_TEST_WIDTH;
const listeners = new Set<() => void>();

const WIDTH_QUERY = /^\((max|min)-width:\s*(\d+)px\)$/;

function matches(query: string): boolean {
  const parsed = WIDTH_QUERY.exec(query.trim());
  if (parsed === null) throw new Error(`the test matchMedia does not understand: ${query}`);
  const bound = Number(parsed[2]);
  return parsed[1] === 'max' ? width <= bound : width >= bound;
}

/** Sets the width and notifies every subscriber, as a resize would. */
export function setViewportWidth(next: number): void {
  width = next;
  for (const listener of [...listeners]) listener();
}

export function resetViewportWidth(): void {
  setViewportWidth(DEFAULT_TEST_WIDTH);
}

export function installMatchMedia(): void {
  Object.defineProperty(window, 'matchMedia', {
    configurable: true,
    writable: true,
    value: (query: string): MediaQueryList => {
      const list = {
        get matches() { return matches(query); },
        media: query,
        onchange: null,
        addEventListener: (_type: string, listener: () => void) => { listeners.add(listener); },
        removeEventListener: (_type: string, listener: () => void) => { listeners.delete(listener); },
        // The deprecated pair, because some libraries still reach for it.
        addListener: (listener: () => void) => { listeners.add(listener); },
        removeListener: (listener: () => void) => { listeners.delete(listener); },
        dispatchEvent: () => true,
      };
      return list as unknown as MediaQueryList;
    },
  });
}
