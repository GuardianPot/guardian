import ts from 'typescript';
import { mkdtempSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

/**
 * The catalogue's compile-time guarantees (WCX-08 section 10.1.2-3).
 *
 * An unknown key and a missing interpolation value must fail the build, and
 * "must fail" is only provable by compiling something that does. A
 * `@ts-expect-error` in a checked-in file proves the same thing more cheaply,
 * but it proves it to `tsc -b` rather than to the test suite, and a reader of
 * the suite would have to take the guarantee on trust.
 *
 * So the fixtures are compiled here, from a temporary directory, against the
 * real accessor. Nothing broken is ever left on disk for the ordinary build to
 * trip over.
 */
const SRC = resolve('src');

/** Diagnostics from type-checking one fixture module against the real source. */
function compile(fixture: string): readonly ts.Diagnostic[] {
  const directory = mkdtempSync(join(tmpdir(), 'wcx08-text-'));
  const file = join(directory, 'fixture.ts');
  writeFileSync(file, fixture, 'utf8');

  const program = ts.createProgram([file], {
    target: ts.ScriptTarget.ES2022,
    module: ts.ModuleKind.ESNext,
    moduleResolution: ts.ModuleResolutionKind.Bundler,
    jsx: ts.JsxEmit.ReactJSX,
    strict: true,
    exactOptionalPropertyTypes: true,
    noUncheckedIndexedAccess: true,
    noEmit: true,
    skipLibCheck: true,
    // The fixture lives outside `src`, so the alias has to be spelled out.
    baseUrl: SRC,
    paths: { '@shared/*': [join(SRC, 'shared', '*')] },
    types: [],
  });

  return ts.getPreEmitDiagnostics(program).filter((diagnostic) => diagnostic.file?.fileName === file.replace(/\\/g, '/'));
}

const messages = (diagnostics: readonly ts.Diagnostic[]): string =>
  diagnostics.map((d) => ts.flattenDiagnosticMessageText(d.messageText, ' ')).join('\n');

describe('the accessor rejects at compile time', () => {
  // Building a program is real work; the default five seconds is not enough
  // under a parallel run.
  const SLOW = { timeout: 60_000 };

  it('a key that is not in the catalogue', SLOW, () => {
    const diagnostics = compile([
      "import { t } from '@shared/text';",
      "export const renamed = t('environments.headingg');",
    ].join('\n'));

    expect(diagnostics.length, 'a typo in a key must not compile').toBeGreaterThan(0);
  });

  it('a key that is not in the catalogue, named in the message', SLOW, () => {
    // The call above fails on argument arity, because an unresolvable key
    // leaves the accessor unable to derive its placeholders. Checking the
    // constraint directly is what produces a diagnostic a reader can act on.
    const diagnostics = compile([
      "import type { CatalogueKey } from '@shared/text';",
      "export const renamed: CatalogueKey = 'environments.headingg';",
    ].join('\n'));

    expect(messages(diagnostics)).toContain('environments.headingg');
  });

  it('an omitted interpolation value', SLOW, () => {
    const diagnostics = compile([
      "import { t } from '@shared/text';",
      // `environments.total` is `{count} total`.
      "export const missing = t('environments.total');",
    ].join('\n'));

    expect(diagnostics.length, 'a placeholder without a value must not compile').toBeGreaterThan(0);
  });

  it('an interpolation value the entry does not ask for', SLOW, () => {
    const diagnostics = compile([
      "import { t } from '@shared/text';",
      "export const extra = t('environments.total', { count: 3, zone: 'a' });",
    ].join('\n'));

    expect(diagnostics.length, 'a stale value after a rewording must not compile').toBeGreaterThan(0);
  });

  it('a plural base that has no forms', SLOW, () => {
    const diagnostics = compile([
      "import { plural } from '@shared/text';",
      "export const wrong = plural('environments.total', 2);",
    ].join('\n'));

    expect(diagnostics.length, 'only keys with `.one` and `.other` are plural bases').toBeGreaterThan(0);
  });

  it('but accepts the correct calls, so the fixtures prove something', SLOW, () => {
    const diagnostics = compile([
      "import { plural, t } from '@shared/text';",
      "export const heading = t('environments.heading');",
      "export const total = t('environments.total', { count: 3 });",
      "export const zones = plural('environments.zoneCount', 2);",
    ].join('\n'));

    expect(messages(diagnostics)).toBe('');
  });
});
