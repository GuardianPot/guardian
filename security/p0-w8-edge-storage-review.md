# P0-W8 edge storage security review

Status: Accepted for Phase 0 fixture scope

## Controls reviewed

- SQLite WAL mode and `synchronous=FULL` are applied on every open.
- Event IDs are unique. Reusing an ID with a different payload fails; the same
  ID and payload is an idempotent no-op.
- Claims are short-lived leases. An expired lease is returned to the FIFO
  queue, which provides crash recovery without acknowledging an uncommitted
  delivery.
- Claim attempt numbers fence late `Ack` and `Retry` calls from an expired
  worker, so an old process cannot mutate a newer lease for the same event.
- Delivery is acknowledged by a durable `delivered` tombstone. Replays and
  duplicate enqueue attempts therefore cannot create a second delivery.
- Resource ownership is explicit: the queue owns only the configured SQLite
  path. The W8 fixture uses a temporary directory and removes it on exit.
- Retry backoff keeps a failed event from starving other available events. The
  test fixture also proves FIFO order for 32 available events.

## Residual risks

The database path must be placed in the Edge agent's private data directory by
the deployment owner. Filesystem permissions, encryption at rest, and backup
retention are deployment concerns and are not silently inferred by this local
fixture.
