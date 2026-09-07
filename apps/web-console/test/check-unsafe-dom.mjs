import console from 'node:console';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';
import process from 'node:process';

/**
 * Forbidden DOM and evaluation APIs (WCX-06 section 8.2).
 *
 * Every one of these turns a string into markup or into code. Guardian renders
 * attacker-authored strings on almost every screen, so the console does not
 * get to use them — not behind a "this input is safe" comment, not in a test,
 * not with a suppression. Section 8.2 permits no suppression, so this is a
 * repository check rather than an ESLint rule that a directive could switch
 * off line by line.
 *
 * React's own escaping is not the protection being enforced here. The
 * protection is that there is no code path from a captured value to the HTML
 * parser at all.
 */
const FORBIDDEN = [
  { pattern: /\bdangerouslySetInnerHTML\b/, why: 'renders a string as markup' },
  { pattern: /\.innerHTML\s*=/, why: 'renders a string as markup' },
  { pattern: /\.outerHTML\s*=/, why: 'renders a string as markup' },
  { pattern: /\binsertAdjacentHTML\s*\(/, why: 'renders a string as markup' },
  { pattern: /\bdocument\s*\.\s*write(?:ln)?\s*\(/, why: 'renders a string as markup' },
  { pattern: /(?<![.\w])eval\s*\(/, why: 'evaluates a string as code' },
  { pattern: /new\s+Function\s*\(/, why: 'evaluates a string as code' },
  { pattern: /\bsetTimeout\s*\(\s*['"`]/, why: 'evaluates a string as code' },
  { pattern: /\bsetInterval\s*\(\s*['"`]/, why: 'evaluates a string as code' },
];

const walk = (dir) =>
  readdirSync(dir).flatMap((entry) => {
    const path = join(dir, entry);
    return statSync(path).isDirectory() ? walk(path) : [path];
  });

const failures = [];
for (const path of walk('src')) {
  const unix = path.split('\\').join('/');
  if (!/\.(?:ts|tsx)$/.test(unix)) continue;
  // The generated OpenAPI types are contract output, not console code.
  if (unix.startsWith('src/generated/')) continue;

  // Comments are blanked rather than removed, so a rule never fires on the
  // prose that documents it and line numbers still point at real code.
  const code = readFileSync(path, 'utf8').replace(/\/\*[\s\S]*?\*\//g, (comment) =>
    comment.replace(/[^\n]/g, ' '),
  );
  code.split('\n').forEach((line, index) => {
    const statement = line.split('//')[0] ?? '';
    for (const { pattern, why } of FORBIDDEN) {
      if (pattern.test(statement)) {
        failures.push(`${unix}:${index + 1} uses a forbidden API that ${why}`);
      }
    }
  });
}

if (failures.length > 0) {
  console.error(`${failures.join('\n')}\n\nSection 8.2 permits no suppression for these.`);
  process.exit(1);
}
console.log(`No forbidden DOM or evaluation API in the console: ${FORBIDDEN.length} patterns checked.`);
