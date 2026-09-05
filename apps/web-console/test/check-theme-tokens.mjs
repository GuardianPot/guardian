import console from 'node:console';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';
import process from 'node:process';

/**
 * Enforces the token boundary (WCX-03, decision WC-D10).
 *
 * Two rules, both about values that change when `WCX-15` adds the light theme:
 *
 * 1. No colour literal outside `src/shared/theme/`. A hard-coded colour would
 *    survive a theme switch unchanged and silently break contrast.
 * 2. No raw duration outside `src/shared/theme/`. The motion budget lives in
 *    one place so it stays inside the approved cap.
 *
 * Layout geometry — grid tracks, widths, clamps — is deliberately **not**
 * covered. It is structural rather than thematic, does not change with the
 * theme, and forcing it through tokens would produce a token per one-off
 * measurement without making anything safer.
 */
const THEME_DIR = 'src/shared/theme';
const COLOUR = /#[0-9a-fA-F]{3,8}\b|\b(?:rgba?|hsla?|oklch|lab|color)\s*\(/;
const DURATION = /(?<![\w-])\d+(?:\.\d+)?m?s(?![\w-])/;

const walk = (dir) =>
  readdirSync(dir).flatMap((entry) => {
    const path = join(dir, entry);
    return statSync(path).isDirectory() ? walk(path) : [path];
  });

const failures = [];
for (const path of walk('src')) {
  const unix = path.split('\\').join('/');
  if (!unix.endsWith('.css')) continue;
  if (unix.startsWith(THEME_DIR)) continue;

  readFileSync(path, 'utf8')
    .split('\n')
    .forEach((line, index) => {
      const code = line.split('/*')[0] ?? '';
      const where = `${unix}:${index + 1}`;
      if (COLOUR.test(code)) {
        failures.push(`${where} has a colour literal; use a semantic token from ${THEME_DIR}`);
      }
      // `transition`/`animation` shorthands are where a raw duration hides.
      if (DURATION.test(code) && !code.includes('var(--motion-')) {
        failures.push(`${where} has a raw duration; use a --motion-* token`);
      }
    });
}

if (failures.length > 0) {
  console.error(failures.join('\n'));
  process.exit(1);
}
console.log('Theme token boundary holds: no colour literal or raw duration outside the theme.');
