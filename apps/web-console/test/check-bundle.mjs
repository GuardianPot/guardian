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
//
// `WCX-06` adds three more classes. The component workbench renders every
// state from fixtures and must not be reachable in production (section 9.5);
// the hostile corpus is a catalogue of attack strings with no place in a
// shipped bundle; and MSW would be a request interceptor sitting in front of a
// real Control Plane (section 8.4). Each is identified by a marker that only
// exists in the module it names, so a rename cannot silently pass this check.
const forbidden = [
  'mockApi',
  'loginHandlers',
  '__boundary__',
  'establishing test session',
  // Workbench (section 9.5, first of three exclusion proofs).
  'guardian-component-workbench',
  'Component workbench',
  // Hostile corpus fixture ids (section 9.2).
  'rtl-override-filename',
  'zero-width-keyword',
  'double-extension-filename',
  // MSW (section 8.4).
  'msw/browser',
  'setupWorker',
  'onUnhandledRequest',
];
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
