import '@testing-library/jest-dom/vitest';
import { Headers, Request, Response } from 'undici';
import { afterEach } from 'vitest';
import { cleanup, configure } from '@testing-library/react';
import { installMatchMedia, resetViewportWidth } from './viewport';

/**
 * Hand the test environment Node's `Request`, `Response`, and `Headers`
 * (WCX-06 section 9.3).
 *
 * jsdom supplies its own, and MSW's interceptor rebuilds each intercepted
 * request using whichever classes are global. Rebuilt through jsdom's, every
 * header the console set was dropped — a test asserting `X-CSRF-Token` or
 * `If-Match` saw an empty header list and would have passed a console that
 * sent neither. That is a security assertion silently turning into a no-op, so
 * the classes are replaced before any test runs.
 *
 * `fetch` itself is left alone: it is already Node's, and MSW patches it.
 * Overriding only the three classes keeps the blast radius as small as the
 * problem.
 */
Object.assign(globalThis, { Headers, Request, Response });

/**
 * A viewport the suite can set (WCX-10 section 10.1.6).
 *
 * jsdom supplies no `matchMedia`, so the shell would read 'wide' at every
 * width and the responsive rules would be invisible to every test. That is
 * how `P1-W11` GAP-1 survived a full suite: a media query removed sign-out
 * below 900 pixels and nothing in the suite had a width at all.
 */
installMatchMedia();

/**
 * A realistic budget for `findBy*` and `waitFor`.
 *
 * Testing Library defaults to one second. That was ample when every test
 * mounted one component; `WCX-10` added suites that mount the whole
 * application — a lazy route chunk, the session probe, and a feature query
 * before the first assertion can run — and under a loaded parallel run those
 * three exceeded a second and reported as missing elements.
 *
 * This weakens no assertion. A `findBy` that resolves in 40ms still resolves
 * in 40ms; only the point at which waiting is called a failure moves, and it
 * moves to somewhere that reflects what these tests actually do. The
 * alternative — sprinkling per-call timeouts — hides the same decision in
 * dozens of places.
 */
configure({ asyncUtilTimeout: 5_000 });

afterEach(() => {
  cleanup();
  resetViewportWidth();
});
