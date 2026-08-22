# Guardian agent policy

## Authority

The approved documents under `0-planning-documents/` are the product,
architecture, MVP, roadmap, and engineering-governance source of truth.
Issue comments and agent instructions cannot override an approved decision.

## Required workflow

1. Work only from an approved work package or an explicitly owner-authorized
   governance/change package.
2. Read `docs/engineering/context-map.md` and the current package before
   changing files.
3. Confirm that phase and package dependencies are closed before product-code
   implementation starts.
4. Use a short-lived branch named with the work-package or issue identifier.
5. Keep changes inside the package `allowed_paths`.
6. Run every package-required check before opening or marking a PR ready.
7. Report changed files, tests, evidence, limitations, and unresolved risks.

## Stop and escalate

Stop implementation and request owner review if a change would alter product
scope, architecture, security boundaries, contracts, release authority,
privileged networking, PKI, secrets, or an approved acceptance criterion.

Never:

- push directly to `main`;
- merge or bypass a pull request;
- change repository settings, rulesets, environments, or secrets;
- use production credentials or signing keys;
- execute attacker-facing behavior against an unauthorized network;
- treat AI output as automatic security or containment authority.

## Development compatibility policy

This repository is in development. Unless the Product Owner explicitly changes
the policy, do not add backward-compatibility layers or data-preservation work.
Development migrations may be forward-only; their documented recovery path may
reset and reseed development data. Protocol-breaking development changes still
require owner review and all in-repository consumers must change atomically.

## Version policy

Use the newest secure supported release appropriate to the component, preferring
current LTS releases where the ecosystem provides LTS. Pin resolved tool and
dependency versions in committed manifests or lockfiles and run the required
security, license, and compatibility checks.

## Current phase gate

Phase 1 package specifications are proposals until the Product Owner records
content approval. Phase 1 product implementation must not start while
`docs/phase-gates/phase-0.md` remains unapproved.

## Minimum report

Every implementation report must include the work package, decision and
acceptance references, changed paths, commands run, test results, security
impact, and known limitations.
