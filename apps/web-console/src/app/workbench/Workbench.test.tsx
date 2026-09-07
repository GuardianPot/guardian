import { render, screen } from '@testing-library/react';
import { readFileSync } from 'node:fs';
import { MemoryRouter } from 'react-router';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { HOSTILE_CORPUS, FORBIDDEN_ELEMENTS } from '@shared/hostile/corpus';
import { expectNoAxeViolations } from '@shared/testing/axe';
import { routes } from '@app/router';
import { Workbench, WORKBENCH_MARKER } from './Workbench';

/**
 * The component workbench (WCX-06 sections 9.5, 9.7.4, and 10.1.10).
 *
 * Two separate claims are under test: that it renders everything it promises,
 * and that it cannot reach a production build. The second matters more — a
 * workbench that shipped would put a page rendering attack strings, and every
 * component in every state, on an operator's Control Plane.
 */
afterEach(() => vi.unstubAllGlobals());

describe('the workbench', () => {
  it('renders every data state, control, and status value from fixtures', () => {
    render(<MemoryRouter><Workbench /></MemoryRouter>);

    // The eight data states.
    expect(screen.getByText(/Loading the device inventory/)).toBeVisible();
    expect(screen.getByText('No Edge devices recorded')).toBeVisible();
    expect(screen.getByText('No observation exists')).toBeVisible();
    expect(screen.getByText('Showing the last data Guardian received')).toBeVisible();
    expect(screen.getByText('Some sources could not be read')).toBeVisible();
    expect(screen.getByText('The health projection is impaired')).toBeVisible();
    expect(screen.getByText('Access was refused')).toBeVisible();
    expect(screen.getByText('This data could not be loaded')).toBeVisible();

    // The four button variants and both unavailable treatments.
    for (const name of ['Primary', 'Secondary', 'Destructive', 'Quiet', 'Pending', 'Disabled']) {
      expect(screen.getByRole('button', { name }), name).toBeVisible();
    }

    // The unknown fallback for a status value the backend added later.
    expect(screen.getAllByText('Unknown').length).toBeGreaterThan(0);
  });

  it('renders the whole hostile corpus without creating a single element from it', async () => {
    const { container } = render(<MemoryRouter><Workbench /></MemoryRouter>);

    for (const fixture of HOSTILE_CORPUS) {
      expect(screen.getAllByText(fixture.id).length, fixture.id).toBeGreaterThan(0);
    }
    // The workbench draws status glyphs, which are `svg`, so those are excluded
    // from the element check; nothing else on the forbidden list may appear.
    const fromData = FORBIDDEN_ELEMENTS.split(', ').filter((element) => element !== 'svg').join(', ');
    expect(container.querySelectorAll(fromData)).toHaveLength(0);
    await Promise.resolve();
  });

  it('performs no network call', () => {
    const fetchMock = vi.fn(() => Promise.reject(new Error('the workbench must not call the network')));
    vi.stubGlobal('fetch', fetchMock);
    render(<MemoryRouter><Workbench /></MemoryRouter>);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('reports no serious or critical axe violation', async () => {
    const { container } = render(<MemoryRouter><Workbench /></MemoryRouter>);
    await expectNoAxeViolations(container);
  });
});

describe('production exclusion', () => {
  it('mounts the route only behind the development flag', () => {
    // First proof (section 9.5). Vite replaces `import.meta.env.DEV` with the
    // literal `false` in a production build, so Rollup evaluates the branch
    // away and never follows the dynamic import.
    const source = readFileSync('src/app/router.tsx', 'utf8');
    expect(source).toMatch(/import\.meta\.env\.DEV/);
    expect(source).toMatch(/await import\('@app\/workbench\/Workbench'\)/);

    // Under vitest the flag is true, so the route is really there — which is
    // what makes the production assertion meaningful rather than vacuous.
    const children = routes[0]?.children ?? [];
    expect(children.some((route) => route.path === '/__components')).toBe(true);
  });

  it('is named by a marker the bundle check looks for', () => {
    // Second proof. `check-bundle.mjs` fails the build if this string appears
    // in any production chunk, so the marker and the check must agree.
    const check = readFileSync('test/check-bundle.mjs', 'utf8');
    expect(check).toContain(WORKBENCH_MARKER);
    for (const fixture of ['rtl-override-filename', 'zero-width-keyword']) {
      expect(check, `${fixture} must be watched for in the bundle`).toContain(fixture);
    }
  });

  it('keeps its styles out of the shared stylesheet', () => {
    // CSS Modules does not tree-shake unused class rules, so a workbench class
    // living in `app.module.css` would ship in every production build even
    // though the component that uses it is dropped.
    const shared = readFileSync('src/shared/styles/app.module.css', 'utf8');
    expect(shared).not.toMatch(/\.workbench/i);
  });
});
