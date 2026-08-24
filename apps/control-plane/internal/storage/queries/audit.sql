-- The two-key advisory-lock namespace is (0x47554152 = "GUAR", 1 = audit
-- append/anchor protocol v1). The first-page reader takes the exclusive side.
-- name: AcquireAuditAnchorLock :exec
SELECT pg_advisory_xact_lock(1196769618, 1);

-- name: AppendAuditRecord :one
WITH audit_append_lock AS MATERIALIZED (
    -- Appends take the shared form so they remain concurrent. Materialization
    -- and the INSERT's dependency on this row guarantee the lock is acquired
    -- before PostgreSQL evaluates the identity default for the new record.
    SELECT pg_advisory_xact_lock_shared(1196769618, 1)
)
INSERT INTO guardian_audit.records (
    occurred_at,
    actor_type,
    actor_id,
    action,
    object_type,
    object_id,
    correlation_id,
    request_id,
    before_snapshot,
    after_snapshot
)
SELECT
    sqlc.arg(occurred_at),
    sqlc.arg(actor_type),
    sqlc.arg(actor_id),
    sqlc.arg(action),
    sqlc.arg(object_type),
    sqlc.arg(object_id),
    sqlc.arg(correlation_id),
    sqlc.narg(request_id),
    sqlc.narg(before_snapshot),
    sqlc.narg(after_snapshot)
FROM audit_append_lock
RETURNING
    event_id::text AS event_id,
    sequence,
    occurred_at,
    recorded_at,
    actor_type,
    actor_id,
    action,
    object_type,
    object_id,
    correlation_id,
    request_id,
    before_snapshot,
    after_snapshot;

-- name: GetAuditAnchor :one
SELECT COALESCE(MAX(sequence), 0)::bigint AS anchor_sequence
FROM guardian_audit.records;

-- name: ListAuditRecords :many
SELECT
    event_id::text AS event_id,
    sequence,
    occurred_at,
    recorded_at,
    actor_type,
    actor_id,
    action,
    object_type,
    object_id,
    correlation_id,
    request_id,
    before_snapshot,
    after_snapshot
FROM guardian_audit.records
WHERE sequence <= sqlc.arg(anchor_sequence)
  AND (
      sqlc.arg(position_sequence)::bigint = 0
      OR sequence < sqlc.arg(position_sequence)
  )
  AND (
      sqlc.arg(filter_action)::text = ''
      OR action = sqlc.arg(filter_action)
  )
  AND (
      sqlc.arg(filter_correlation_id)::text = ''
      OR correlation_id = sqlc.arg(filter_correlation_id)
  )
  AND (
      sqlc.arg(filter_object_type)::text = ''
      OR (
          object_type = sqlc.arg(filter_object_type)
          AND object_id = sqlc.arg(filter_object_id)
      )
  )
ORDER BY sequence DESC
LIMIT sqlc.arg(page_size);
