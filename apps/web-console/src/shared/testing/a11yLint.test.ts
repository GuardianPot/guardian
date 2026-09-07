import { ESLint } from 'eslint';
import jsxA11y from 'eslint-plugin-jsx-a11y';
import { readFileSync, readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';

/**
 * Static accessibility enforcement (WCX-05 sections 9.3 and 10.2).
 *
 * Two things have to be true and neither is provable by the workspace merely
 * being clean: the rules have to be *configured* as errors in the real config,
 * and they have to actually *fire*. A rule that is configured but silently
 * unloaded looks exactly like a codebase with no violations.
 */
const REQUIRED_RULES = [
  'jsx-a11y/alt-text',
  'jsx-a11y/anchor-has-content',
  'jsx-a11y/aria-props',
  'jsx-a11y/aria-role',
  'jsx-a11y/label-has-associated-control',
  'jsx-a11y/control-has-associated-label',
  'jsx-a11y/no-autofocus',
  'jsx-a11y/no-redundant-roles',
  'jsx-a11y/no-noninteractive-element-interactions',
  'jsx-a11y/role-has-required-aria-props',
  'jsx-a11y/tabindex-no-positive',
];

describe('jsx-a11y configuration', () => {
  it('is active as errors for every component file', async () => {
    const eslint = new ESLint();
    const config = (await eslint.calculateConfigForFile('src/app/Shell.tsx')) as {
      rules?: Record<string, unknown>;
    };
    for (const rule of REQUIRED_RULES) {
      const configured = config.rules?.[rule];
      expect(configured, `${rule} must be configured`).toBeDefined();
      // A rule is either `severity` or `[severity, ...options]`.
      const severity: unknown = Array.isArray(configured) ? (configured as unknown[])[0] : configured;
      expect(severity, `${rule} must be an error, not a warning`).toBe(2);
    }
  });

  it('rejects an unlabelled input and a positive tabIndex', async () => {
    // Linted as text against a standalone instance carrying the same rule
    // block: the repository config is type-aware, and a file that does not
    // exist on disk has no TypeScript program to be aware of.
    const eslint = new ESLint({
      overrideConfigFile: true,
      overrideConfig: [
        jsxA11y.flatConfigs.recommended,
        {
          files: ['**/*.tsx'],
          languageOptions: { parserOptions: { ecmaFeatures: { jsx: true } } },
          rules: {
            'jsx-a11y/label-has-associated-control': ['error', { assert: 'either' }],
            'jsx-a11y/tabindex-no-positive': 'error',
          },
        },
      ],
    });

    const fixture = [
      'export const Unlabelled = () => <label>Zone name</label>;',
      'export const Reordered = () => <button type="button" tabIndex={3}>Act</button>;',
      'export const Undescribed = () => <img src="x" />;',
    ].join('\n');
    const [result] = await eslint.lintText(`${fixture}\n`, { filePath: 'a11y-fixture.tsx' });
    const fired = (result?.messages ?? []).map((message) => message.ruleId);

    expect(fired).toContain('jsx-a11y/label-has-associated-control');
    expect(fired).toContain('jsx-a11y/tabindex-no-positive');
    expect(fired).toContain('jsx-a11y/alt-text');
    for (const message of result?.messages ?? []) {
      expect(message.severity, `${message.ruleId ?? 'rule'} must be an error`).toBe(2);
    }
  });
});

const walk = (dir: string): string[] =>
  readdirSync(dir).flatMap((entry) => {
    const path = join(dir, entry);
    return statSync(path).isDirectory() ? walk(path) : [path];
  });

describe('rule suppressions', () => {
  it('carry a reason, so a silenced rule is a decision rather than an accident', () => {
    // Section 9.3. `--` is ESLint's own convention for a suppression comment's
    // description, so a suppression that explains itself is also a suppression
    // ESLint can report on.
    const unexplained: string[] = [];
    for (const path of walk('src').filter((file) => /\.tsx?$/.test(file))) {
      readFileSync(path, 'utf8').split('\n').forEach((line, index) => {
        if (!/eslint-disable/.test(line)) return;
        if (!/jsx-a11y/.test(line)) return;
        const description = line.split('--')[1]?.trim() ?? '';
        if (description.length < 10) unexplained.push(`${path}:${index + 1}`);
      });
    }
    expect(unexplained, 'every jsx-a11y suppression must state its reason after `--`').toEqual([]);
  });
});
