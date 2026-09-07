import { fileURLToPath, URL } from 'node:url';
import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';

const alias = {
  '@app': fileURLToPath(new URL('./src/app', import.meta.url)),
  '@features': fileURLToPath(new URL('./src/features', import.meta.url)),
  '@shared': fileURLToPath(new URL('./src/shared', import.meta.url)),
  '@generated': fileURLToPath(new URL('./src/generated', import.meta.url)),
};

export default defineConfig({
  plugins: [react()],
  resolve: { alias },
  build: {
    sourcemap: false,
    target: 'es2022',
    rollupOptions: {
      output: {
        /*
         * Chunk boundaries (WCX-07 section 9.2).
         *
         * Declared rather than left to Rollup's defaults so growth is
         * attributable: `check-bundle.mjs` reports every chunk by name, and a
         * dependency that lands in the wrong one is visible instead of being
         * absorbed into a single opaque bundle.
         *
         * Names carry route and package identifiers only. Section 8.4 forbids
         * a chunk name from carrying an environment, device, or incident
         * identifier, and none of these can: they are computed from the
         * module path, never from data.
         */
        manualChunks(id: string) {
          const path = id.split('\\').join('/');

          if (path.includes('/node_modules/')) {
            if (/\/node_modules\/(react|react-dom|scheduler)\//.test(path)) return 'vendor-react';
            if (path.includes('/node_modules/@tanstack/')) return 'vendor-query';
            if (path.includes('radix')) return 'vendor-ui';
            // The router and everything else load with the shell.
            return undefined;
          }

          /*
           * Sign-in is its own chunk even though it lives in the auth feature.
           * The rest of that feature — the session context and the queries it
           * wraps — is needed before any route renders, so leaving the
           * unauthenticated screen in it would put sign-in into every
           * authenticated load.
           */
          if (/\/src\/features\/auth\/(LoginPage|text)\./.test(path)) return 'login';

          const feature = /\/src\/features\/([^/]+)\//.exec(path);
          if (feature) return `feature-${feature[1]}`;

          /*
           * The modal stack is deliberately left out of the entry chunk.
           *
           * Radix's dialog brings a focus trap, a dismissable layer, a portal,
           * and scroll locking. The sign-in screen opens no dialog, and
           * shipping all of that to an unauthenticated visitor is what pushed
           * the initial login load over its budget. Left unassigned, it
           * travels with the first feature chunk that actually opens one.
           */
          if (/\/src\/shared\/ui\/(controls\/Dialog|confirm\/)/.test(path)) return undefined;

          /*
           * The rest of the shared layer and the app shell are named
           * explicitly rather than left to the bundler. Unassigned, a module
           * reachable from several feature chunks is placed in one of them —
           * the shared UI layer and the whole stylesheet landed in
           * `feature-devices`, so opening the environments screen downloaded
           * the device chunk to get a button. Naming the chunk makes the
           * shell's cost visible in the budget report instead of hiding it
           * inside a feature.
           */
          if (/\/src\/(shared|app)\//.test(path)) return 'entry';
          return undefined;
        },
      },
    },
  },
  test: {
    environment: 'jsdom',
    setupFiles: './src/shared/testing/setup.ts',
    css: true,
  },
});
