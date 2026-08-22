# Guardian agent policy

## Authority

The approved documents under `0-planning-documents/` are the product,
architecture, MVP, roadmap, and engineering-governance source of truth.
Issue comments and agent instructions cannot override an approved decision.

## Required workflow

1. Work only from an approved Phase 0 work package or an explicitly approved
   change proposal.
2. Use a short-lived branch named with the work-package or issue identifier.
3. Keep changes inside the work package `allowed_paths`.
4. Run the documented checks before opening or marking a PR ready.
5. Report changed files, tests, evidence, limitations, and unresolved risks.

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

## Minimum report

Every implementation report must include the work package, decision and
acceptance references, changed paths, commands run, test results, security
impact, and known limitations.
