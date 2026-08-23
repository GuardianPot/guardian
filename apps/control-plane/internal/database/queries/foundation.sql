-- name: PutServiceState :exec
INSERT INTO guardian_system.service_state (state_key, state_value, updated_at)
VALUES ($1, $2, now())
ON CONFLICT (state_key) DO UPDATE
SET state_value = EXCLUDED.state_value,
    updated_at = EXCLUDED.updated_at;

-- name: GetServiceState :one
SELECT state_key, state_value, updated_at
FROM guardian_system.service_state
WHERE state_key = $1;

-- name: CreateJob :exec
INSERT INTO guardian_jobs.jobs (job_id, job_type, payload, status, available_at)
VALUES ($1, $2, $3, 'queued', $4);

-- name: GetJob :one
SELECT job_id, job_type, payload, status, attempts, available_at,
       lease_owner, lease_until, created_at, updated_at
FROM guardian_jobs.jobs
WHERE job_id = $1;

-- name: AppendAuditRecord :one
INSERT INTO guardian_audit.records (
    actor_type, actor_id, action, object_type, object_id, metadata, trace_id
)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING sequence, occurred_at;
