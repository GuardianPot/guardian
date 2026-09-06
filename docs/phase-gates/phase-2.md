# Phase 2 gate

Phase 2 turns the Web Console from the Phase 1 onboarding surface into the
incident-first operator console described in the extended package set. It is
not complete until every approved Phase 2 package has acceptance evidence and
the Product Owner closes this gate.

## Entry dependencies

- Phase 1 gate is Product Owner approved and closed (2026-09-04).
- The extended Web Console package set `WCX-01` through `WCX-21` is Product
  Owner approved and recorded in
  [`00-master-decision-record.md`](../work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/00-master-decision-record.md).
- Owner decisions `WC-D01` through `WC-D32` are closed; the register records
  zero open material decisions.

## Required exit evidence

Per the master decision record section 8. Summarised:

- module boundaries, generated API contract, and the capability seam;
- design tokens, severity semantics, theme and motion policy;
- incident-first navigation, list, and detail surfaces;
- evidence rendering with hostile content treated as untrusted throughout;
- the AI assist surface, gated behind evidence;
- operator text catalogue, freshness policy, and error surfacing;
- accessibility conformance at WCAG 2.2 AA across the shipped surfaces;
- retention, notification, update, and diagnostics configuration.

## Work-package evidence

Merging is not acceptance. A row reaches `Accepted` only when the Product
Owner approves the package's acceptance evidence, which is recorded here with
the date of that approval.

| Package | Issue | PR | Acceptance state |
|---|---|---|---|
| WCX-01 | [#63](https://github.com/GuardianPot/guardian/issues/63) | [#85](https://github.com/GuardianPot/guardian/pull/85) | Accepted 2026-09-04 |
| WCX-02 | [#64](https://github.com/GuardianPot/guardian/issues/64) | [#86](https://github.com/GuardianPot/guardian/pull/86) | Accepted 2026-09-05 |
| WCX-03 | [#65](https://github.com/GuardianPot/guardian/issues/65) | [#87](https://github.com/GuardianPot/guardian/pull/87), [#89](https://github.com/GuardianPot/guardian/pull/89) | Accepted 2026-09-06 |
| WCX-04 through WCX-21 | [#66](https://github.com/GuardianPot/guardian/issues/66)–[#83](https://github.com/GuardianPot/guardian/issues/83) | — | Not started |

### WCX-01 — module boundaries and the capability seam

Feature-sliced tree, path aliases identical in TypeScript, Vite, and Vitest,
`no-restricted-imports` boundary rules with the approved cross-feature pairs
documented, and `test/check-boundaries.mjs` proving each rule still fails on a
fixture. Closes `P1-W11 GAP-5`.

### WCX-02 — generated API contract and error taxonomy

`openapi-typescript` generation committed with a CI freshness gate, one
transport, and an eleven-kind `ConsoleError` taxonomy decoupled from HTTP
status codes. Closes `RE-10` drift and `P1-W11 GAP-3`; the inert `Origin`
header is gone and a test asserts no request sets it.

### WCX-03 — design tokens, severity semantics, theme and motion policy

Two token layers with a lint-enforced boundary, colour meaning confined to
three disjoint groups, neutral device and configuration states, total status
encoding with an unknown fallback, and a 200 ms motion cap. Twenty-one new
tests; 70 tests pass. WCAG 2.2 AA contrast is computed over 36 enumerated
token pairings.

CSS grew 10952 to 19500 bytes minified against a 32 KiB budget. The package
estimated roughly 3 KiB, which was wrong: `var()` references cost more than
the literals they replace, so tokenising has no offset to collect. The figures
and the reasoning are in
[`docs/runbooks/web-console/development.md`](../runbooks/web-console/development.md).

Accepted in two steps, and the reason belongs in the evidence. The 2026-09-05
approval covered [#87](https://github.com/GuardianPot/guardian/pull/87) alone,
and the axe scan in `full.yml` then failed on that merge: the health badge
measured 4.22 against the 4.5 threshold, because `.panelHeading > span`
outranked the tone class and repainted it with the muted neutral. The unit
table had not caught it and could not — it measured tones against the bare
panel token, a background the browser never paints, which also reported
severity high and critical as 4.96 and 5.05 when composited they were 4.30 and
4.34. [#89](https://github.com/GuardianPot/guardian/pull/89) made the tone
selectors compound, derived every tint from its primitive with `color-mix()`,
lightened the three failing primitives, and measured the composited
background. `full.yml` passed on `899bbb8`, the merged head. The Product Owner
approved the corrected state on 2026-09-06, which is the date in the table
above; the superseded 2026-09-05 approval is recorded here rather than
overwritten.

## Gate authority

Only the Product Owner closes this gate. An agent may add evidence rows and
record an owner approval with its date; it may not set the gate decision.

## Current status

**OPEN.** Three of twenty-one packages accepted.

## Known open items

- `edge:integration` fails on `main`. It is unrelated to any Web Console
  package and now runs only in `full.yml` under change proposal `0007`.
- `enforce_admins` is `false` on `main`. Change proposal `0009` records why
  that matters for the agent's merge bound and recommends setting it.
- `BR-15` required `CODEOWNERS` review is specified and deliberately not
  enabled.
