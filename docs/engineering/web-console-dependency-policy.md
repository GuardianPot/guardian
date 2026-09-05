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

## Peer conflicts

**`--legacy-peer-deps` is forbidden.** It does not merely relax the failing
range; it stops npm installing peers generally, so an unrelated peer can
disappear from the lockfile without any error.

That is not hypothetical. On 2026-09-04 `openapi-typescript@7.13.0` was added
with the flag because it declares `peer typescript@^5.x` while the console pins
TypeScript `6.0.3`. The install silently dropped `@testing-library/dom`, a peer
of `@testing-library/react` from which `screen` and `waitFor` are re-exported.
Local runs still passed because the package remained in an existing
`node_modules`; CI's clean `npm ci` failed with thirteen "has no exported
member" errors.

When a dependency's peer range is genuinely stale, resolve it in this order:

1. Take a newer release of the dependency whose range includes our version.
2. Run the tool without declaring it, pinned at the point of use:
   `npx --yes <package>@<exact version>`. This suits development-only
   generators that run on demand and keeps the dependency tree untouched.
3. Stop and escalate. Do not reach for a resolution flag.

Whichever path is taken, declare every package the source actually imports,
including one reached through a re-export. `@testing-library/dom` is now an
explicit development dependency for that reason.

## Recorded exceptions

### `openapi-typescript` — resolved by option 2, 2026-09-04

`openapi-typescript@7.13.0` declares `peer typescript@^5.x` against our pinned
TypeScript `6.0.3`. It is **not** a declared dependency. `npm run generate:api`
and `tools/check-generated.mjs` both invoke it as
`npx --yes openapi-typescript@7.13.0`, so the version is pinned at the call
site and the dependency tree is unaffected.

The trade recorded knowingly: the generator is not lockfile-pinned, so it is
fetched at generation time rather than resolved from the committed tree. It is
development-only, contributes zero production bytes, and its output is
committed and verified byte-for-byte by the freshness check, so a substituted
generator cannot alter the shipped types without failing that check. Declare it
normally once its peer range includes TypeScript 6.
