import { useSyncExternalStore } from 'react';

/**
 * The single responsive breakpoint (WCX-10 section 9.3.1).
 *
 * One number, in one place, used by the shell's JavaScript and named in the
 * stylesheet's media query beside it. Two breakpoints that drift apart give a
 * width at which the disclosure believes it is closed while the layout has
 * already expanded — and at that width the operator block is in a panel
 * nothing can open.
 */
export const NARROW_BREAKPOINT = 900;
export const NARROW_QUERY = `(max-width: ${NARROW_BREAKPOINT}px)`;

/**
 * Whether the viewport is at or below the breakpoint.
 *
 * `useSyncExternalStore` rather than an effect writing state: the value is
 * read during render and React needs to know it comes from outside, otherwise
 * the first paint is at the wrong width and the disclosure flickers.
 *
 * **Absent `matchMedia` means wide.** That is the fail-safe direction and it
 * is deliberate: wide renders every operator control unconditionally, so an
 * environment the console cannot measure gets the layout that hides nothing.
 * The opposite default would put sign-out behind a disclosure whose state we
 * could not evaluate.
 */
export function useNarrowViewport(): boolean {
  return useSyncExternalStore(subscribe, isNarrow, () => false);
}

function media(): MediaQueryList | null {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') return null;
  return window.matchMedia(NARROW_QUERY);
}

function isNarrow(): boolean {
  return media()?.matches ?? false;
}

function subscribe(onChange: () => void): () => void {
  const query = media();
  if (query === null) return () => undefined;
  query.addEventListener('change', onChange);
  return () => { query.removeEventListener('change', onChange); };
}
