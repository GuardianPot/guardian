import { ESLint } from 'eslint';
import console from 'node:console';
import process from 'node:process';

/**
 * Proves each import boundary rule actually fails.
 *
 * The fixtures under `src/**\/__boundary__/` are excluded from the normal lint
 * run and are never imported by the application. Linting them with ignores
 * disabled asserts that a violation is an error rather than a convention.
 */
const fixtures = [
  {
    file: 'src/features/devices/__boundary__/deep-feature-import.ts',
    expect: 'never a file inside it',
    describes: 'a feature deep-importing another feature',
  },
  {
    file: 'src/shared/api/__boundary__/shared-imports-feature.ts',
    expect: 'shared must not depend on features',
    describes: 'shared importing a feature',
  },
  {
    file: 'src/features/auth/__boundary__/escaping-relative-import.ts',
    expect: 'instead of a relative import that escapes',
    describes: 'a relative import escaping a feature',
  },
];

const eslint = new ESLint({ ignore: false });
let failed = 0;

for (const fixture of fixtures) {
  const [result] = await eslint.lintFiles([fixture.file]);
  const violations = (result?.messages ?? []).filter(
    (message) => message.ruleId === 'no-restricted-imports',
  );
  if (violations.length === 0) {
    console.error(`FAIL ${fixture.describes}: no no-restricted-imports error (${fixture.file})`);
    failed++;
    continue;
  }
  if (!violations.some((message) => message.message.includes(fixture.expect))) {
    console.error(
      `FAIL ${fixture.describes}: expected "${fixture.expect}", got:\n` +
        violations.map((message) => `  ${message.message}`).join('\n'),
    );
    failed++;
    continue;
  }
  console.log(`ok   ${fixture.describes}`);
}

if (failed > 0) process.exit(1);
console.log(`boundary rules enforced: ${fixtures.length} violations each rejected`);
