# Control Plane development runbook

## Preconditions

- Ubuntu 26.04 LTS development environment
- Go 1.27.0, Task 3.53.1, Docker/Compose
- no production credentials or signing material

## Repeatable acceptance gate

From the repository root:

```bash
task control-plane:integration
```

The harness uses the digest-pinned PostgreSQL 18.6 image, random runtime-only
credentials, unique databases, and a unique Compose project. It validates fresh
and idempotent migrations, a deliberate migration failure, transaction
rollback, process restart persistence, durable unfinished job metadata,
readiness/liveness, secret-free logs, and graceful shutdown.

## Manual development startup

Generate a random development password in your shell and do not commit it:

```bash
export GUARDIAN_POSTGRES_PASSWORD="guardian-$(tr -d '-' </proc/sys/kernel/random/uuid)"
export GUARDIAN_POSTGRES_PORT=55432
docker compose --file deploy/control-plane/compose.yaml --project-name guardian-control-plane-dev up --detach --wait
export GUARDIAN_DATABASE_URL="postgres://guardian:${GUARDIAN_POSTGRES_PASSWORD}@127.0.0.1:${GUARDIAN_POSTGRES_PORT}/guardian?sslmode=disable"
task control-plane:migrate
GOWORK=off go -C apps/control-plane run ./cmd/control-plane serve
```

The service never migrates at startup. An empty or out-of-date database makes
startup fail explicitly and keeps readiness false until `migrate` succeeds.

## Development reset and reseed

This project is in development and migrations are forward-only. There are no
down migrations or compatibility shims. To recover from an incompatible code
rollback, stop the service, revert the code, and explicitly destroy only the
named development Compose project volume:

```bash
docker compose --file deploy/control-plane/compose.yaml --project-name guardian-control-plane-dev down --volumes --remove-orphans
docker compose --file deploy/control-plane/compose.yaml --project-name guardian-control-plane-dev up --detach --wait
task control-plane:migrate
```

This permanently removes that development database. Never run this procedure
against pilot or production data. Seed commands will be added by the package
that first owns user/environment seed data; P1-W1 creates no product records.

## Runtime configuration

| Variable | Required | Default | Notes |
|---|---|---|---|
| `GUARDIAN_DATABASE_URL` | yes | none | Sensitive; never logged |
| `GUARDIAN_HTTP_ADDRESS` | no | `127.0.0.1:8080` | Set explicitly for container binding |
| `GUARDIAN_DATABASE_MAX_CONNS` | no | `10` | Must be positive |
| `GUARDIAN_SHUTDOWN_TIMEOUT` | no | `15s` | Minimum one second |
| `GUARDIAN_LOG_LEVEL` | no | `info` | `debug`, `info`, `warn`, or `error` |

`GET /livez` reports process liveness. `GET /readyz` returns only a sanitized
status and requires PostgreSQL connectivity plus the exact embedded migration
version. OpenTelemetry HTTP instrumentation uses the global provider and does
not make an external collector mandatory.

## Runtime image diagnostics

The release-style container uses the digest-pinned Distroless Debian 13 static
nonroot image. It has no interactive shell or package manager. Diagnose the
running artifact through structured logs, `/livez`, `/readyz`, OpenTelemetry,
and runtime metadata. If an interactive investigation is necessary, reproduce
the exact binary and configuration in a separately tagged, digest-pinned
`debug-nonroot` image or an authorized ephemeral debug container. Never promote
that debug artifact as the reviewed runtime image.

## Module and transaction boundary

The `auth`, `environment`, `devices`, `deception`, `health`, `audit`,
`jobs/outbox`, and `api` shells have explicit lifecycle packages. Future module
repositories may receive generated queries only for their own schema. Direct
cross-module table access is prohibited; multi-write operations use the
`database.Store.WithTx` boundary.
