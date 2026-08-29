import { readdir, stat } from 'node:fs/promises';
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
if (javascriptBytes > 450 * 1024) throw new Error(`JavaScript bundle ${javascriptBytes} exceeds 450 KiB`);
if (cssBytes > 32 * 1024) throw new Error(`CSS bundle ${cssBytes} exceeds 32 KiB`);
process.stdout.write(`bundle budget: js=${javascriptBytes} bytes css=${cssBytes} bytes\n`);
