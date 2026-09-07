import '@testing-library/jest-dom/vitest';
import { Headers, Request, Response } from 'undici';
import { afterEach } from 'vitest';
import { cleanup } from '@testing-library/react';

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

afterEach(() => cleanup());
