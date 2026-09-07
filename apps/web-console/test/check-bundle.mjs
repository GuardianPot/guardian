import { gzipSync } from 'node:zlib';
import { readdir, readFile, stat } from 'node:fs/promises';
import { join } from 'node:path';
import process from 'node:process';
import { fileURLToPath, URL } from 'node:url';

/**
 * The performance budget (WCX-07 section 9.4, decision WC-D30, PERF-08).
 *
 * Three dimensions rather than one total, because a total hides the number
 * that an operator actually waits for. A console can stay under 450 KiB while
 * making the sign-in screen download every authenticated feature, and only a
 * per-load budget catches that.
 *
 * Sizes are reported per chunk, raw and gzipped, so growth is attributable to
 * a chunk rather than to "the bundle".
 */
const directory = fileURLToPath(new URL('../dist/assets/', import.meta.url));
const entries = await readdir(directory);

const measured = new Map();
let javascriptBytes = 0;
let cssBytes = 0;
for (const entry of entries) {
  if (entry.endsWith('.map')) throw new Error(`production source map is forbidden: ${entry}`);
  const raw = (await stat(join(directory, entry))).size;
  const gzip = gzipSync(await readFile(join(directory, entry))).length;
  measured.set(entry, { raw, gzip });
  if (entry.endsWith('.js')) javascriptBytes += raw;
  if (entry.endsWith('.css')) cssBytes += raw;
}

/** Resolves a content-hashed chunk by its declared name. */
const chunk = (name) => [...measured.keys()].find((file) => file.startsWith(`${name}-`) && file.endsWith('.js'));
const gzipOf = (name) => {
  const file = chunk(name);
  return file === undefined ? 0 : measured.get(file).gzip;
};

// Everything a browser fetches before the first screen paints, whichever
// screen that is. The bootstrap and the runtime are tiny but they are still
// bytes on the wire, so they are counted.
const bootstrap = gzipOf('index') + gzipOf('rolldown-runtime');
const loginLoad = bootstrap + gzipOf('entry') + gzipOf('vendor-react') + gzipOf('vendor-query') + gzipOf('login');
const authenticatedLoad =
  bootstrap + gzipOf('entry') + gzipOf('vendor-react') + gzipOf('vendor-query') + gzipOf('feature-environments');

/*
 * Test-only helpers and boundary fixtures must never reach a production build.
 * A bundled harness could stub fetch in front of a real operator session.
 *
 * `WCX-06` adds three more classes. The component workbench renders every
 * state from fixtures and must not be reachable in production (section 9.5);
 * the hostile corpus is a catalogue of attack strings with no place in a
 * shipped bundle; and MSW would be a request interceptor sitting in front of a
 * real Control Plane (section 8.4). Each is identified by a marker that only
 * exists in the module it names, so a rename cannot silently pass this check.
 */
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

const javascript = entries.filter((name) => name.endsWith('.js'));
for (const entry of javascript) {
  const source = await readFile(join(directory, entry), 'utf8');
  for (const marker of forbidden) {
    if (source.includes(marker)) {
      throw new Error(`test-only code reached the production bundle: ${marker} in ${entry}`);
    }
  }
}

/*
 * Section 9.2.2. The sign-in screen is reachable without a session, so it must
 * not carry code that only an authenticated operator can use. Asserted against
 * the emitted chunk rather than against the source, because the source says
 * what was intended and the chunk says what shipped.
 */
const loginChunk = chunk('login');
if (loginChunk !== undefined) {
  const source = await readFile(join(directory, loginChunk), 'utf8');
  for (const authenticated of javascript.filter((name) => name.startsWith('feature-'))) {
    if (source.includes(authenticated)) {
      throw new Error(`the login chunk pulls an authenticated feature chunk: ${authenticated}`);
    }
  }
}

/*
 * Section 8.4. A chunk name is fetched by an unauthenticated visitor from the
 * static asset listing, so it may name a route or a package and nothing else.
 * A UUID in a chunk name would disclose an environment, device, or incident.
 */
const UUID = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/i;
for (const entry of entries) {
  if (UUID.test(entry)) throw new Error(`chunk name carries a resource identifier: ${entry}`);
}

/*
 * The recorded baseline. `WC-D30` and `PERF-08` make a regression above twenty
 * percent an owner decision, so the comparison needs a committed number rather
 * than the previous run. Update these deliberately, in the same commit as the
 * change that moved them, with the reason in the message.
 */
const BASELINE = {
  loginLoad: 112_920,
  authenticatedLoad: 127_640,
  javascriptBytes: 410_508,
  cssBytes: 25_036,
};

const budgets = [
  { name: 'initial login load (gzip)', value: loginLoad, budget: 120 * 1024, baseline: BASELINE.loginLoad },
  { name: 'initial authenticated load (gzip)', value: authenticatedLoad, budget: 200 * 1024, baseline: BASELINE.authenticatedLoad },
  { name: 'total JavaScript (raw)', value: javascriptBytes, budget: 450 * 1024, baseline: BASELINE.javascriptBytes },
  { name: 'total CSS (raw)', value: cssBytes, budget: 32 * 1024, baseline: BASELINE.cssBytes },
];

const report = [...measured.entries()]
  .sort((left, right) => right[1].gzip - left[1].gzip)
  .map(([file, { raw, gzip }]) => `  ${file.padEnd(38)} ${String(raw).padStart(8)} raw ${String(gzip).padStart(8)} gzip`)
  .join('\n');
process.stdout.write(`chunks:\n${report}\n\n`);

const failures = [];
for (const { name, value, budget, baseline } of budgets) {
  const overBudget = value > budget;
  // A regression is measured against the committed baseline, not against the
  // budget: a change can stay inside the budget and still double a number.
  const growth = baseline === 0 ? 0 : (value - baseline) / baseline;
  const regressed = growth > 0.2;
  const percent = `${(growth * 100).toFixed(1)}%`;
  process.stdout.write(
    `  ${name.padEnd(36)} ${String(value).padStart(8)} / ${String(budget).padStart(7)}  (${percent} vs baseline)\n`,
  );
  if (overBudget) failures.push(`${name}: ${value} exceeds the ${budget} budget`);
  if (regressed) {
    failures.push(`${name}: ${percent} above the recorded baseline ${baseline}; PERF-08 requires owner review`);
  }
}

if (failures.length > 0) {
  process.stderr.write(`\n${failures.join('\n')}\n`);
  process.exit(1);
}
process.stdout.write('\nAll three budget dimensions hold.\n');
