---
id: WCX-08
phase: 2
wave: foundation
title: Text catalogue and canonical timestamp presentation
status: draft
risk: low
components:
  - web-console
decision_refs:
  - WC-D18
  - WC-D22
  - WC-D03
  - SRC-07
  - EV-06
  - TP-03
  - OPS-03
remediates:
  - P1-W11 GAP-6
acceptance_refs:
  - WCX-000 section 3.3 and 3.4
  - SRC-07 provenance-aware wording
depends_on:
  - WCX-04
allowed_paths:
  - "apps/web-console/src/**"
  - "apps/web-console/eslint.config.js"
  - "apps/web-console/index.html"
  - "docs/runbooks/web-console/development.md"
  - "docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/WCX-08.md"
forbidden_paths:
  - "apps/control-plane/**"
  - "apps/edge-agent/**"
  - "openapi/**"
  - "proto/**"
  - "tests/**"
  - "0-planning-documents/**"
requires_owner_review: true
requires_security_review: false
---

# WCX-08 — Text catalogue and canonical timestamp presentation

## 1. Purpose

Move every operator-facing string into one reviewable catalogue, and replace
ad-hoc date formatting with a single forensically correct timestamp
presentation.

## 2. Why now

Two reasons converge. First, this product's differentiator is its wording:
`SRC-07` requires provenance-aware language and `EV-04` requires evidence to
precede inference. Wording that lives inline across dozens of components
cannot be reviewed as a whole. Second, the console currently formats every
timestamp with a local medium date and short time, with no timezone label, no
seconds, and no absolute value — insufficient for an attacker journey whose
correctness is ordering, and inconsistent with `clock_quality` being a health
condition the product already tracks.

## 3. Inputs and decisions

- `WC-D22` — English only, i18n-ready: all operator-facing text in one
  catalogue, no literal strings in components, backend-originated text never
  translated.
- `WC-D18` — canonical time presentation with visible zone, UTC on detail,
  seconds in evidence contexts, relative time only alongside absolute, and
  degraded `clock_quality` marking affected timestamps.
- `SRC-07`, `EV-06`, `TP-03`, `OPS-03`.
- Remediates `P1-W11 GAP-6`.

## 4. Dependencies

`WCX-04` must be accepted; `Timestamp` is a shared component and the catalogue
replaces the catalogue-ready constants that `WCX-02` through `WCX-07`
deliberately left in place.

## 5. Scope

1. Create the text catalogue and its typed key accessor.
2. Move every operator-facing string into it, including the constants left by
   earlier packages.
3. Add a lint rule forbidding literal user-facing text in components.
4. Implement the `Timestamp` component and its formatting rules.
5. Replace `formatTime` and `formatExpiry` with it.
6. Establish and apply the provenance-aware wording rules.

## 6. Non-goals

- No second language, no locale negotiation, no translation files beyond
  English, and no i18n runtime library.
- No right-to-left layout work.
- No new screen, capability, or endpoint.
- No change to what any screen displays other than timestamp precision and the
  wording corrections listed in the evidence.
- No translation of backend reason codes, condition types, status slugs, or
  any device-originated text.

## 7. Allowed paths

Only the paths in frontmatter.

## 8. Security constraints

1. Backend and attacker-originated text is never a catalogue key and never a
   translation input. It is rendered through the untrusted components from
   `WCX-06`, unchanged.
2. Interpolation inserts values as text nodes only. The accessor must not
   accept or produce markup, and a test asserts that a value containing markup
   produces no element.
3. No catalogue entry may contain a credential, a token, a bootstrap value, or
   a recovery code, including as an example. A repository check scans the
   catalogue for secret-shaped strings.
4. Timestamp formatting must never fabricate precision. An absent or invalid
   timestamp renders the unknown treatment, never a default date, never the
   epoch, and never the current time.
5. When the `clock_quality` health condition is not `True` for the source
   device, timestamps attributed to that source are marked as being of
   uncertain quality. Silently displaying them as authoritative would
   overstate evidence.

## 9. Implementation requirements

### 9.1 Catalogue

One module tree under `src/shared/text/`, organised by namespace matching the
feature boundaries: `common`, `auth`, `environments`, `devices`, `health`,
`errors`, `states`, `time`, `a11y`. Each entry is a typed key mapping to an
English string.

Rules:

1. Keys are dot-separated, stable, and describe meaning rather than wording,
   for example `devices.enrollment.secretShownOnce`, not
   `devices.enrollment.enterThisOnTheHost`.
2. The accessor is typed so an unknown key fails typecheck.
3. Interpolation uses named placeholders and accepts only strings, numbers,
   and already-rendered nodes. A missing placeholder value fails typecheck.
4. Pluralisation uses `Intl.PluralRules` with explicit `one` and `other`
   forms; no string concatenation of a count and a noun.
5. `errors.*` entries are keyed by the `messageKey` values that `WCX-02`
   reserved, so the temporary map created there is deleted, not rewritten.
6. No entry is generated at runtime and no entry is composed by concatenating
   two entries.

Lint enforcement: a rule forbids a string literal appearing as JSX text or as
the value of a user-facing attribute such as `aria-label`, `title`,
`placeholder`, or `alt`, outside the catalogue and test files. Technical
strings such as class names, test identifiers, and route paths are exempt by
an explicit allowlist of attributes rather than by judgement.

### 9.2 Wording rules

Recorded in the runbook and applied while moving each string:

1. Provenance-aware phrasing per `SRC-07`. Prefer `Observed source`,
   `Reported by`, `Last observed`, and `Supplied during authentication` over
   assertions of identity.
2. Absence is never phrased as health. `No health projection has been
   reported` is correct; `All good` is not.
3. Inference is labelled. Anything not directly observed is prefixed as
   probable, inferred, or suggested.
4. Configuration completeness and health are never described with the same
   word.
5. Error text states what happened and the next action, never a diagnostic
   detail and never a submitted value.
6. Destructive confirmations name the object and the irreversible effect.

Every wording change made during extraction is listed in the evidence, so this
package cannot silently alter product language.

### 9.3 Timestamp presentation

A single `Timestamp` component in `@shared/ui/`, replacing `formatTime` and
`formatExpiry`.

Props: the ISO value, a precision of `minute` or `second`, a display mode of
`absolute` or `absoluteWithRelative`, and an optional quality flag.

Rendering rules:

1. Rendered as a `<time>` element whose `dateTime` attribute carries the
   original ISO-8601 value.
2. The visible value is the operator's local time with a **visible timezone
   abbreviation**, always. Never a bare local time.
3. Precision `second` is mandatory in evidence, journey, audit, and health
   transition contexts. Precision `minute` is permitted elsewhere.
4. The accessible description and the title carry the full UTC ISO-8601
   value, so the absolute instant is always retrievable without leaving the
   keyboard.
5. Relative time, when shown, appears alongside the absolute value and never
   replaces it. It is recomputed on a one-minute cadence and does not announce
   on each recomputation.
6. An absent, empty, or unparseable value renders the unknown treatment from
   `WCX-04` with an explanatory string, never a fallback date.
7. When the quality flag is set, the timestamp carries a visible marker and an
   accessible description stating that the source device reported degraded
   clock quality. The marker uses a neutral token, not a severity token.
8. Duration and age formatting used by the `stale` state comes from the same
   module, so `WCX-07`'s temporary helper is deleted.

### 9.4 UI/UX requirements

1. Every existing timestamp gains a timezone abbreviation. This is the only
   intended visible change beyond listed wording corrections.
2. Evidence-adjacent surfaces gain seconds. In Phase 2 that means health
   condition transition times and enrollment secret expiry.
3. Layout must absorb the longer timestamp without wrapping in the existing
   fact grid and condition list; if it cannot, the fix is a layout adjustment
   here, not a shortened timestamp.

### 9.5 Accessibility requirements

1. `<time>` carries a machine-readable `dateTime`.
2. The UTC value is available to assistive technology through the accessible
   description, not only through a mouse hover title.
3. Relative-time recomputation does not create a live-region announcement.
4. The clock-quality marker is not conveyed by colour or icon alone; its
   description is part of the accessible name or description.
5. Catalogue entries used as accessible names are reviewed so that no name is
   a bare identifier.

### 9.6 API and data contracts

None changed. The component consumes ISO-8601 strings already present in the
generated types.

### 9.7 Error and failure behaviour

An invalid timestamp renders unknown and is reported by a component test. A
missing catalogue key fails typecheck rather than rendering the key at
runtime. A missing interpolation value fails typecheck.

### 9.8 Internationalisation

The catalogue is the i18n seam. No locale negotiation is implemented and the
document language remains `en`. Number, date, and plural formatting already go
through `Intl`, so adding a locale later changes catalogue files and a locale
resolver, not components.

### 9.9 Theme

The clock-quality marker uses a neutral semantic token from `WCX-03`. No new
token is introduced.

### 9.10 Performance

The catalogue is a plain object tree, bundled with the entry chunk. It must
stay under 12 KiB uncompressed at the end of this package. `Intl` formatters
are constructed once and memoised, never per render.

### 9.11 Observability

None.

### 9.12 Documentation

Add `Text catalogue and wording rules` and `Time presentation` sections to
`docs/runbooks/web-console/development.md`, including the wording rules, the
key naming convention, and the timestamp precision table.

## 10. Required tests

### 10.1 Unit and component

1. No component contains a user-facing string literal; a lint fixture proves
   the rule fails.
2. An unknown catalogue key fails typecheck; a fixture proves it.
3. A missing interpolation value fails typecheck; a fixture proves it.
4. Interpolating a value containing markup produces text, not elements.
5. Pluralisation produces correct `one` and `other` forms for zero, one, and
   many.
6. `Timestamp` renders a `<time>` element with the original ISO value in
   `dateTime`.
7. Every rendered timestamp includes a timezone abbreviation.
8. The UTC ISO value is present in the accessible description.
9. Precision `second` renders seconds; `minute` does not.
10. An absent, empty, and unparseable value each render the unknown treatment
    and never a date.
11. The quality flag renders the marker and its description, using a neutral
    token rather than a severity token.
12. Relative time appears only alongside absolute time and produces no
    announcement on recomputation.
13. A repository check finds no secret-shaped string in the catalogue.

### 10.2 Browser and E2E scenarios

The existing onboarding suite passes unchanged. Selector text that changed
because of a listed wording correction is updated in this package's evidence
and the corresponding browser assertion is updated by `WCX-06`'s owner of
`tests/**` in a coordinated follow-up; if a browser assertion must change
here, stop and escalate because `tests/**` is forbidden.

## 11. Acceptance criteria and Definition of Done

1. Every operator-facing string lives in the catalogue and the lint rule
   enforces it.
2. All temporary constants left by `WCX-02` through `WCX-07` are gone.
3. `Timestamp` replaces `formatTime` and `formatExpiry`, and no other date
   formatting exists in the console.
4. Every timestamp shows a timezone; evidence contexts show seconds; the UTC
   value is always reachable.
5. Degraded clock quality is visibly marked and described.
6. Every wording change is listed in the evidence.
7. `task web:check` and `task web:e2e` pass; the catalogue stays under 12 KiB.

## 12. Evidence required

- A table of every extracted string: old inline text, new key, and whether the
  wording changed and why.
- Lint output for the literal-text fixture and the typecheck fixtures.
- Screenshots showing timestamps with timezone, and one showing the degraded
  clock-quality marker.
- Catalogue size measurement.
- Secret-shaped-string scan output.

## 13. Stop and escalate

Stop and request owner review if any of the following occurs:

- extracting a string requires changing its meaning rather than its location;
- a wording rule conflicts with an approved acceptance criterion's language;
- a browser assertion in `tests/**` must change, since that path is forbidden
  here;
- the timezone-bearing timestamp cannot fit an existing layout without a
  design change;
- degraded clock quality cannot be associated with a timestamp because the
  health projection does not expose the needed linkage.

## 14. Deliverables

The typed text catalogue with namespaces and the literal-text lint rule, the
recorded wording rules, the `Timestamp` component with full precision, UTC
availability, unknown handling, and clock-quality marking, removal of all
duplicated date helpers and temporary constants, the test suite above, and the
two runbook sections.
