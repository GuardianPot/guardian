# Web Console extended multi-phase work packages

This directory holds the Web Console implementation packages that follow the
Phase 1 console shell delivered by `P1-W11`. Their governing decisions are
closed in [`00-master-decision-record.md`](00-master-decision-record.md).

The directory name records the Phase 1 to Phase 2 bridge that produced this
set. Each package carries its own approved `phase` in frontmatter; the
directory name is not a phase assignment.

## Status

The Product Owner promoted `WCX-01` through `WCX-21` to
`approved-for-implementation` on 2026-09-04. Specification approval is not the
same as `READY`: an issue becomes `READY` only when its dependencies are
closed, per `PM-16`.

`WCX-21` was promoted on the same date once change proposal
[`0006`](../../change-proposals/0006-retention-configuration-ownership.md)
gave it a narrowly enumerated Control Plane surface for the retention domain
it consumes.

Change proposals [`0003`](../../change-proposals/0003-web-console-session-csrf-reissue.md)
and [`0004`](../../change-proposals/0004-web-console-error-contract.md) were
approved on 2026-09-04, so `WCX-09` level-3 actions and `WCX-11` field-level
validation are unblocked.

## Package index

| Package | Issue | Phase | Wave | Title | Risk |
|---|---|---|---|---|---|
| [WCX-01](WCX-01.md) | [#63](https://github.com/GuardianPot/guardian/issues/63) | 2 | foundation | Module boundaries, capability seam, lint and typecheck hardening | medium |
| [WCX-02](WCX-02.md) | [#64](https://github.com/GuardianPot/guardian/issues/64) | 2 | foundation | Generated OpenAPI types, API transport, and error taxonomy | high |
| [WCX-03](WCX-03.md) | [#65](https://github.com/GuardianPot/guardian/issues/65) | 2 | foundation | Design tokens, severity semantics, theme and motion policy | medium |
| [WCX-04](WCX-04.md) | [#66](https://github.com/GuardianPot/guardian/issues/66) | 2 | foundation | Shared component layer, data-state matrix, error boundary | high |
| [WCX-05](WCX-05.md) | [#67](https://github.com/GuardianPot/guardian/issues/67) | 2 | foundation | Accessibility baseline and enforcement | medium |
| [WCX-06](WCX-06.md) | [#68](https://github.com/GuardianPot/guardian/issues/68) | 2 | foundation | Test tooling, hostile-content contract, component workbench | high |
| [WCX-07](WCX-07.md) | [#69](https://github.com/GuardianPot/guardian/issues/69) | 2 | foundation | Freshness policy, code splitting, performance budgets | medium |
| [WCX-08](WCX-08.md) | [#70](https://github.com/GuardianPot/guardian/issues/70) | 2 | foundation | Text catalogue and canonical timestamp presentation | low |
| [WCX-09](WCX-09.md) | [#71](https://github.com/GuardianPot/guardian/issues/71) | 2 | capability | Operator completeness and sensitive-action reauthentication | high |
| [WCX-10](WCX-10.md) | [#72](https://github.com/GuardianPot/guardian/issues/72) | 2 | foundation | Navigation information architecture and responsive correction | medium |
| [WCX-11](WCX-11.md) | [#73](https://github.com/GuardianPot/guardian/issues/73) | 2 | capability | Form and validation stack with decoy management UI | high |
| [WCX-12](WCX-12.md) | [#74](https://github.com/GuardianPot/guardian/issues/74) | 3 | capability | Incident-first dashboard, tables, pagination, virtualisation | high |
| [WCX-13](WCX-13.md) | [#75](https://github.com/GuardianPot/guardian/issues/75) | 3 | capability | Incident detail, attacker journey, evidence explorer | high |
| [WCX-14](WCX-14.md) | [#76](https://github.com/GuardianPot/guardian/issues/76) | 4 | capability | AI explanation, notification centre, disposition | high |
| [WCX-15](WCX-15.md) | [#77](https://github.com/GuardianPot/guardian/issues/77) | 5 | capability | Security hardening, theme completion, regression suites | high |
| [WCX-16](WCX-16.md) | [#78](https://github.com/GuardianPot/guardian/issues/78) | 2 | capability | Guided onboarding, placement validation, coverage verification | high |
| [WCX-17](WCX-17.md) | [#79](https://github.com/GuardianPot/guardian/issues/79) | 3 | capability | Expected-source management and incident merge and split | high |
| [WCX-18](WCX-18.md) | [#80](https://github.com/GuardianPot/guardian/issues/80) | 4 | capability | Notification channel configuration and escalation contacts | high |
| [WCX-19](WCX-19.md) | [#81](https://github.com/GuardianPot/guardian/issues/81) | 5 | capability | Update and rollback, operational health, diagnostics bundle | high |
| [WCX-20](WCX-20.md) | [#82](https://github.com/GuardianPot/guardian/issues/82) | 2 | capability | Synthetic honey credential workflow | high |
| [WCX-21](WCX-21.md) | [#83](https://github.com/GuardianPot/guardian/issues/83) | 2 | capability | Retention configuration and purge visibility | high |

`WCX-16` through `WCX-21` were added on 2026-09-04 by the coverage audit
recorded in `00-master-decision-record.md` section 3.8. They cover approved
operator surfaces that the first decomposition left without a console home.
`WCX-15` runs last within Phase 5, after `WCX-19`.

## Dependency map

```text
P1-W11 (delivered console shell)
  |
  +--> WCX-01 boundaries + capability seam + lint/typecheck
  |      |
  |      +--> WCX-02 generated types + transport + error taxonomy
  |      |      |
  |      |      +--> WCX-07 freshness + splitting + budgets
  |      |      +--> WCX-09 operator completeness        [needs CP-0003]
  |      |      +--> WCX-11 forms + decoy UI             [needs CP-0004]
  |      |
  |      +--> WCX-03 tokens + severity + theme + motion
  |             |
  |             +--> WCX-04 shared components + data-state matrix + boundary
  |                    |
  |                    +--> WCX-05 accessibility baseline
  |                    +--> WCX-08 text catalogue + timestamps
  |                    +--> WCX-10 navigation IA + responsive
  |
  +--> WCX-06 test tooling + hostile contract + workbench
         (required before WCX-09 and later capability work)

WCX-01..WCX-08, WCX-10  ──> WCX-09, WCX-11        (Phase 2 capability)
WCX-11                  ──> WCX-16, WCX-20, WCX-21  (Phase 2 capability)
WCX-04, WCX-07, WCX-10  ──> WCX-12 ──> WCX-13     (Phase 3)
WCX-13                  ──> WCX-17                (Phase 3)
WCX-12, WCX-13          ──> WCX-14 ──> WCX-18     (Phase 4)
WCX-16, WCX-18          ──> WCX-19                (Phase 5)
WCX-19 and all others   ──> WCX-15                (Phase 5, last)
```

### Hard dependencies

| Package | Depends on | Reason |
|---|---|---|
| WCX-01 | P1-W11 | Restructures the delivered console source tree |
| WCX-02 | WCX-01 | Generated types and transport live in the new boundary layout |
| WCX-03 | WCX-01 | Token files live in `src/shared/theme` |
| WCX-04 | WCX-02, WCX-03 | Components consume the error taxonomy and semantic tokens |
| WCX-05 | WCX-04 | Enforcement targets the shared component layer |
| WCX-06 | WCX-01 | Handlers are typed against the boundary layout; hostile contract lands in `src/shared` |
| WCX-07 | WCX-02 | Freshness policy wraps the query layer |
| WCX-08 | WCX-04 | `Timestamp` is a shared component; catalogue keys replace component literals |
| WCX-09 | WCX-04, WCX-06, WCX-08, CP-0003 | Uses confirmation levels, hostile rendering, catalogue text, and CSRF re-issue |
| WCX-10 | WCX-04 | Shell and navigation are shared components |
| WCX-11 | WCX-02, WCX-04, WCX-08, P2-W15 | Decoy management UI needs the Phase 2 decoy backend |
| WCX-12 | WCX-07, WCX-10, P3-W13 | Dashboard needs incident backend and the incident-first root |
| WCX-13 | WCX-12, P3-W14, P3-W15 | Detail, journey, and evidence explorer need incident detail backend |
| WCX-14 | WCX-13, P4-W9, P4-W10 | Explanation and notification UI need AI and notification backends |
| WCX-16 | WCX-11, P2-W14 | Coverage state is backend truth from the functional health package |
| WCX-21 | WCX-08, WCX-11, CP-0006 | Owns its own retention backend under CP-0006 narrow paths |
| WCX-20 | WCX-11, P2-W9 | The credential workflow attaches to decoys and needs the synthetic credential domain |
| WCX-17 | WCX-13, P3-W8, P3-W11 | Corrections and suppression act on incident detail |
| WCX-18 | WCX-14, P4-W11, P4-W12 | Channel configuration needs the email and webhook backends |
| WCX-19 | WCX-16, WCX-18, P5-W3, P5-W4, P5-W8, P5-W11 | The health surface aggregates coverage and channel health; update and diagnostics need their Phase 5 backends |
| WCX-15 | all preceding, including WCX-19 | Hardening and regression suites close the set |

### Roadmap ownership

No package pulls a roadmap capability earlier than its approved phase.
`WCX-11` cannot start before `P2-W15` provides the decoy backend, `WCX-16`
cannot start before `P2-W14` provides functional coverage, `WCX-12`, `WCX-13`,
and `WCX-17` cannot start before Phase 3 provides incidents and corrections,
`WCX-14` and `WCX-18` cannot start before Phase 4 provides AI and the
notification channels, and `WCX-19` cannot start before Phase 5 provides the
updater, queue policy, and diagnostics bundle. Foundation-wave packages add no
capability and may run as soon as their structural dependency closes.

## Execution waves

1. `WCX-01`. Its dependencies are the only ones already closed, so it is the
   only package that can reach `READY` today.
2. `WCX-02`, `WCX-03`, and `WCX-06` once `WCX-01` is accepted.
3. `WCX-04`.
4. `WCX-05`, `WCX-07`, `WCX-08`, `WCX-10`.
5. `WCX-09` once change proposal `0003` is approved.
6. `WCX-11` once `P2-W15` is accepted and change proposal `0004` is approved.
7. `WCX-16` once `P2-W14` is accepted; `WCX-20` once `P2-W9` is accepted;
   `WCX-21` once a backend owner for retention configuration exists.
8. `WCX-12` then `WCX-13` then `WCX-17` inside Phase 3.
9. `WCX-14` then `WCX-18` inside Phase 4.
10. `WCX-19` inside Phase 5.
11. `WCX-15` last, closing the set.

## Lifecycle

1. The Product Owner promotes a package specification from `draft`.
2. Its GitHub issue is created and added to `Guardian Delivery`.
3. The issue becomes `READY` only after every hard dependency is accepted.
4. One isolated branch or worktree executes the package.
5. Merge moves the item to `AC-VALIDATION`; evidence acceptance moves it to
   `DONE`.
6. Only the Product Owner closes a phase gate.

## Standing constraints

Every package in this set inherits the constraints recorded in
[`00-master-decision-record.md`](00-master-decision-record.md) section 9. A
package that needs a decision not recorded there stops and escalates rather
than choosing silently.
