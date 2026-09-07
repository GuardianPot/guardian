/**
 * `eslint-plugin-jsx-a11y` ships no types (WCX-05).
 *
 * Only the surface this repository uses is declared: the flat-config export
 * that `eslint.config.js` spreads and `a11yLint.test.ts` lints against. A
 * blanket `any` would let a rename pass typecheck and fail at lint time.
 */
declare module 'eslint-plugin-jsx-a11y' {
  import type { Linter } from 'eslint';

  const plugin: {
    flatConfigs: {
      recommended: Linter.Config;
      strict: Linter.Config;
    };
  };

  export default plugin;
}
