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
           * The auth feature is split three ways, because its parts have three
           * different lifetimes.
           *
           * Sign-in and the MFA method chooser are the unauthenticated screen.
           * They travel together: step-up reauthentication offers the same
           * proofs through the same control, and a signed-in operator already
           * holds this chunk, whereas the reverse would make the sign-in
           * screen pull authenticated code.
           */
          if (/\/src\/features\/auth\/(LoginPage|MfaMethodField)\./.test(path)) return 'login';

          /*
           * The step-up prompt is loaded on demand. It brings Radix's dialog,
           * and it is needed only when an operator reaches for an irreversible
           * action — never on a first paint, authenticated or not.
           */
          if (/\/src\/features\/auth\/StepUpDialog\./.test(path)) return 'step-up';

          /*
           * Everything else in the feature — the session context, the
           * capability seam, the session query — is needed before any route
           * renders, including the sign-in route. It belongs with the shell.
           * Left as `feature-auth` it became a chunk the login chunk imported,
           * which is exactly the edge `check-bundle.mjs` forbids.
           */
          if (path.includes('/src/features/auth/')) return 'entry';

          const feature = /\/src\/features\/([^/]+)\//.exec(path);
          if (feature) return `feature-${feature[1]}`;

          /*
           * The modal stack stays out of the entry chunk.
           *
           * Radix's dialog brings a focus trap, a dismissable layer, a portal,
           * and scroll locking. The sign-in screen opens no dialog, and
           * shipping all of that to an unauthenticated visitor is what pushed
           * the initial login load over its budget in `WCX-07`. Keeping it
           * unassigned is what holds that, and it still does.
           *
           * It does *not* get its own name, and the reason is a limitation
           * rather than a preference. `WCX-09` gave dialogs to three features
           * and tried `return 'modals'` here; no such chunk is emitted. Radix
           * is placed by the bundler's own logic regardless of what this
           * function returns for the console modules that import it — the same
           * behaviour `WCX-07` recorded when `vendor-ui` never appeared.
           *
           * The observable consequence: the stack travels inside
           * `feature-account`, and `feature-devices` and
           * `feature-environments` import that chunk to get it. Every budget
           * holds and `check-bundle.mjs` reports each chunk by name, so the
           * cost is visible rather than hidden — but opening a device page
           * does fetch the account chunk. Revisit if a chunking API that
           * reaches third-party modules becomes available.
           */
          if (/\/src\/shared\/ui\/(controls\/Dialog|confirm\/|secret\/)/.test(path)) return undefined;

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
          /*
           * The home placeholder and the not-found screen are screens, not
           * shell. Section 9.9 keeps the shell in `entry`, and it is —
           * these are what it renders into, and leaving them there put the
           * incident placeholder in the chunk an unauthenticated visitor
           * downloads.
           */
          if (/\/src\/app\/(HomePage|NotFoundPage)\./.test(path)) return 'home';

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
    /*
     * Above Vitest's five-second default, because `setup.ts` raises Testing
     * Library's async budget to five. A test that legitimately waits four
     * seconds for a lazy route under load would otherwise hit the test wall
     * first and report as a timeout rather than as whatever it was waiting
     * for. Suites that build a TypeScript program or run ESLint set their own
     * longer timeouts on top of this.
     */
    testTimeout: 20_000,
  },
});
