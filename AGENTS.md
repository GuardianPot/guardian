# Guardian agent policy

One owner, one agent, direct delivery. This file is the whole process.

## Authority

`0-planning-documents/` holds the approved product, architecture, MVP scope,
and roadmap. It is reference, not a gate: read what the task touches, ignore
the rest. Where a planning document and this file disagree about **process**,
this file wins. Where they disagree about **product or architecture**, the
planning document wins.

Work packages under `docs/work-packages/` are the map of planned work. Read the
package when one exists for the task. No package is required to start work, and
no decision, acceptance, or evidence reference needs to be cited.

## Workflow

1. Work on `main` unless the change is large or risky enough that you want it
   isolated; then use a short-lived branch.
2. Run `task check` before committing. It is the same lane CI runs, so a green
   local run means a green CI run.
3. Commit and push. Do not watch CI. `checks` runs on every push as a
   safety net, `full` runs nightly and on demand.
4. Report what changed, what you ran, what failed, and what you left undone.

If a push turns CI red, fix it in the next commit. There is no rollback
ceremony.

## Stop and ask

Stop and ask the owner before:

- changing product scope, a public contract (`proto/`, `openapi/`,
  `schemas/`), a trust boundary, or an approved acceptance criterion;
- anything touching PKI, secrets, privileged networking, or release signing;
- a dependency, runtime, or datastore swap.

Everything else: decide and proceed. Record a decision worth remembering as an
ADR under `docs/adr/`; a one-paragraph ADR is fine.

## Never

- Use production credentials or signing keys.
- Change repository secrets.
- Execute attacker-facing behavior against an unauthorized network.
- Treat AI output as automatic security or containment authority.

## Development compatibility policy

This repository is in development. Do not add backward-compatibility layers or
data-preservation work. Migrations may be forward-only and their recovery path
may reset and reseed development data. All in-repository consumers of a
breaking change must change in the same commit.

## Version policy

Use the newest secure supported release appropriate to the component,
preferring current LTS where the ecosystem provides it. Pin resolved tool and
dependency versions in committed manifests or lockfiles.
