import { useEffect, useRef } from 'react';
import styles from '@shared/styles/app.module.css';
import { documentTitle, navigationAnnouncement } from './screens';

/**
 * Route-change focus, announcement, and document title (WCX-05 section 9.2,
 * remediates `P1-W11` GAP-4).
 *
 * Before this, a navigation changed the rendered content and nothing else:
 * focus stayed wherever it was, no screen reader was told anything happened,
 * and the tab title read "Guardian Console" forever. A keyboard operator who
 * activated a device link landed on a screen whose content was below their
 * focus position, with no way to know it had changed.
 *
 * Three things happen on every completed navigation:
 *
 * 1. `document.title` becomes `<screen name> — Guardian Console`.
 * 2. Focus moves to the screen's `h1`. When the screen is still loading there
 *    is no `h1` yet, so focus goes to the `main` landmark and moves on to the
 *    heading when it renders.
 * 3. The screen name is announced through one polite live region that lives
 *    here and is never re-created, so an announcement replaces the previous
 *    one instead of stacking regions up.
 *
 * Focus is only ever *claimed*, never stolen: if the operator has already
 * moved focus somewhere themselves while a screen loaded, the late-arriving
 * heading does not take it back.
 */
export function RouteAnnouncer({ screen, locationKey }: { screen: string | undefined; locationKey: string }) {
  // Derived during render rather than held in state, so the region's text and
  // the new screen's content land in the same commit. A screen reader
  // announces the change to a region that was already there, which is the
  // whole reason the region outlives the navigation.
  const announcement = navigationAnnouncement(screen);
  // The element this component last focused. Anything else holding focus means
  // the operator put it there, and it is not ours to move.
  const claimed = useRef<HTMLElement | null>(null);

  useEffect(() => {
    document.title = documentTitle(screen);
  }, [screen]);

  useEffect(() => {
    const claim = (target: HTMLElement): void => {
      const active = document.activeElement;
      const unclaimed = active === null || active === document.body || active === claimed.current;
      if (!unclaimed) return;
      claimed.current = target;
      target.focus();
    };

    /** True once focus is on the heading and there is nothing left to wait for. */
    const settle = (): boolean => {
      const main = document.querySelector('main');
      if (!main) return false;
      const heading = main.querySelector('h1');
      if (heading instanceof HTMLElement) {
        claim(heading);
        return true;
      }
      // Section 9.2.2: land on the landmark, then follow the heading in.
      if (main instanceof HTMLElement) claim(main);
      return false;
    };

    if (settle()) return;

    const observer = new MutationObserver(() => {
      if (settle()) observer.disconnect();
    });
    observer.observe(document.body, { childList: true, subtree: true });
    return () => observer.disconnect();
  }, [locationKey]);

  return (
    <div className={styles.visuallyHidden} role="status">{announcement}</div>
  );
}
