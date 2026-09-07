import { render, screen } from '@testing-library/react';
import { readFileSync } from 'node:fs';
import { describe, expect, it } from 'vitest';
import { CATALOGUE, plural, t, tx, type CatalogueKey } from './index';

/**
 * The text catalogue's own guarantees (WCX-08 section 10.1.1-5 and 13).
 *
 * The compiler enforces two of them, so those are proved by
 * `typeFixtures.test.ts` rather than here: this file covers what the compiler
 * cannot see — that interpolation produces text, that plural selection is
 * correct at the boundaries, that the catalogue carries no secret, and that it
 * stays small enough to be read in one sitting.
 */
const KEYS = Object.keys(CATALOGUE) as CatalogueKey[];

describe('interpolation', () => {
  it('inserts a value carrying markup as text, never as an element', () => {
    // Section 9.8.2. The accessor is not an HTML parser and must never
    // become one by accident, because backend text reaches sentences that
    // wrap a value.
    const hostile = '<img src=x onerror="alert(1)">';
    const filled = t('environments.total', { count: hostile });

    expect(filled).toContain('<img src=x onerror="alert(1)">');

    const { container } = render(<p>{filled}</p>);
    expect(container.querySelector('img')).toBeNull();
    expect(container.textContent).toContain(hostile);
  });

  it('renders a node placeholder as that node and the rest as text', () => {
    render(<p>{tx('common.sessionReadOnlyFull', { reauthenticate: <a href="/login">Re-authenticate</a> })}</p>);

    // The sentence survives as a sentence around the control.
    expect(screen.getByRole('link', { name: 'Re-authenticate' })).toBeInTheDocument();
    expect(document.body.textContent).toContain('Read-only session restored.');
    expect(document.body.textContent).toContain('before changing configuration or signing out.');
  });

  it('leaves an unsupplied placeholder visible rather than rendering "undefined"', () => {
    // Unreachable through the typed accessor; this is the runtime floor under
    // it. A visible `{count}` is a bug report. "undefined" reads like data.
    const filled = t('environments.total', {} as { count: string });
    expect(filled).toContain('{count}');
    expect(filled).not.toContain('undefined');
  });
});

describe('pluralisation', () => {
  it.each([
    [0, '0 zones'],
    [1, '1 zone'],
    [2, '2 zones'],
    [17, '17 zones'],
  ])('selects the right form for %i', (count, expected) => {
    expect(plural('environments.zoneCount', count)).toBe(expected);
  });

  it.each([
    [0, '0 minutes'],
    [1, '1 minute'],
    [5, '5 minutes'],
  ])('applies to durations too, at %i', (count, expected) => {
    expect(plural('time.age.minute', count)).toBe(expected);
  });

  it('has both forms for every plural base in the catalogue', () => {
    const missing = KEYS
      .filter((key) => key.endsWith('.one'))
      .map((key) => key.replace(/\.one$/, '.other'))
      .filter((other) => !(other in CATALOGUE));
    expect(missing, 'every `.one` entry needs its `.other`').toEqual([]);
  });
});

describe('catalogue hygiene', () => {
  const source = readFileSync('src/shared/text/catalogue.ts', 'utf8');

  /**
   * Section 9.8.3. Shapes, not a wordlist: an example credential is added by
   * someone illustrating a field, and they will not name it `password`.
   */
  const SECRET_SHAPES: readonly { name: string; pattern: RegExp }[] = [
    { name: 'a bearer or API token', pattern: /\b(?:bearer|token|secret|apikey|api[_-]key)\s*[:=]\s*\S/i },
    { name: 'a JWT', pattern: /\beyJ[A-Za-z0-9_-]{8,}\./ },
    { name: 'an AWS access key id', pattern: /\b(?:AKIA|ASIA)[0-9A-Z]{16}\b/ },
    { name: 'a PEM private key', pattern: /-----BEGIN [A-Z ]*PRIVATE KEY-----/ },
    { name: 'a base64 blob long enough to carry a credential', pattern: /\b[A-Za-z0-9+/]{40,}={0,2}\b/ },
    { name: 'a hex blob long enough to carry a credential', pattern: /\b[0-9a-f]{32,}\b/i },
    { name: 'a grouped recovery code', pattern: /\b[0-9a-z]{4,5}-[0-9a-z]{4,5}-[0-9a-z]{4,5}\b/i },
    { name: 'a URL carrying credentials', pattern: /\b[a-z][a-z0-9+.-]*:\/\/[^\s/]*:[^\s/]*@/i },
  ];

  it('contains no secret-shaped string', () => {
    const found: string[] = [];
    for (const [key, value] of Object.entries(CATALOGUE)) {
      for (const shape of SECRET_SHAPES) {
        if (shape.pattern.test(value)) found.push(`${key} looks like ${shape.name}: ${value}`);
      }
    }
    expect(found, 'a catalogue entry must never carry a credential, even as an example').toEqual([]);
  });

  it.each([
    // Assembled rather than written out. `tools/check-secrets.mjs` greps the
    // whole tracked tree for exactly these shapes, and a test proving a
    // scanner works must not itself be the thing a scanner finds.
    ['an AWS access key id', `${'AKIA'}IOSFODNN7EXAMPLE`],
    ['a JWT', `${'eyJ'}hbGciOiJIUzI1NiJ9.e30.sig`],
    ['a PEM private key', `${'-----BEGIN'} OPENSSH PRIVATE KEY-----`],
    ['a bearer token', 'token: abc123'],
    ['a grouped recovery code', 'k7v2n-9wqte-4bxrm'],
    ['a URL carrying credentials', 'https://owner:hunter2@guardian.example'],
  ])('detects %s if one were added, so the scan is not vacuous', (_name, planted) => {
    expect(SECRET_SHAPES.some((shape) => shape.pattern.test(planted))).toBe(true);
  });

  it('stays inside its size budget', () => {
    // The budget is on the shipped data, not on the file: the comments
    // explaining a decision are what make the catalogue reviewable and they do
    // not survive the bundler.
    //
    // `WCX-08` section 9.10 set 12 KiB as that package's exit condition and it
    // held there at 12,196 bytes. `WCX-09` adds five operator screens whose
    // whole point is stating an irreversible effect in plain words before it
    // happens — 19,592 bytes at the end of that package — and its own section
    // 9.10 binds the authenticated initial-load budget rather than this
    // number.
    //
    // So the ceiling moves with the scope. What does not move is that it is a
    // ceiling, and that the constraint an operator actually pays for is
    // enforced by `check-bundle.mjs`, which measures the shipped chunk.
    const shipped = Buffer.byteLength(JSON.stringify(CATALOGUE), 'utf8');
    expect(shipped).toBeLessThan(20 * 1024);
  });

  it('composes no entry from another entry', () => {
    // Section 9.1.6. A sentence assembled at runtime cannot be reviewed as a
    // sentence and cannot later be translated.
    expect(source).not.toMatch(/CATALOGUE\[/);
    expect(source).not.toMatch(/\$\{/);
  });

  it('names keys after meaning, in a known namespace', () => {
    const NAMESPACES = [
      'common', 'auth', 'account', 'environments', 'environment', 'devices', 'health',
      'states', 'confirm', 'stepUp', 'secret', 'untrusted', 'time', 'errors',
    ];
    const stray = KEYS.filter((key) => !NAMESPACES.includes(key.split('.')[0] ?? ''));
    expect(stray, 'add the namespace to this list and to the runbook first').toEqual([]);
  });

  it('holds no empty entry', () => {
    expect(KEYS.filter((key) => CATALOGUE[key].trim() === '')).toEqual([]);
  });

  it('describes absence without describing health', () => {
    // Section 9.2.2 and 9.2.4. The product's whole claim is that
    // configuration completeness is not health; the words have to hold it.
    const HEALTH_CLAIMS = /\b(all good|everything(?:'s| is) fine|no (?:issues|problems)|you'?re all set|looks good|all clear)\b/i;
    const offenders = KEYS.filter((key) => HEALTH_CLAIMS.test(CATALOGUE[key]));
    expect(offenders, 'absence of an observation is never a healthy result').toEqual([]);
  });
});
