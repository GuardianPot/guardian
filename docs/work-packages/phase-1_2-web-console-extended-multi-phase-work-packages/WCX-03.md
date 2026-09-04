---
id: WCX-03
phase: 2
wave: foundation
title: Design tokens, severity semantics, theme and motion policy
status: draft
risk: medium
components:
  - web-console
decision_refs:
  - WC-D10
  - WC-D11
  - WC-D12
  - WC-D13
  - WC-D09
  - CS-01
  - CS-02
  - SRC-07
  - EV-04
  - W11-C2-A
acceptance_refs:
  - WCX-000 section 3.2
  - CS-01 confidence levels
  - CS-02 severity levels separate from confidence
depends_on:
  - WCX-01
allowed_paths:
  - "apps/web-console/src/shared/theme/**"
  - "apps/web-console/src/styles/**"
  - "apps/web-console/src/**/*.module.css"
  - "apps/web-console/index.html"
  - "apps/web-console/eslint.config.js"
  - "apps/web-console/package.json"
  - "package-lock.json"
  - "docs/runbooks/web-console/development.md"
  - "docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/WCX-03.md"
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

# WCX-03 — Design tokens, severity semantics, theme and motion policy

## 1. Purpose

Establish the visual system that carries the product's truth semantics:
a two-layer token architecture, disjoint palettes for brand, severity, and
health, a theme model, and a motion budget that prevents security signals from
becoming decorative.

## 2. Why now

Today the brand accent and the healthy state are the same green, and the
warning colour serves both a neutral configuration state and an unknown health
state. Phase 3 introduces five severity levels and three confidence levels on
top of that collision. Fixing the palette after the incident dashboard exists
means re-deciding the meaning of every colour already shipped.

## 3. Inputs and decisions

- `WC-D10` — two-layer primitive and semantic tokens owned by one file;
  components may reference semantic tokens only.
- `WC-D11` — brand, severity, and health palettes are disjoint; confidence is
  never colour-coded; every severity or status indicator carries icon, text,
  and colour.
- `WC-D12` — dark-only through Phase 4; system-preference light support is
  Phase 5 work in `WCX-15`; no manual toggle and no browser storage.
- `WC-D13` — bounded motion budget; no security signal is encoded by motion.
- `CS-01`, `CS-02` — confidence `Low`, `Medium`, `High`; severity
  `Informational`, `Low`, `Medium`, `High`, `Critical`, separate from
  confidence.
- `W11-C2-A` — CSS Modules and CSS variables remain the styling model.

## 4. Dependencies

`WCX-01` must be accepted; token files live at `@shared/theme`.

## 5. Scope

1. Define primitive and semantic token layers in `src/shared/theme/`.
2. Replace every hard-coded colour, spacing, radius, and font-size literal in
   `app.module.css` with a semantic token.
3. Separate the brand palette from the health palette, which changes the brand
   accent hue.
4. Define the severity and confidence encoding contract, including the
   non-colour channel.
5. Define the motion budget and its reduced-motion behaviour.
6. Scaffold the theme structure so `WCX-15` can add light values without
   restructuring.
7. Add a lint rule forbidding raw colour literals and primitive-token use
   outside the theme directory.

## 6. Non-goals

- No new component, route, screen, or capability.
- No light theme values. `WCX-15` adds them.
- No theme toggle and no persistence of any kind.
- No layout change. Spacing tokens replace literals at their current values;
  visual spacing is preserved.
- No incident, severity, or confidence UI. This package defines the contract
  and the tokens; `WCX-12` and `WCX-13` consume them.
- No icon set adoption beyond the minimal status glyphs in 9.2.

## 7. Allowed paths

Only the paths in frontmatter. Component TypeScript files are not editable
here except where a CSS Module class rename requires it; if a component's
markup or logic must change, that work belongs to `WCX-04`.

## 8. Security constraints

1. Colour may never be the only channel carrying a severity, confidence,
   health, or device-state meaning. Every such indicator renders an icon and a
   text label alongside the colour. A test asserts the text label exists for
   each value.
2. No token, class name, or style may be derived from backend or
   attacker-supplied content. Dynamic class lookup must be a total mapping
   from a closed union to a known class, with an explicit fallback for an
   unrecognised value that renders as unknown, never as healthy.
3. The unknown health state must be visually distinct from both healthy and
   failing. It must not read as a softened healthy state.
4. No inline `style` attribute may carry a colour, because the approved CSP
   forbids `unsafe-inline` and inline styles are a rendering-injection
   surface.

## 9. Implementation requirements

### 9.1 Token architecture

Two files under `src/shared/theme/`:

- `primitives.css` — raw scales only. Names carry no meaning:
  `--color-<hue>-<step>`, `--space-<n>`, `--radius-<size>`, `--size-<n>`,
  `--font-size-<n>`, `--line-height-<n>`, `--weight-<n>`, `--elevation-<n>`,
  `--duration-<n>`, `--easing-<name>`.
- `semantic.css` — meaning only, defined in terms of primitives.

Semantic groups, complete:

| Group | Tokens |
|---|---|
| Surface | `--surface-page`, `--surface-raised`, `--surface-sunken`, `--surface-overlay` |
| Line | `--line-subtle`, `--line-strong`, `--line-focus` |
| Text | `--text-primary`, `--text-muted`, `--text-inverse`, `--text-link` |
| Brand | `--brand-accent`, `--brand-accent-contrast`, `--brand-mark` |
| Health | `--health-true`, `--health-false`, `--health-unknown`, plus a `-surface` variant of each |
| Severity | `--severity-informational`, `--severity-low`, `--severity-medium`, `--severity-high`, `--severity-critical`, plus a `-surface` variant of each |
| Config state | `--config-complete`, `--config-pending` |
| Device state | `--device-pending`, `--device-active`, `--device-disabled`, `--device-revoked` |
| Interaction | `--action-primary`, `--action-primary-contrast`, `--action-secondary`, `--action-disabled-opacity`, `--focus-ring` |
| Spacing and shape | `--space-inset-*`, `--space-stack-*`, `--radius-control`, `--radius-panel`, `--radius-pill` |
| Motion | `--motion-fast`, `--motion-standard`, `--motion-overlay`, `--motion-easing` |

Rules:

1. `semantic.css` is the only file that may reference a primitive token.
2. Component CSS Modules may reference semantic tokens only.
3. No file outside `src/shared/theme/` may contain a colour literal in any
   notation, a raw `px`, `rem`, or `em` length outside a token definition, or
   a raw `ms` duration.
4. Enforced by a stylelint-class rule or an equivalent repository check that
   fails the build on violation.

Structure `semantic.css` so that theme values live in a single
`:root` block whose entire body is token assignments, allowing `WCX-15` to add
a `@media (prefers-color-scheme: light)` block without touching any other
file.

### 9.2 Severity, confidence, health, and state encoding

Palette disjointness is mandatory:

1. The brand accent must not share a hue family with `--health-true` or with
   any severity token. Because today's brand accent is the same green as the
   healthy state, the brand accent hue changes in this package. This is an
   intentional, visible change and is the only visible change the package
   permits.
2. `--config-complete` and `--config-pending` describe configuration
   completeness, not health, and must not reuse health tokens.
3. Severity uses a single hue ramp from `informational` to `critical` so that
   ordering is perceivable. Health uses three unrelated, categorical values so
   that `Unknown` cannot read as a midpoint between healthy and failing.

Non-colour channel, mandatory for every indicator:

| Domain | Values | Required glyph shape | Required text |
|---|---|---|---|
| Health | `True`, `False`, `Unknown` | filled circle, filled square, hollow diamond | `Healthy`, `Action required`, `Unknown` |
| Severity | `Informational` to `Critical` | five distinct shapes, ordered | the severity name |
| Device state | `pending`, `active`, `disabled`, `revoked` | four distinct shapes | the state name |
| Configuration | complete, pending | two distinct shapes | `Configured`, `Needs zones` |

Confidence is **never** colour-coded and never shares the severity ramp. It is
rendered as a labelled three-step indicator, `Low`, `Medium`, `High`, using a
neutral text token and a discrete step glyph. A test asserts no confidence
value resolves to a severity or health token.

Glyphs are inline SVG shipped from `src/shared/theme/glyphs/`. They carry
`aria-hidden="true"` because the adjacent text is the accessible name. No icon
font and no external icon package is added.

### 9.3 Theme model

1. Dark is the only defined theme through Phase 4.
2. `index.html` keeps `<meta name="color-scheme" content="dark">` until
   `WCX-15` changes it.
3. No theme toggle, no `localStorage`, `sessionStorage`, or cookie use.
4. `semantic.css` is structured for a later `prefers-color-scheme` block; no
   light values are written now.

### 9.4 Motion budget

Permitted motion, exhaustive:

| Purpose | Maximum duration | Token |
|---|---|---|
| Control state feedback such as hover, press, focus | 150 ms | `--motion-fast` |
| Overlay and dialog enter and exit | 200 ms | `--motion-overlay` |
| Layout or disclosure transition | 200 ms | `--motion-standard` |
| Skeleton or pending shimmer on first load only | continuous, single element | `--motion-standard` |

Forbidden without a change proposal: any motion that encodes severity,
confidence, health, or device state; auto-playing timelines; looping
attention-seeking animation on data; parallax; motion longer than 200 ms.

Under `prefers-reduced-motion: reduce` all transition and animation durations
collapse to effectively zero, preserving the current global rule, and shimmer
is replaced by a static pending treatment.

### 9.5 UI/UX requirements

Beyond the brand accent hue change, the rendered layout, spacing, type scale,
and component appearance must be preserved. Every replaced literal keeps its
current computed value unless a value is required to change to satisfy the
contrast requirement in 9.6, in which case the change is listed in the
evidence.

### 9.6 Accessibility requirements

1. Target `WCAG 2.2 Level AA`. Text and its background meet 4.5 to 1;
   large text and non-text indicators meet 3 to 1.
2. Every semantic colour pair that can occur is contrast-checked by an
   automated test over the token file, not by manual inspection. The test
   enumerates the defined foreground and background pairings and fails on any
   pair below its threshold.
3. The focus ring token must meet 3 to 1 against both the surface it sits on
   and the control it outlines.
4. Every status glyph is `aria-hidden` and accompanied by text.

### 9.7 API and data contracts

None. This package makes no request and reads no contract field. The closed
unions it maps over are the ones already generated in `WCX-02`:
`HealthStatus`, the device state enum, and the environment status enum.
Severity and confidence unions are declared here as local types and are
reconciled with the contract in `WCX-12` when the incident schema exists.

### 9.8 Error and failure behaviour

An unrecognised value in any mapped union resolves to the unknown treatment
and must never resolve to a healthy, low-severity, or complete treatment. A
test asserts this for each mapping using a deliberately invalid value.

### 9.9 Internationalisation

All new text labels are introduced as catalogue-ready constants so `WCX-08`
can move them without renaming. No literal user-facing string is embedded in
CSS content properties.

### 9.10 Performance

The CSS budget of 32 KiB must hold. Token definitions add roughly 3 KiB;
removing duplicated literals is expected to offset most of it. Report the CSS
size before and after. No JavaScript is added.

### 9.11 Observability

None.

### 9.12 Documentation

Add a `Visual system` section to `docs/runbooks/web-console/development.md`
covering the token layers, the disjoint-palette rule, the non-colour channel
requirement, the confidence rule, and the motion budget.

## 10. Required tests

### 10.1 Unit and component

1. Contrast test over the token pairing table; every pair meets its threshold.
2. Disjointness test: no severity token resolves to the same computed value as
   a health token or the brand accent.
3. Confidence test: no confidence value maps to a severity or health token.
4. Encoding test: each health, severity, device-state, and configuration value
   renders a glyph and a text label.
5. Fallback test: an invalid union value renders the unknown treatment.
6. Reduced-motion test: with `prefers-reduced-motion: reduce`, no element
   reports a non-trivial transition or animation duration.

### 10.2 Static analysis

A lint or repository check fails when a colour literal, raw length, or raw
duration appears outside `src/shared/theme/`, and when a component CSS Module
references a primitive token.

### 10.3 Browser and E2E scenarios

The existing onboarding suite passes in all three browsers, including its axe
scan and its reduced-motion step. The brand accent change must not break any
existing selector; if it does, the selector was colour-dependent and the test,
not the palette, is corrected in `WCX-06`.

## 11. Acceptance criteria and Definition of Done

1. Primitive and semantic token layers exist, and no colour, length, or
   duration literal remains outside the theme directory.
2. Brand, health, and severity palettes are disjoint and proven so by test.
3. Every status indicator carries a glyph and text; confidence is not
   colour-coded.
4. Contrast is automatically verified for every defined pairing.
5. The motion budget is documented and enforced; reduced motion removes all
   animation.
6. `task web:check` and `task web:e2e` pass; CSS stays within 32 KiB.
7. Apart from the brand accent hue, no visual regression is introduced.

## 12. Evidence required

- Contrast test report listing every pairing and its ratio.
- Before and after screenshots of the environment and device routes, with any
  enrollment secret dismissed first.
- CSS bundle size before and after.
- The list of any computed value that changed to satisfy contrast, with its
  justification.
- Lint output showing a colour-literal violation failing.

## 13. Stop and escalate

Stop and request owner review if any of the following occurs:

- a required contrast ratio cannot be met without changing the product's
  visual identity beyond the brand accent hue;
- five severity steps cannot be made perceivably ordered while also remaining
  disjoint from the health palette;
- an approved decision appears to require colour as the only channel;
- the CSS budget cannot be met;
- a token change would alter layout rather than only colour.

## 14. Deliverables

Primitive and semantic token files, the disjoint palette, the status glyph
set, the severity, confidence, health, and device-state encoding contract, the
motion budget with reduced-motion behaviour, automated contrast and
disjointness tests, the literal-forbidding lint rule, and the visual system
runbook section.
