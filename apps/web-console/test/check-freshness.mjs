import console from 'node:console';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';
import process from 'node:process';

/**
 * One place decides cadence (WCX-07 section 9.1, decision WC-D05).
 *
 * Before this package four queries carried a hard-coded five-second interval
 * and nothing stopped the next screen inventing a fifth value. That is not
 * untidiness: `WC-D05` chose polling over a server-driven channel on purpose
 * and recorded a measured condition for reconsidering it. A trigger cannot be
 * evaluated against constants scattered through feature modules.
 *
 * So a resource picks a freshness class and never an interval. This fails the
 * lint if a cadence option appears anywhere but the policy module that defines
 * what a class means.
 */
const POLICY = 'src/shared/api/freshness.ts';
const OPTIONS =
  /\b(refetchInterval|staleTime|refetchIntervalInBackground|refetchOnWindowFocus|refetchOnReconnect)\b/;

const walk = (dir) =>
  readdirSync(dir).flatMap((entry) => {
    const path = join(dir, entry);
    return statSync(path).isDirectory() ? walk(path) : [path];
  });

const failures = [];
for (const path of walk('src')) {
  const unix = path.split('\\').join('/');
  if (!/\.(?:ts|tsx)$/.test(unix)) continue;
  if (unix === POLICY) continue;
  // A test may assert what a class resolves to. That is reading the policy,
  // not setting a cadence beside it.
  if (/\.test\.tsx?$/.test(unix)) continue;

  readFileSync(path, 'utf8')
    // Blank comments rather than remove them, so a rule never fires on the
    // prose that documents it and line numbers still point at real code.
    .replace(/\/\*[\s\S]*?\*\//g, (comment) => comment.replace(/[^\n]/g, ' '))
    .split('\n')
    .forEach((line, index) => {
      const code = line.split('//')[0] ?? '';
      if (OPTIONS.test(code)) {
        failures.push(
          `${unix}:${index + 1} sets a query cadence; declare a freshness class in ${POLICY} instead`,
        );
      }
    });
}

if (failures.length > 0) {
  console.error(failures.join('\n'));
  process.exit(1);
}
console.log(`Freshness policy holds: no cadence option outside ${POLICY}.`);
