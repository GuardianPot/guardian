import axe, { type ImpactValue, type Result } from 'axe-core';
import { expect } from 'vitest';

/**
 * Component-level accessibility assertion (WCX-05 section 9.4, WC-D23).
 *
 * The second of three enforcement layers. `jsx-a11y` catches what is visible
 * in the source; this catches what is only visible once a component has
 * rendered — a name that resolves to nothing, a role that conflicts with its
 * markup, a control whose description points at an id that is not there. The
 * third layer is the full-page browser scan in `full.yml`, which is the only
 * one that sees the real cascade.
 *
 * `serious` and `critical` fail. `moderate` and `minor` are collected and
 * printed at the end of the run so a regression is visible without turning
 * every best-practice heuristic into a build break.
 */
const FAILING: ReadonlySet<ImpactValue> = new Set<ImpactValue>(['serious', 'critical']);

/**
 * Rules that cannot produce a truthful answer in jsdom.
 *
 * `color-contrast` needs a rendering engine. jsdom resolves no cascade and no
 * composited background, so axe either skips it or measures a colour the
 * operator never sees — the exact mistake `WCX-03` shipped as a real failure.
 * Contrast is covered by the token table in `@shared/theme/tokens.ts` and by
 * the browser scan, both of which measure what is actually painted.
 */
const JSDOM_BLIND_RULES = ['color-contrast'] as const;

export type AxeFinding = {
  id: string;
  impact: ImpactValue | null | undefined;
  help: string;
  targets: string[];
};

const advisory = new Map<string, AxeFinding>();

const toFinding = (result: Result): AxeFinding => ({
  id: result.id,
  impact: result.impact,
  help: result.help,
  targets: result.nodes.map((node) => node.target.join(' ')),
});

const describe = (finding: AxeFinding): string =>
  `${finding.id} (${finding.impact ?? 'unknown'}): ${finding.help}\n    ${finding.targets.join('\n    ')}`;

/**
 * Runs axe over a rendered container and fails on a serious or critical
 * violation.
 *
 * Pass the `container` from `render`, or `document.body` when the component
 * portals content outside it — a dialog does.
 */
export async function expectNoAxeViolations(container: Element): Promise<void> {
  const results = await axe.run(container, {
    rules: Object.fromEntries(JSDOM_BLIND_RULES.map((rule) => [rule, { enabled: false }])),
    resultTypes: ['violations'],
  });

  const findings = results.violations.map(toFinding);
  for (const finding of findings) {
    if (!FAILING.has(finding.impact as ImpactValue)) advisory.set(finding.id, finding);
  }

  const blocking = findings.filter((finding) => FAILING.has(finding.impact as ImpactValue));
  expect(
    blocking.map(describe),
    'axe reported a serious or critical accessibility violation',
  ).toEqual([]);
}

/** Every moderate and minor finding seen so far, for the reporting test. */
export function advisoryAxeFindings(): readonly AxeFinding[] {
  return [...advisory.values()];
}

export function resetAdvisoryAxeFindings(): void {
  advisory.clear();
}
