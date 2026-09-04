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
  },
  test: {
    environment: 'jsdom',
    setupFiles: './src/shared/testing/setup.ts',
    css: true,
  },
});
