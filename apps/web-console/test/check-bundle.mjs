import { readdir, readFile, stat } from 'node:fs/promises';
import { join } from 'node:path';
import process from 'node:process';
import { fileURLToPath, URL } from 'node:url';

const directory = fileURLToPath(new URL('../dist/assets/', import.meta.url));
const entries = await readdir(directory);
let javascriptBytes = 0;
let cssBytes = 0;
for (const entry of entries) {
  const size = (await stat(join(directory, entry))).size;
  if (entry.endsWith('.js')) javascriptBytes += size;
  if (entry.endsWith('.css')) cssBytes += size;
  if (entry.endsWith('.map')) throw new Error(`production source map is forbidden: ${entry}`);
}

// Test-only helpers and boundary fixtures must never reach a production build.
// A bundled harness could stub fetch in front of a real operator session.
const forbidden = ['stubFetch', 'loginHandlers', '__boundary__', 'establishing test session'];
for (const entry of entries.filter((name) => name.endsWith('.js'))) {
  const source = await readFile(join(directory, entry), 'utf8');
  for (const marker of forbidden) {
    if (source.includes(marker)) {
      throw new Error(`test-only code reached the production bundle: ${marker} in ${entry}`);
    }
  }
}
if (javascriptBytes > 450 * 1024) throw new Error(`JavaScript bundle ${javascriptBytes} exceeds 450 KiB`);
if (cssBytes > 32 * 1024) throw new Error(`CSS bundle ${cssBytes} exceeds 32 KiB`);
process.stdout.write(`bundle budget: js=${javascriptBytes} bytes css=${cssBytes} bytes\n`);
