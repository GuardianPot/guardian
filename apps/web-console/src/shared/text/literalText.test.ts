import { ESLint } from 'eslint';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import { guardianPlugin } from '../../../eslint.config.js';
import { CATALOGUE, type CatalogueKey } from './catalogue';

/**
 * The literal-text rule (WCX-08 section 9.1 and 10.1.1).
 *
 * Two things have to hold and a clean workspace proves neither: the rule has
 * to be configured as an error on real component files, and it has to actually
 * fire. A rule that silently fails to load looks exactly like a codebase with
 * no violations — which is how the first run of this rule reported zero
 * problems while ESLint was in fact refusing to start.
 *
 * The plugin is imported from the real `eslint.config.js`, so the rule proved
 * here is the rule the build runs, not a copy that can drift from it.
 */
const SLOW = { timeout: 60_000 };

/** Lints fixture source against the rule alone, with no type-aware layer. */
async function lint(fixture: string): Promise<ESLint.LintResult> {
  const eslint = new ESLint({
    overrideConfigFile: true,
    overrideConfig: [
      {
        files: ['**/*.tsx'],
        languageOptions: { parserOptions: { ecmaFeatures: { jsx: true } } },
        plugins: { guardian: guardianPlugin },
        rules: { 'guardian/no-literal-text': 'error' },
      },
    ],
  });
  const [result] = await eslint.lintText(`${fixture}\n`, { filePath: 'literal-fixture.tsx' });
  if (result === undefined) throw new Error('ESLint returned no result for the fixture');
  return result;
}

describe('guardian/no-literal-text', () => {
  it('is an error for component files in the repository config', SLOW, async () => {
    const eslint = new ESLint();
    const config = (await eslint.calculateConfigForFile('src/app/Shell.tsx')) as {
      rules?: Record<string, unknown>;
    };
    const configured = config.rules?.['guardian/no-literal-text'];
    expect(configured, 'the rule must be configured for components').toBeDefined();
    const severity: unknown = Array.isArray(configured) ? (configured as unknown[])[0] : configured;
    expect(severity, 'the rule must be an error, not a warning').toBe(2);
  });

  it('rejects a sentence written as JSX text', SLOW, async () => {
    const result = await lint('export const Bad = () => <p>Everything looks healthy.</p>;');
    expect(result.messages.map((m) => m.ruleId)).toEqual(['guardian/no-literal-text']);
  });

  it('rejects a sentence smuggled through a child expression', SLOW, async () => {
    const result = await lint("export const Bad = () => <p>{'Everything looks healthy.'}</p>;");
    expect(result.messages.map((m) => m.ruleId)).toEqual(['guardian/no-literal-text']);
  });

  it('rejects a template literal with no interpolation', SLOW, async () => {
    const result = await lint('export const Bad = () => <p>{`Everything looks healthy.`}</p>;');
    expect(result.messages.map((m) => m.ruleId)).toEqual(['guardian/no-literal-text']);
  });

  it.each([
    ['aria-label', '<button aria-label="Dismiss the banner" />'],
    ['title', '<span title="Last observed" />'],
    ['placeholder', '<input placeholder="Zone name" />'],
    ['alt', '<img src="/x.png" alt="A network diagram" />'],
    // Not in the four the specification names: caught because the allowlist
    // is of technical attributes, so a new operator-facing prop is a
    // violation by default rather than once someone remembers to list it.
    ['a component prop', '<Panel heading="Environments" />'],
  ])('rejects operator-facing text in %s', SLOW, async (_name, element) => {
    const result = await lint(`export const Bad = () => ${element};`);
    expect(result.messages.map((m) => m.ruleId)).toEqual(['guardian/no-literal-text']);
  });

  it('allows technical attributes, punctuation, and catalogue calls', SLOW, async () => {
    const result = await lint([
      "import { t } from '@shared/text';",
      'export const Fine = () => (',
      '  <a className="brand" href="/environments" data-testid="brand" aria-hidden="true">',
      "    {t('common.product')} · {t('common.productScope')}",
      '  </a>',
      ');',
    ].join('\n'));
    expect(result.messages).toEqual([]);
  });
});

const walk = (dir: string): string[] =>
  readdirSync(dir).flatMap((entry) => {
    const path = join(dir, entry);
    return statSync(path).isDirectory() ? walk(path) : [path];
  });

describe('the catalogue and the components agree', () => {
  const sources = walk('src')
    .filter((file) => /\.tsx?$/.test(file) && !file.includes(join('shared', 'text')))
    .map((file) => readFileSync(file, 'utf8'))
    .join('\n');

  it('has no entry no component reads', () => {
    // Section 9.1's premise is that the catalogue can be read in one sitting.
    // Wording left behind by a deleted screen is wording a reviewer spends
    // attention on for nothing, and it is indistinguishable from live text.
    const unused = (Object.keys(CATALOGUE) as CatalogueKey[]).filter((key) => {
      if (sources.includes(`'${key}'`) || sources.includes(`"${key}"`)) return false;
      // `plural('time.age.minute', n)` reaches `.one` and `.other`.
      const base = key.replace(/\.(one|other)$/, '');
      if (base !== key && sources.includes(`'${base}'`)) return false;
      // `t(\`health.condition.${type}\`)` reaches a whole sub-namespace.
      const parent = key.slice(0, key.lastIndexOf('.'));
      return !sources.includes(`\`${parent}.\${`);
    });
    expect(unused, 'delete the entry, or read it from a component').toEqual([]);
  });
});
