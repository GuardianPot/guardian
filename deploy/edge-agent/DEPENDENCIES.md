# Edge Agent dependency review

Reviewed for P1-W7 on 2026-08-23.

| Dependency | Pinned version | Purpose | Security/license note |
|---|---:|---|---|
| Go | 1.27.0 | Native Edge daemon | Approved by EN-01; static application build supported. |
| modernc.org/sqlite | v1.57.0 | Pure-Go SQLite WAL driver | Current module release at review time; BSD-3-Clause. No CGO/native SQLite runtime dependency. |
| go.opentelemetry.io/otel | v1.45.0 | Vendor-neutral trace API | Matches ADR 0014 and the Control Plane baseline; Apache-2.0. |
| systemd | Debian 13 platform package | Service supervision and sandboxing | Host platform dependency; no runtime socket is exposed to the daemon. |

Resolved transitive versions are pinned in `apps/edge-agent/go.sum`. The P1-W7
gate includes `go mod verify`, retraction/update inspection, vulnerability
analysis, race tests, and license review. Release SBOM production is a later
release-package responsibility; this inventory is the package evidence.

Verification results:

- `go list -m -u` reported no update for the direct SQLite or OpenTelemetry
  modules. No selected direct module or `modernc.org/libc` version is retracted.
- `govulncheck` v1.7.0 reported no vulnerabilities in the Edge module.
- `go-licenses` v2.0.1 accepted the runtime graph when the repository's own
  private module was excluded. Scanner warnings for assembly in `x/sys`,
  `modernc.org/libc`, and `xxhash/v2` were resolved by verifying their committed
  module-root BSD/MIT license files.
- `go mod verify` passed for every downloaded Edge module.

The SQLite v1.57.0 module's resolved modernc transitive set is retained rather
than overriding individual internals beyond the upstream-tested graph. This is
the newest direct SQLite release and is the safer supported unit to update.
