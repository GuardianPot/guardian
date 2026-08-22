# Phase 1 work packages

Phase 1 builds the connected platform skeleton defined by the approved roadmap.
P1-W1 through P1-W11 remain `proposed` until Product Owner content approval.
Even after package approval, product implementation is blocked until the Phase
0 gate is approved.

## Approved execution strategy

The Product Owner approved a foundation-first, limited-parallelism strategy on
2026-08-22.

```text
P1-G0 Phase 1 execution bootstrap
  |
  +--> P1-W1 Control Plane shell
  |      +--> P1-W10 Audit baseline
  |      |      +--> P1-W2 Local authentication
  |      |      +--> P1-W3 Environment domain
  |      |      +--> P1-W4 Edge enrollment
  |      |
  |      +--> P1-W9 Health domain (with Edge/helper contracts)
  |
  +--> P1-W7 Edge daemon shell
         +--> P1-W8 Privileged helper
         +--> P1-W4 Edge enrollment

P1-W4 + P1-W7 --> P1-W5 Device channel
P1-W3 + P1-W5 + P1-W7 --> P1-W6 Reconciler
P1-W2 + P1-W3 + P1-W4 --> P1-W11 Web Console shell
P1-W5 + P1-W6 + P1-W9 + P1-W11 --> Phase 1 browser E2E and gate
```

P1-W9 and P1-W11 may use a documented sequential PR chain so contract/UI shell
work can start in parallel, but neither package is accepted until real backend
integration and its final acceptance evidence pass.

## Execution waves

1. P1-W1 and P1-W7.
2. P1-W10 and P1-W8.
3. P1-W2, P1-W3, and P1-W4.
4. P1-W5, P1-W9 integration, and the P1-W11 shell.
5. P1-W6.
6. P1-W9/P1-W11 completion, browser E2E, and gate evidence.

## Approved implementation defaults

- PostgreSQL access: `pgx/v5` + `sqlc`; SQL migrations: `goose`.
- Development migrations are forward-only. Recovery is reset/reseed, with no
  backward-compatibility or development-data preservation layer.
- Auth defaults: CLI-created 15-minute bootstrap token, PostgreSQL opaque
  sessions, 15-minute idle and 8-hour absolute expiry, TOTP, single-use
  recovery codes, and synchronizer CSRF protection.
- Phase 1 rejects overlapping CIDRs inside one environment.
- Enrollment tokens expire after 15 minutes. Device certificates are valid for
  30 days and rotate with jitter in the final 10 days.
- Device transport uses a dedicated 443-compatible endpoint, 30-second
  heartbeat, 90-second stale threshold, full-jitter 1-to-60-second reconnect,
  and a 1 MiB control-message limit.
- Privileged RPC uses Protobuf/gRPC over Unix domain sockets with peer
  credential checks.
- Web UI uses Radix Primitives with CSS Modules/variables, Vitest, React Testing
  Library, and Playwright.
- Exact dependency versions are pinned to the newest secure supported release
  at implementation time, preferring LTS where an ecosystem provides it.

## Lifecycle

1. The Markdown specification is reviewed and approved.
2. Its GitHub issue is created and added to `Guardian Delivery`.
3. The issue becomes `READY` only after every hard dependency is accepted.
4. One isolated branch/worktree executes the package.
5. Merge moves the item to `AC-VALIDATION`; evidence acceptance moves it to
   `DONE`.
6. Only the Product Owner closes the phase gate.
