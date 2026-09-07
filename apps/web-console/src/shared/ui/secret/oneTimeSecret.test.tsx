import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it, vi } from 'vitest';
import { expectNoAxeViolations } from '@shared/testing/axe';
import { OneTimeSecretDialog } from './OneTimeSecretDialog';

/**
 * One-time bootstrap material (W11-C3-A, WCX-09 sections 8.11 and 10.1.14).
 *
 * Two kinds of assertion here, deliberately. The behavioural ones prove the
 * value leaves when it should. The source-level one proves something stronger
 * and different: that **no read path exists**. A component can only re-display
 * a secret it can reach, and the point of the rule is that nothing can reach
 * one — which is a property of the code, not of any particular render.
 */
const SECRET = { token: 'one-time-bootstrap-value', expires_at: '2026-08-29T12:15:00Z' };

const props = {
  titleKey: 'environment.secret.title',
  descriptionKey: 'environment.secret.description',
  labelKey: 'environment.secret.label',
} as const;

describe('the dialog', () => {
  it('shows the value once, with a second-precision expiry', async () => {
    render(<OneTimeSecretDialog secret={SECRET} onDismiss={() => undefined} {...props} />);

    const box = await screen.findByTestId('one-time-secret');
    expect(box).toHaveTextContent(SECRET.token);
    // A 15-minute window is not actionable to the minute.
    expect(box.querySelector('time')?.textContent).toMatch(/:\d\d:\d\d/);
  });

  it('renders nothing at all without a secret', () => {
    const { baseElement } = render(
      <OneTimeSecretDialog secret={null} onDismiss={() => undefined} {...props} />,
    );
    expect(baseElement.querySelector('[data-testid="one-time-secret"]')).toBeNull();
    expect(baseElement.textContent).not.toContain(SECRET.token);
  });

  it('asks the caller to drop the value on dismissal', async () => {
    const dismiss = vi.fn();
    render(<OneTimeSecretDialog secret={SECRET} onDismiss={dismiss} {...props} />);

    await userEvent.click(await screen.findByRole('button', { name: 'I have stored it securely' }));
    expect(dismiss).toHaveBeenCalledTimes(1);
  });

  it('drops the value when the page is being unloaded', () => {
    // A closed or reloaded tab must not leave the value in a restored page or
    // a back-forward cache entry.
    const dismiss = vi.fn();
    render(<OneTimeSecretDialog secret={SECRET} onDismiss={dismiss} {...props} />);

    window.dispatchEvent(new Event('pagehide'));
    expect(dismiss).toHaveBeenCalledTimes(1);
  });

  it('writes nothing to browser storage', async () => {
    const setItem = vi.spyOn(Storage.prototype, 'setItem');
    render(<OneTimeSecretDialog secret={SECRET} onDismiss={() => undefined} {...props} />);
    await screen.findByTestId('one-time-secret');

    expect(setItem).not.toHaveBeenCalled();
    expect(localStorage).toHaveLength(0);
    expect(sessionStorage).toHaveLength(0);
    setItem.mockRestore();
  });

  it('reports no serious or critical axe violation', async () => {
    const { baseElement } = render(
      <OneTimeSecretDialog secret={SECRET} onDismiss={() => undefined} {...props} />,
    );
    await screen.findByRole('dialog');
    await expectNoAxeViolations(baseElement);
  });
});

const walk = (dir: string): string[] =>
  readdirSync(dir).flatMap((entry) => {
    const path = join(dir, entry);
    return statSync(path).isDirectory() ? walk(path) : [path];
  });

/** Source with comments stripped, so a rule cannot fire on its own prose. */
const codeOf = (path: string): string =>
  readFileSync(path, 'utf8').replace(/\/\*[\s\S]*?\*\//g, '').replace(/^\s*\/\/.*$/gm, '');

describe('no path exists to read one-time material back', () => {
  const sources = walk('src')
    .filter((file) => /\.tsx?$/.test(file) && !file.includes('.test.'))
    .map((file) => [file, codeOf(file)] as const);

  it('caches no response that carries a token value', () => {
    // Section 8.11. Both producers bypass the query cache; a `queryOptions`
    // over either endpoint would put a secret somewhere it can be read again.
    //
    // A bounded window after each `queryOptions(` rather than a whole-file
    // match: `devices/api.ts` legitimately holds both a query and the issuing
    // mutation, and a file-wide regex cannot tell the two apart.
    const cached = sources.filter(([, code]) =>
      code.split('queryOptions(').slice(1)
        .some((region) => /re-enrollment-token|auth\/password/.test(region.slice(0, 400))));
    expect(cached.map(([file]) => file), 'one-time material must not enter the query cache').toEqual([]);
  });

  it('offers no copy, reveal, or re-display control for a secret', () => {
    const offenders = sources.filter(([file, code]) =>
      file.includes(join('ui', 'secret')) && /clipboard|navigator\.|reveal|showAgain|toggle/i.test(code));
    expect(offenders.map(([file]) => file)).toEqual([]);
  });

  it('holds the value in a prop and never in state or a module variable', () => {
    const [, dialog] = sources.find(([file]) => file.endsWith('OneTimeSecretDialog.tsx')) ?? [];
    expect(dialog, 'the dialog must exist').toBeDefined();
    // No `useState`, no `useRef`, no module-scope binding for the value: the
    // caller owns it, so route exit destroys it.
    expect(dialog).not.toMatch(/useState|useRef/);
  });

  it('reaches a token only through the two functions that issue one', () => {
    // `token` is the field name in `EnrollmentTokenSecret`. Anything reading
    // it outside the issuing call sites and the dialog is a second read path.
    const readers = sources
      .filter(([, code]) => /\.token\b/.test(code))
      .map(([file]) => file.replace(/\\/g, '/'));
    // Exactly two: the screen that hands an issued token straight to the
    // dialog, and the dialog that renders it. A third entry is a second read
    // path and needs a reason before it joins this list.
    expect(readers.sort()).toEqual([
      'src/features/devices/DeviceLifecycle.tsx',
      'src/shared/ui/secret/OneTimeSecretDialog.tsx',
    ]);
  });
});
