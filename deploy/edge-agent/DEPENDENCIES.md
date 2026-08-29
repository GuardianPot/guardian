# Edge Agent dependency review

Reviewed for P1-W7/P1-W8 on 2026-08-23, P1-W5 on 2026-08-28, and the
P1-W9 read-only runtime probe on 2026-08-29.

| Dependency | Pinned version | Purpose | Security/license note |
|---|---:|---|---|
| Go | 1.27.0 | Native Edge daemon | Approved by EN-01; static application build supported. |
| modernc.org/sqlite | v1.57.0 | Pure-Go SQLite WAL driver | Current module release at review time; BSD-3-Clause. No CGO/native SQLite runtime dependency. |
| go.opentelemetry.io/otel | v1.46.0 | Vendor-neutral trace API | Matches ADR 0014 and the Control Plane baseline; Apache-2.0. |
| go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc | v0.71.0 | W3C trace context on the device channel | Apache-2.0; payloads and credentials are not recorded. |
| google.golang.org/grpc | v1.83.2 | Typed local helper RPC and remote mTLS device channel | Current secure patch release at review time; Apache-2.0. Each transport has distinct credentials and limits. |
| google.golang.org/protobuf | v1.36.12 | Generated privileged-helper messages | Current module/tool release at review time; BSD-3-Clause. |
| google.golang.org/grpc/cmd/protoc-gen-go-grpc | v1.6.2 | Pinned gRPC stub generator | Repository tool only; Apache-2.0. Declared with the Go `tool` directive. |
| golang.org/x/sys | v0.47.0 | `SO_PEERCRED` and pidfd Linux primitives | Direct P1-W8 dependency; BSD-3-Clause. |
| github.com/containerd/containerd/api | v1.11.1 | Version-only RPC contract used by the fixed privileged health probe | Apache-2.0. The helper uses only the parameterless Version RPC over `/run/containerd/containerd.sock`; no lifecycle client is linked. |
| golang.org/x/net | v0.58.0 | gRPC transport support | Explicit secure transitive selection after vulnerability review; BSD-3-Clause. |
| golang.org/x/text | v0.41.0 | gRPC HTTP/2 text support | Explicit secure transitive selection after vulnerability review; BSD-3-Clause. |
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
- P1-W8 pins `protoc-gen-go` through the Protobuf module and
  `protoc-gen-go-grpc` through its generator module. `buf generate` invokes
  both with `go tool`; no unpinned binary from `PATH` generates committed code.
- The privileged-helper service still exposes no reflection service, TCP
  listener, arbitrary command/path, raw nftables ruleset, or runtime lifecycle
  authority. Its P1-W9 runtime operation is parameterless and reads only the
  fixed containerd Version RPC. The separate P1-W5 TCP service is the
  authenticated product device plane and cannot invoke privileged-helper
  methods.

The SQLite v1.57.0 module's resolved modernc transitive set is retained rather
than overriding individual internals beyond the upstream-tested graph. This is
the newest direct SQLite release and is the safer supported unit to update.
