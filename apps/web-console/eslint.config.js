import eslint from '@eslint/js';
import reactHooks from 'eslint-plugin-react-hooks';
import tseslint from 'typescript-eslint';

/**
 * Import boundaries (WCX-01, decision WC-D01).
 *
 *   app/**        may import feature public APIs and shared.
 *   features/a/** may import shared, generated, and its own internals.
 *                 A feature may import another feature only through that
 *                 feature's index, and only for the pairs listed below.
 *   shared/**     may import shared and generated only.
 *   generated/**  imports nothing from the application.
 *
 * Deep imports into another feature are always errors, as are relative
 * imports that escape a feature. Flat config replaces rather than merges a
 * rule's options, so every block below repeats the patterns it needs.
 */
const DEEP_FEATURE_IMPORT = {
  group: ['@features/*/*'],
  message: 'Import another feature through its index (@features/<name>), never a file inside it.',
};

const ESCAPING_RELATIVE_IMPORT = {
  group: ['../*/*', '../../**'],
  message: 'Use an alias (@app, @features, @shared, @generated) instead of a relative import that escapes this directory.',
};

const TEST_ONLY_IMPORT = {
  group: ['@shared/testing', '@shared/testing/**'],
  message: 'Test-only helpers must not be imported by production code.',
};

const NO_APP_IMPORT = {
  group: ['@app/*', '@app/**'],
  message: 'A feature may not import application shell code.',
};

const FEATURES = ['auth', 'environments', 'devices', 'health'];

/** One boundary block per feature. `allowedPeers` documents approved pairs. */
const featureBoundary = (name, allowedPeers) => {
  const forbidden = FEATURES
    .filter((peer) => peer !== name && !allowedPeers.includes(peer))
    .map((peer) => `@features/${peer}`);
  return {
    files: [`src/features/${name}/**/*.{ts,tsx}`],
    ignores: [`src/features/${name}/**/*.test.{ts,tsx}`],
    rules: {
      'no-restricted-imports': ['error', {
        patterns: [
          DEEP_FEATURE_IMPORT,
          ESCAPING_RELATIVE_IMPORT,
          TEST_ONLY_IMPORT,
          NO_APP_IMPORT,
          ...(forbidden.length
            ? [{ group: forbidden, message: `Feature "${name}" has no approved dependency on that feature. Add the pair to eslint.config.js with a reason first.` }]
            : []),
        ],
      }],
    },
  };
};

export default tseslint.config(
  { ignores: ['dist/**', 'node_modules/**', 'src/generated/**', 'src/**/__boundary__/**'] },
  eslint.configs.recommended,
  ...tseslint.configs.recommendedTypeChecked,
  {
    languageOptions: {
      parserOptions: { projectService: true, tsconfigRootDir: import.meta.dirname },
    },
  },
  { files: ['**/*.{js,mjs}'], ...tseslint.configs.disableTypeChecked },
  {
    files: ['src/**/*.{ts,tsx}'],
    plugins: { 'react-hooks': reactHooks },
    rules: reactHooks.configs.recommended.rules,
  },
  // Default for production modules outside the layers handled below.
  {
    files: ['src/**/*.{ts,tsx}'],
    ignores: ['src/**/*.test.{ts,tsx}', 'src/shared/testing/**'],
    rules: {
      'no-restricted-imports': ['error', {
        patterns: [DEEP_FEATURE_IMPORT, ESCAPING_RELATIVE_IMPORT, TEST_ONLY_IMPORT],
      }],
    },
  },
  // shared may never reach upward into features or the application shell.
  {
    files: ['src/shared/**/*.{ts,tsx}'],
    ignores: ['src/shared/testing/**', 'src/shared/**/*.test.{ts,tsx}'],
    rules: {
      'no-restricted-imports': ['error', {
        patterns: [
          { group: ['@features/*', '@features/**', '@app/*', '@app/**'], message: 'shared must not depend on features or the application shell.' },
          ESCAPING_RELATIVE_IMPORT,
          TEST_ONLY_IMPORT,
        ],
      }],
    },
  },
  // Approved cross-feature pairs, each with its reason:
  //   environments -> auth    session state and the capability seam
  //   environments -> health  renders the backend health projection panel
  //   environments -> devices creating an enrollment secret changes the device
  //                           inventory, so the environment feature invalidates it
  //   devices      -> health  renders the backend health projection panel
  featureBoundary('auth', []),
  featureBoundary('environments', ['auth', 'health', 'devices']),
  featureBoundary('devices', ['health']),
  featureBoundary('health', []),
  // Tests may reach the harness but still may not deep-import a feature.
  {
    files: ['src/**/*.test.{ts,tsx}', 'src/shared/testing/**/*.{ts,tsx}'],
    rules: {
      'no-restricted-imports': ['error', { patterns: [DEEP_FEATURE_IMPORT] }],
    },
  },
);
