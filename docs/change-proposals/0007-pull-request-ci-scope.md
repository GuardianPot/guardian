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

**Option 2**, with these constraints as part of the approval:

1. The fast gate is unconditional on every pull request: repository policy,
   Markdown format, contract layout, generated-artifact freshness, dependency
   policy, workflow SHA pins, and the secret scan.
2. Area selection is computed from the diff by `.github/scripts/changed-areas.sh`
   using `git diff` only, so no new third-party action is introduced and
   `CI-03` is unaffected.
3. Selection fails safe where it matters. A change to `.github/workflows/`
   or `.github/scripts/` runs every area, because those decide what runs at
   all, as does an unavailable base commit. Node manifests select only the
   areas that consume them, so a console change that also edits
   `Taskfile.yml` no longer drags in the Go and container suites.
4. Nothing is deleted. A push to `main`, the nightly schedule, and manual
   dispatch all set `FULL=true`, which runs every job regardless of paths and
   restores the three-engine browser matrix.
5. A pull request that touches the console runs the browser flow in Chromium
   only. `CI-18` keeps Playwright E2E required; this narrows the per-pull-request
   engine matrix, not the requirement.
6. Caches cover the pinned Go tools and the Playwright engines, under `CI-07`.

## Impact

- **Product scope:** none.
- **Architecture:** none.
- **Contracts and data:** none.
- **Security and trust boundary:** the secret scan, repository policy, workflow
  pin check, and dependency policy remain unconditional on every pull request,
  so no security gate is weakened for any change. Hostile-rendering and
  session tests live in the web job, which runs whenever console code changes.
- **Operations and release:** required-check names change, so the branch
  protection required-check list must be updated by the owner. Under `CI-26`
  that list is owner-controlled and cannot be set by an agent.

## Rollout and failure behavior

The workflow replaces the previous single job in one commit. Reverting is a
single file revert plus deleting the selection script.

**The accepted risk, stated plainly:** a regression in Go, container, contract,
or cross-engine browser behaviour introduced by a pull request that does not
touch those paths is no longer caught before merge. It is caught on the push to
`main` or by the nightly sweep, so the exposure window is at most one day and
the offending commit is already on `main` when it is found. Options 1 and 3 sit
either side of that trade; option 2 was chosen because the paths are
well-separated in this repository and the fail-safe in constraint 3 covers the
cases where they are not.

If cross-path regressions turn out to reach `main` in practice, the correction
is to widen the selection patterns in the script, not to restore the single
job.

## Owner decision record

- Decision: APPROVED
- Decided by: `@sinanganiz`
- Date: 2026-09-04
- Rationale: pull-request feedback time was slowing development out of
  proportion to the risk it caught, and the same coverage still runs on `main`
  and nightly. Security and policy gates stay unconditional.
