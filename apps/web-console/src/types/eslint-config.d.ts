/**
 * The repository's own ESLint config, as seen from a test (WCX-08).
 *
 * `eslint.config.js` is JavaScript and sits outside `src`, so `tsc` has
 * nothing to read it with. Only the surface `literalText.test.ts` imports is
 * declared: the local plugin holding `guardian/no-literal-text`. Importing the
 * real object rather than restating the rule is the point — a copy could drift
 * from the rule the build actually runs.
 */
declare module '*/eslint.config.js' {
  import type { ESLint, Linter } from 'eslint';

  export const guardianPlugin: ESLint.Plugin;

  const config: Linter.Config[];
  export default config;
}
