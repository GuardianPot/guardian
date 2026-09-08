import { screen, within } from '@testing-library/react';
import { readdirSync, readFileSync, statSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';
import { deviceID, environmentID } from '@shared/testing/harness';
import { renderApp } from '@shared/testing/renderApp';
import { CATALOGUE } from '@shared/text';

/**
 * The route tree, the home placeholder, and the breadcrumbs (WCX-10 sections
 * 9.1, 9.4, 8.5, tests 10.1.1, 9, and 10).
 *
 * The placeholder is the part worth the most attention. A blank incident
 * dashboard is not neutral: an operator who glances at it and comes away
 * believing Guardian looked and found nothing is worse off than one who never
 * opened it, and that is exactly the failure this product exists to prevent,
 * reproduced in its own shell. So the assertions below are mostly negative.
 */
describe('every previously bookmarkable path', () => {
  it.each([
    ['/', 'Home'],
    ['/environments', 'Environments'],
    [`/environments/${environmentID}`, 'Lab'],
    [`/environments/${environmentID}/devices/${deviceID}`, 'edge-one'],
    ['/account', 'Account'],
  ])('resolves %s to its screen', async (entry, heading) => {
    // Test 10.1.1. No path moved in this package; what changed is that `/` is
    // now a screen rather than a redirect, and an unknown path is a screen
    // rather than a redirect. Everything an operator had bookmarked still
    // lands where it did.
    renderApp(entry);
    expect(await screen.findByRole('heading', { name: heading, level: 1 })).toBeVisible();
  });
});

describe('the home placeholder', () => {
  async function openHome() {
    const view = renderApp('/');
    await screen.findByRole('heading', { name: 'Home', level: 1 });
    return view;
  }

  it('says the dashboard is not built rather than showing an empty one', async () => {
    await openHome();
    expect(screen.getByRole('heading', { name: /incident dashboard arrives in a later phase/ })).toBeVisible();
    expect(screen.getByText(/absence of content on this page is a fact about the console/)).toBeVisible();
  });

  it('offers the two screens that do work', async () => {
    await openHome();
    const links = within(screen.getByRole('navigation', { name: 'Screens that are built' }));
    expect(links.getByRole('link', { name: 'Environments' })).toBeVisible();
    expect(links.getByRole('link', { name: 'Account' })).toBeVisible();
  });

  it('shows no incident count, no incident list, and no claim of absence', async () => {
    // Test 10.1.9, asserted negatively and deliberately broadly. Anything
    // that could be read as "Guardian looked and found nothing" fails here.
    const { container } = await openHome();
    const text = container.textContent ?? '';

    expect(text).not.toMatch(/\b\d+\s+(?:incident|detection|alert|event)/i);
    expect(text).not.toMatch(/\bno (?:incidents?|detections?|alerts?|threats?|activity)\b/i);
    expect(text).not.toMatch(/\b(?:all clear|nothing detected|nothing to report|you'?re all set|looks good|all good)\b/i);
    expect(text).not.toMatch(/\b(?:secure|protected|safe|healthy)\b/i);
    // No list that could be mistaken for an empty incident feed. The only
    // list-shaped thing here is the pair of entry-point links.
    expect(container.querySelectorAll('ul, ol, table')).toHaveLength(0);
  });

  it('states no scope when none is selected', async () => {
    await openHome();
    expect(screen.queryByRole('region', { name: 'Current environment' })).toBeNull();
  });
});

describe('breadcrumbs', () => {
  it('name the whole path on a device screen', async () => {
    // Section 9.4.2. A single back link left the environment unnamed, so an
    // operator two levels deep could not read their position off the page.
    renderApp(`/environments/${environmentID}/devices/${deviceID}`);
    await screen.findByRole('heading', { name: 'edge-one', level: 1 });

    const trail = within(screen.getByRole('navigation', { name: 'Breadcrumb' }));
    expect(trail.getByRole('link', { name: 'Environments' })).toHaveAttribute('href', '/environments');
    expect(trail.getByRole('link', { name: 'Environment' }))
      .toHaveAttribute('href', `/environments/${environmentID}`);
    // The current screen is not a link: a link to where you already are is a
    // control that does nothing. `closest` because an untrusted name renders
    // inside its own element, so the text node is a descendant of the crumb.
    expect(trail.getByText('edge-one').closest('[aria-current="page"]')).not.toBeNull();
    expect(trail.queryByRole('link', { name: 'edge-one' })).toBeNull();
  });

  it('name the environment on an environment screen', async () => {
    renderApp(`/environments/${environmentID}`);
    await screen.findByRole('heading', { name: 'Lab', level: 1 });

    const trail = within(screen.getByRole('navigation', { name: 'Breadcrumb' }));
    expect(trail.getByRole('link', { name: 'Environments' })).toBeVisible();
    expect(trail.getByText('Lab').closest('[aria-current="page"]')).not.toBeNull();
    expect(trail.queryByRole('link', { name: 'Lab' })).toBeNull();
  });
});

const walk = (dir: string): string[] =>
  readdirSync(dir).flatMap((entry) => {
    const path = join(dir, entry);
    return statSync(path).isDirectory() ? walk(path) : [path];
  });

describe('redirect safety', () => {
  it('derives no navigation target from a query parameter', () => {
    // Test 10.1.10 and section 8.5. An open redirect is built by feeding a
    // parameter into a destination; the console never does, so the check is
    // that no `to=` or `Navigate` reads one.
    const sources = walk('src')
      .filter((file) => /\.tsx?$/.test(file) && !file.includes('.test.'))
      .map((file) => [file, readFileSync(file, 'utf8')] as const);

    const offenders = sources.filter(([, code]) =>
      /(?:to|href)=\{[^}]*(?:searchParams|useSearchParams|location\.search|scope\.raw)/.test(code)
      || /<Navigate[^>]*to=\{[^}]*(?:searchParams|location\.search)/.test(code));
    expect(offenders.map(([file]) => file), 'a redirect target must be statically known').toEqual([]);
  });

  it('names no navigation entry that has no screen', () => {
    // Section 6 and 9.1. A navigation entry pointing at a screen that does
    // not exist is a promise the console cannot keep.
    const entries = ['home.heading', 'environments.heading', 'account.heading'] as const;
    for (const key of entries) expect(CATALOGUE[key]).toBeTruthy();
    // Nothing in the catalogue offers a surface that does not exist yet.
    // `Decoys` left this list in `WCX-11`, when the screen behind it shipped;
    // the rest stay until theirs do.
    const labels = Object.values(CATALOGUE).join('\n');
    expect(labels).not.toMatch(/^(?:Incidents|Notifications)$/m);
  });
});
