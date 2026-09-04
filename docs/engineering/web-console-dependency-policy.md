# Web Console dependency policy

Records the admission bar approved as `WC-D29` in
`docs/work-packages/phase-1_2-web-console-extended-multi-phase-work-packages/00-master-decision-record.md`.
It applies to every dependency added to `apps/web-console`.

## Admission record

A pull request that adds a dependency records this table for each one. A
reviewer who cannot answer a row from the pull request should reject it.

| Field | Requirement |
|---|---|
| Purpose | The decision or work package the dependency implements. A dependency with no decision behind it is not admissible. |
| Size | Minified and gzipped size, and its effect on the budgets in `WCX-07`. Development-only dependencies record `0` production bytes. |
| Licence | MIT, Apache-2.0, or BSD for production dependencies. MPL-2.0 is permitted for development-only dependencies. Anything else stops and escalates. |
| Install scripts | Whether the package or its transitive graph runs install scripts. Unreviewed install scripts are denied. |
| Transitive count | Number of packages the addition brings in. |
| Maintenance | Evidence the project is maintained: recent releases, an active issue tracker, and a security-reporting path. |

## Standing rules

1. Versions are exact in `package.json` and resolved in the committed
   lockfile. Ranges are not permitted.
2. No dependency is loaded from a CDN at runtime. Everything ships from the
   same origin as the console.
3. A production dependency that duplicates something the console already has
   is rejected. Two libraries doing one job is a maintenance cost with no
   product value.
4. Headless utilities are not a second design system and do not reopen
   `WC-D09`. A dependency that brings its own visual language does.
5. Development-only dependencies still record licence and install-script
   status, because they run on developer and CI machines.
6. Removing a dependency needs no record beyond the pull request description.

## Current direct dependencies

Production dependencies as of `WCX-01`:

| Package | Purpose | Decision |
|---|---|---|
| `react`, `react-dom` | UI runtime | TS-02 |
| `react-router-dom` | Explicit route tree | TS-05 |
| `@tanstack/react-query` | Server state | TS-04 |
| `@radix-ui/react-dialog`, `@radix-ui/react-label` | Accessible primitives | W11-C2-A, WC-D09 |

`WCX-04` consolidates the two Radix packages onto the single `radix-ui`
package. `WCX-02`, `WCX-06`, `WCX-11`, `WCX-12`, and `WCX-19` each add
dependencies their specifications name; none may be added ahead of its
package.

## Recorded exceptions

### `openapi-typescript` peer range — 2026-09-04

`openapi-typescript@7.13.0` declares `peer typescript@^5.x` while the console
pins TypeScript `6.0.3`, so a plain `npm install` reports `ERESOLVE`. It was
added with `--legacy-peer-deps`.

This is a stale peer range, not a functional incompatibility. The evidence:

- the generator produces correct output against the pinned TypeScript, and
  that output is then typechecked by the same TypeScript in `tsc -b`, which is
  the real compatibility proof;
- `npm ci`, which is what CI runs, resolves the committed lockfile without the
  flag, so the continuous-integration path is unaffected;
- the package is development-only and contributes zero production bytes.

The cost is that a developer running a fresh `npm install` must pass
`--legacy-peer-deps`. Remove this exception once `openapi-typescript` widens
its peer range to include TypeScript 6.

A future peer conflict follows the same rule as a licence: record it here with
evidence that it is a stale range rather than an incompatibility, or stop and
escalate.
