import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { readFileSync } from 'node:fs';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { reveal, untrusted } from '@shared/api/untrusted';
import {
  FORBIDDEN_ATTRIBUTES,
  FORBIDDEN_ELEMENTS,
  HOSTILE_CORPUS,
} from '@shared/hostile/corpus';
import { expectNoAxeViolations } from '@shared/testing/axe';
import { UntrustedText } from './UntrustedText';
import { UntrustedBlock } from './UntrustedBlock';
import { BLOCK_LIMIT, TEXT_LIMIT, transformUntrusted } from './transform';

/**
 * The rendering contract (WCX-06 section 10.1, `SEC-08`, `RE-12`).
 *
 * React escaping is assumed, not tested: what is tested is everything React
 * escaping does not do. A right-to-left override, an ANSI sequence, and a
 * zero-width character all survive escaping intact and all forge what an
 * operator sees.
 */
afterEach(() => vi.unstubAllGlobals());

const cp = (code: number): string => String.fromCodePoint(code);

describe('every fixture in the corpus', () => {
  it.each(HOSTILE_CORPUS)('renders inertly through UntrustedText: $id', ({ value, intent }) => {
    const { container } = render(<UntrustedText value={untrusted(value)} />);

    expect(container.querySelectorAll(FORBIDDEN_ELEMENTS), intent).toHaveLength(0);
    for (const element of container.querySelectorAll('*')) {
      for (const attribute of element.attributes) {
        expect(attribute.name.toLowerCase(), `${element.tagName}[${attribute.name}]`).not.toMatch(/^on/);
        expect(FORBIDDEN_ATTRIBUTES, `${element.tagName}[${attribute.name}]`).not.toContain(
          attribute.name.toLowerCase(),
        );
      }
    }
  });

  it.each(HOSTILE_CORPUS)('renders inertly through UntrustedBlock: $id', ({ value, intent }) => {
    const { container } = render(<UntrustedBlock value={untrusted(value)} />);

    // The block carries a copy control, which is a button this test rendered
    // rather than something the value produced.
    expect(container.querySelectorAll(FORBIDDEN_ELEMENTS), intent).toHaveLength(0);
    for (const element of container.querySelectorAll('*')) {
      for (const attribute of element.attributes) {
        expect(attribute.name.toLowerCase()).not.toMatch(/^on/);
        expect(FORBIDDEN_ATTRIBUTES).not.toContain(attribute.name.toLowerCase());
      }
    }
  });

  it('keeps the corpus source reviewable', () => {
    // A fixture file containing literal invisible bytes cannot be reviewed,
    // and an unreviewable corpus is not evidence of anything.
    const source = readFileSync('src/shared/hostile/corpus.ts', 'utf8');
    const offending = [...source].filter((character) => {
      const code = character.codePointAt(0) ?? 0;
      if (code === 0x0a || code === 0x0d || code === 0x09) return false;
      return code < 0x20 || (code >= 0x7f && code <= 0x9f) || (code >= 0x200b && code <= 0x200f)
        || (code >= 0x2060 && code <= 0x2069) || (code >= 0x202a && code <= 0x202e) || code === 0xfeff;
    });
    expect(offending, 'build invisible characters with cp() instead of typing them').toEqual([]);
  });
});

describe('ANSI sequences', () => {
  it('appear as escaped source and produce no styling', () => {
    const { container } = render(
      <UntrustedBlock value={untrusted(`${cp(0x1b)}[32mALL CHECKS PASSED${cp(0x1b)}[0m`)} />,
    );

    expect(container.textContent).toContain('\\x1b[32mALL CHECKS PASSED');
    expect(container.textContent).not.toContain(cp(0x1b));
    // Nothing anywhere acquired a colour from the sequence.
    for (const element of container.querySelectorAll('*')) {
      expect(element.getAttribute('style')).toBeNull();
    }
  });

  it('shows a cursor-control sequence rather than obeying it', () => {
    render(<UntrustedText value={untrusted(`denied${cp(0x1b)}[2K${cp(0x1b)}[1Agranted`)} />);
    // Both words survive: the overwrite never happened.
    expect(document.body.textContent).toContain('denied');
    expect(document.body.textContent).toContain('granted');
    expect(document.body.textContent).toContain('\\x1b[2K');
  });
});

describe('directional and invisible characters', () => {
  it('does not let a right-to-left override reorder a filename', () => {
    const { container } = render(<UntrustedText value={untrusted(`report${cp(0x202e)}gnp.exe`)} />);

    // The override is gone from the text and replaced by its escaped source,
    // so the browser has nothing to reorder and `.exe` stays at the end.
    expect(container.textContent).not.toContain(cp(0x202e));
    expect(container.textContent).toContain('\\u202e');
    expect(container.textContent?.endsWith('gnp.exe')).toBe(true);
    expect(screen.getByRole('img', { name: /right-to-left override/ })).toBeInTheDocument();
  });

  it('makes a zero-width character visible', () => {
    const { container } = render(<UntrustedText value={untrusted(`pass${cp(0x200b)}word`)} />);

    expect(container.textContent).not.toContain(cp(0x200b));
    expect(container.textContent).toContain('\\u200b');
    expect(screen.getByRole('img', { name: /zero-width space/ })).toBeInTheDocument();
  });

  it('describes an escape so a screen reader learns a character was there', () => {
    render(<UntrustedText value={untrusted(`edge${cp(0x00)}one`)} />);
    expect(screen.getByRole('img', { name: 'escaped null byte' })).toBeInTheDocument();
  });
});

describe('line breaks', () => {
  it('keeps newline and tab in a block, because they are structure', () => {
    const { container } = render(<UntrustedBlock value={untrusted('GET / HTTP/1.1\nHost: decoy\n\tX: 1')} />);
    expect(container.textContent).toContain('\n');
    expect(container.textContent).toContain('\t');
  });

  it('escapes them in a single-line value, which cannot contain them', () => {
    const { container } = render(<UntrustedText value={untrusted('edge-one\r\nX-Injected: yes')} />);
    expect(container.textContent).not.toContain('\n');
    expect(container.textContent).toContain('\\x0d');
    expect(container.textContent).toContain('\\x0a');
  });
});

describe('length bounds', () => {
  it('truncates a long value and states the original length', () => {
    const value = 'a'.repeat(TEXT_LIMIT + 250);
    render(<UntrustedText value={untrusted(value)} />);
    expect(screen.getByText(new RegExp(`first ${TEXT_LIMIT} of ${TEXT_LIMIT + 250} characters`))).toBeVisible();
  });

  it('never truncates silently', () => {
    const transformed = transformUntrusted('a'.repeat(TEXT_LIMIT + 1), { limit: TEXT_LIMIT, allowLineBreaks: false });
    expect(transformed.truncated).toBe(true);
    expect(transformed.originalLength).toBe(TEXT_LIMIT + 1);
  });

  it('leaves a value inside the bound whole', () => {
    const value = 'a'.repeat(TEXT_LIMIT);
    const { container } = render(<UntrustedText value={untrusted(value)} />);
    expect(container.textContent).toBe(value);
    expect(screen.queryByText(/Showing the first/)).toBeNull();
  });

  it('counts code points, so an astral character is never split in half', () => {
    // Four code points, eight UTF-16 units. A byte or unit bound would cut one
    // in half and render a replacement glyph that was never in the evidence.
    const emoji = '🙂🙂🙂🙂';
    const transformed = transformUntrusted(emoji, { limit: 2, allowLineBreaks: false });
    expect(transformed.originalLength).toBe(4);
    expect(transformed.segments.map((segment) => segment.value).join('')).toBe('🙂🙂');
  });

  it('bounds a block far higher than a single-line value', () => {
    expect(BLOCK_LIMIT).toBeGreaterThan(TEXT_LIMIT);
  });
});

describe('linkification', () => {
  it.each(['javascript:alert(1)', 'https://attacker.invalid/x', 'data:text/html,<script>alert(1)</script>'])(
    'never turns %s into a link',
    (value) => {
      const { container } = render(<UntrustedText value={untrusted(value)} />);
      expect(container.querySelector('a')).toBeNull();
      expect(container.textContent).toContain(value.slice(0, 20));
    },
  );

  it('never offers a filename as a download', () => {
    const { container } = render(<UntrustedText value={untrusted('invoice.pdf.exe')} />);
    expect(container.querySelector('[download]')).toBeNull();
    expect(container.querySelector('a')).toBeNull();
  });
});

describe('copying', () => {
  it('copies the original value, not the escaped rendering', async () => {
    const writeText = vi.fn(() => Promise.resolve());
    vi.stubGlobal('navigator', { ...navigator, clipboard: { writeText } });
    const original = `${cp(0x1b)}[32mgranted${cp(0x1b)}[0m`;

    render(<UntrustedBlock value={untrusted(original)} />);
    await userEvent.click(screen.getByRole('button', { name: /Copy the original untrusted value/ }));

    expect(writeText).toHaveBeenCalledWith(original);
    expect(await screen.findByText('Copied the original value to the clipboard.')).toBeVisible();
  });

  it('reports a failure rather than appearing to succeed', async () => {
    vi.stubGlobal('navigator', {
      ...navigator,
      clipboard: { writeText: () => Promise.reject(new Error('denied')) },
    });

    render(<UntrustedBlock value={untrusted('captured')} />);
    await userEvent.click(screen.getByRole('button', { name: /Copy the original untrusted value/ }));

    expect(await screen.findByRole('alert')).toHaveTextContent('The clipboard is unavailable');
  });

  it('names what is copied and that it is untrusted', () => {
    render(<UntrustedBlock value={untrusted('captured')} />);
    expect(screen.getByRole('button', { name: 'Copy the original untrusted value' })).toBeVisible();
  });
});

describe('accessibility', () => {
  it('gives the block a keyboard-reachable named scroll container', () => {
    render(<UntrustedBlock value={untrusted('captured payload')} label="HTTP request body" />);
    const region = screen.getByRole('group', { name: 'HTTP request body' });
    expect(region).toHaveAttribute('tabindex', '0');
  });

  it('explains the escapes once per block rather than per character', () => {
    render(<UntrustedBlock value={untrusted(`${cp(0x1b)}[0m${cp(0x1b)}[1m`)} />);
    expect(screen.getAllByText(/were control or invisible characters/)).toHaveLength(1);
  });

  it('omits the legend when nothing was escaped', () => {
    render(<UntrustedBlock value={untrusted('plain captured text')} />);
    expect(screen.queryByText(/were control or invisible characters/)).toBeNull();
  });

  it('reports no serious or critical axe violation', async () => {
    const { container } = render(
      <main>
        <UntrustedText value={untrusted(`report${cp(0x202e)}gnp.exe`)} />
        <UntrustedBlock value={untrusted(`${cp(0x1b)}[32mok${cp(0x1b)}[0m`)} label="Captured transcript" />
      </main>,
    );
    await expectNoAxeViolations(container);
  });
});

describe('compiler enforcement', () => {
  /**
   * Section 10.1.8, proved by `@ts-expect-error` rather than by a fixture the
   * build has to be told to ignore. Each directive *fails the typecheck if the
   * line below it compiles*, so if the brand ever stops working these turn
   * from passing assertions into build errors. `tsc -b` runs them in CI on
   * every commit; no extra tooling, no excluded directory.
   */
  it('refuses to let an untrusted value reach JSX', () => {
    const value = untrusted('edge-one');
    // @ts-expect-error an untrusted value is not a ReactNode
    const rendered = <span>{value}</span>;
    expect(rendered).toBeDefined();
  });

  it('refuses to let one become an attribute', () => {
    const value = untrusted('/etc/passwd');
    // @ts-expect-error an untrusted value is not a string
    const rendered = <span title={value} />;
    expect(rendered).toBeDefined();
  });

  it('refuses to let one be assigned to a plain string', () => {
    const value = untrusted('edge-one');
    // @ts-expect-error the brand is the point: it does not widen back to string
    const plain: string = value;
    expect(plain).toBeDefined();
  });

  it('leaves string coercion to the type-aware lint rules, which are active', { timeout: 60_000 }, async () => {
    // TypeScript itself permits `'x' + anObject` and `` `${anObject}` `` — both
    // produce a string, so `tsc` has nothing to complain about and a
    // `@ts-expect-error` there would be an unused directive. The rules that do
    // catch it are type-aware ESLint rules, so the enforcement is asserted
    // where it actually lives rather than assumed.
    const { ESLint } = await import('eslint');
    const config = (await new ESLint().calculateConfigForFile('src/app/Shell.tsx')) as {
      rules?: Record<string, unknown>;
    };
    for (const rule of [
      '@typescript-eslint/restrict-template-expressions',
      '@typescript-eslint/restrict-plus-operands',
      '@typescript-eslint/no-base-to-string',
    ]) {
      const configured = config.rules?.[rule];
      const severity: unknown = Array.isArray(configured) ? (configured as unknown[])[0] : configured;
      expect(severity, `${rule} must be an error`).toBe(2);
    }
  });

  it('lets the renderer read the original through the single escape hatch', () => {
    // `reveal` exists because the renderer and the clipboard need it. That it
    // compiles here is the counterpart to the four failures above: the brand
    // routes values, it does not lock them away.
    expect(reveal(untrusted('edge-one'))).toBe('edge-one');
  });
});

describe('failure behaviour', () => {
  it('escapes an unpaired surrogate rather than throwing', () => {
    const { container } = render(<UntrustedText value={untrusted(`edge${cp(0xd800)}one`)} />);
    expect(container.textContent).toContain('edge');
    expect(container.textContent).toContain('one');
    expect(container.textContent).toContain('\\ud800');
  });

  it('renders an empty value without throwing', () => {
    const { container } = render(<UntrustedText value={untrusted('')} />);
    expect(container.textContent).toBe('');
  });
});
