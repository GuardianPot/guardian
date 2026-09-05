# Change proposal 0007: pull-request CI scope

- Status: APPROVED
- Owner decision: `@sinanganiz` approved on 2026-09-04.
- Affected decision IDs: `CI-09`, `CI-10`, `CI-11`, `CI-15`, `CI-17`, `CI-18`,
  `CI-26`
- Affected acceptance criteria: none. Every gate still runs; this changes when.
- Work package: `WCX-000`

## Problem and context

The `quality` workflow ran sixteen steps sequentially in one job on every
pull request: repository policy, formatting, contracts, Go checks, two
PostgreSQL integration suites, the full web check, a three-engine Playwright
install and run, a container smoke build, the Cowrie fixture, buf breaking
checks, and the secret scan.

Two consequences. First, wall time: a green run took roughly twenty-five
minutes, and the Playwright install of three engines plus `go install`
compiling `task` and `buf` from source accounted for much of it. Second, no
selectivity: a pull request touching only Markdown still built containers and
launched three browsers.

The Product Owner directed on 2026-09-04 that this be visibly loosened because
it was slowing development out of proportion to the risk it caught.

## Options

1. **Keep one sequential job, tune the slow steps.** Cache the Go tools and the
   browser engines, drop nothing. Preserves the gate exactly. Saves perhaps a
   third of the time and leaves a docs change still building containers.
2. **Split into parallel jobs selected by changed paths, and move the widest
   coverage off the pull-request path.** The fast gate always runs; Go, web,
   browser, contract, and container jobs run only when their paths change; the
   three-engine browser matrix and everything else runs in full on every push
   to `main`, on a nightly schedule, and on manual dispatch.
3. **Make heavy suites manual-only.** Fastest pull requests, but a regression
   could reach `main` with nothing having run.

## Recommendation

**Option 3, simplified further than originally proposed.** Path-based job
selection was tried first and produced three configuration defects in a row: a
missing `buf`, a duplicated web gate, and a screenshot count that counted
argument-vector entries instead of engines. Each cost a full round trip. The
selection machinery was more error-prone than the coverage it bought, so it is
gone.

Two workflows, one job each, no conditions and no scripts:

1. `pr.yml` runs on every pull request: repository policy, Markdown format,
   contract layout, generated-artifact freshness, dependency policy, workflow
   SHA pins, the secret scan, the full Web Console gate, and Go vet, unit
   tests, and formatting. No Docker, no browser engines, no generation
   tooling.
2. `full.yml` runs on every push to `main`, nightly at 03:00 UTC, and on
   manual dispatch. It is the complete previous gate: Go checks with the
   integration and race suites, both PostgreSQL integrations, contract
   tooling, the container smoke build, the Cowrie fixture, buf breaking
   checks, and the three-engine browser flow.

Nothing was deleted. Every check that ran before still runs; the heavy half
moved from pre-merge to post-merge and nightly.

## Impact

- **Product scope:** none.
- **Architecture:** none.
- **Contracts and data:** none.
- **Security and trust boundary:** every static and policy gate stays on the
  pull request: secret scan, repository policy, dependency policy, workflow
  SHA pins, and the full Web Console suite, which carries the hostile-content
  and session-lifetime tests. The release blockers that need a browser or a
  container run on main and nightly.
- **Operations and release:** the required check is now a single name,
  `PR checks`. The branch protection list must be updated by the owner; under
  `CI-26` an agent cannot set it.

## Rollout and failure behavior

`quality.yml` and the selection script are deleted and replaced by `pr.yml`
and `full.yml`. Reverting is restoring one file from git history.

**The accepted risk, stated plainly:** a regression in Go integration,
container, contract, or browser behaviour is no longer caught before merge on
any pull request. It surfaces on the push to `main` or in the nightly sweep,
so the exposure window is at most one day and the offending commit is already
on `main` when it is found. Go vet, Go unit tests, and the whole Web Console
suite still run pre-merge, so compile errors, unit regressions, and
hostile-content failures are caught as before.

If regressions start reaching `main` in practice, the correction is to move a
specific suite back into `pr.yml`, one step at a time, rather than restoring
path-based selection.

## Owner decision record

- Decision: APPROVED
- Decided by: `@sinanganiz`
- Date: 2026-09-04
- Rationale: pull-request feedback time was slowing development out of
  proportion to the risk it caught, and the same coverage still runs on `main`
  and nightly. Security and policy gates stay unconditional.
