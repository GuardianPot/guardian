# P1-W1 dependency and license inventory

Resolved versions are committed in the relevant `go.mod` and `go.sum` files.

| Dependency | Version | Scope | License |
|---|---:|---|---|
| Go | 1.27.0 | Runtime/toolchain | BSD-3-Clause |
| Go build image | `golang:1.27-bookworm` (digest pinned) | Container build | BSD-3-Clause and Debian package licenses |
| Distroless runtime image | `static-debian13:nonroot` (digest pinned) | Container runtime | Apache-2.0 project and bundled Debian package licenses |
| PostgreSQL image | 18.6-trixie (digest pinned) | Development/integration database | PostgreSQL License |
| `github.com/jackc/pgx/v5` | 5.10.0 | PostgreSQL driver/pool | MIT |
| `github.com/pressly/goose/v3` | 3.27.3 | Embedded forward migrations | MIT |
| `github.com/sqlc-dev/sqlc` | 1.31.1 | Isolated repository tool module | MIT |
| `go.opentelemetry.io/otel` | 1.45.0 | HTTP trace/metric API | Apache-2.0 |
| `otelhttp` | 0.70.0 | `net/http` instrumentation | Apache-2.0 |

`sqlc` and the upstream goose CLI are tool-only modules under
`apps/control-plane/tools/`; they are not linked into the Control Plane runtime.
The runtime and goose tool modules explicitly raise retracted transitive
`modernc.org/libc` 1.74.3 to fixed 1.74.4. The sqlc tool module raises its
security-sensitive transitive graph to `cel-go` 0.30.0, gRPC 1.83.1,
`x/net` 0.58.0, and `x/text` 0.41.0.

The Control Plane is built with `CGO_ENABLED=0` and runs on the Debian 13
distroless static nonroot image. The selected multi-platform digest is verified
before adoption, and the resulting image must remain free of high or critical
OS and Go binary findings at review time.
