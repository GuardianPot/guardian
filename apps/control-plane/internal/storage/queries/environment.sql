-- name: GetOrganizationSingleton :one
SELECT organization_id::text AS organization_id, created_at
FROM guardian_environment.organizations
WHERE singleton = true;

-- name: ListEnvironments :many
SELECT e.environment_id::text AS environment_id,
       e.organization_id::text AS organization_id,
       e.display_name,
       e.revision,
       COUNT(z.zone_id)::bigint AS zone_count,
       CASE WHEN COUNT(z.zone_id) = 0 THEN 'needs_zones' ELSE 'zones_defined' END::text AS status,
       e.created_at,
       e.updated_at
FROM guardian_environment.environments e
LEFT JOIN guardian_environment.zones z ON z.environment_id = e.environment_id
GROUP BY e.environment_id
ORDER BY e.name_key, e.environment_id
LIMIT $1;

-- name: GetEnvironment :one
SELECT e.environment_id::text AS environment_id,
       e.organization_id::text AS organization_id,
       e.display_name,
       e.revision,
       COUNT(z.zone_id)::bigint AS zone_count,
       CASE WHEN COUNT(z.zone_id) = 0 THEN 'needs_zones' ELSE 'zones_defined' END::text AS status,
       e.created_at,
       e.updated_at
FROM guardian_environment.environments e
LEFT JOIN guardian_environment.zones z ON z.environment_id = e.environment_id
WHERE e.environment_id = $1
  AND e.organization_id = (SELECT organization_id FROM guardian_environment.organizations WHERE singleton = true)
GROUP BY e.environment_id;

-- name: GetEnvironmentForUpdate :one
SELECT environment_id::text AS environment_id,
       organization_id::text AS organization_id,
       display_name,
       name_key,
       revision,
       created_at,
       updated_at
FROM guardian_environment.environments
WHERE environment_id = $1
  AND organization_id = (SELECT organization_id FROM guardian_environment.organizations WHERE singleton = true)
FOR UPDATE;

-- name: CreateEnvironment :one
INSERT INTO guardian_environment.environments (organization_id, display_name, name_key)
VALUES (
    (SELECT organization_id FROM guardian_environment.organizations WHERE singleton = true),
    $1,
    $2
)
RETURNING environment_id::text AS environment_id,
          organization_id::text AS organization_id,
          display_name,
          revision,
          created_at,
          updated_at;

-- name: UpdateEnvironment :one
UPDATE guardian_environment.environments
SET display_name = $2,
    name_key = $3,
    revision = revision + 1,
    updated_at = clock_timestamp()
WHERE environment_id = $1
RETURNING environment_id::text AS environment_id,
          organization_id::text AS organization_id,
          display_name,
          revision,
          created_at,
          updated_at;

-- name: BumpEnvironmentRevision :exec
UPDATE guardian_environment.environments
SET revision = revision + 1,
    updated_at = clock_timestamp()
WHERE environment_id = $1;

-- name: LockEnvironmentZoneSet :exec
SELECT pg_advisory_xact_lock(hashtextextended(sqlc.arg(environment_id)::text, 1464554834));

-- name: ZoneOverlapExists :one
SELECT EXISTS (
    SELECT 1
    FROM guardian_environment.zones
    WHERE environment_id = sqlc.arg(environment_id)
      AND network && sqlc.arg(network)::cidr
      AND (sqlc.narg(exclude_zone_id)::uuid IS NULL OR zone_id <> sqlc.narg(exclude_zone_id)::uuid)
) AS overlaps;

-- name: ListZones :many
SELECT z.zone_id::text AS zone_id,
       z.environment_id::text AS environment_id,
       z.display_name,
       z.network::text AS cidr,
       z.revision,
       z.created_at,
       z.updated_at
FROM guardian_environment.zones z
WHERE z.environment_id = $1
  AND z.environment_id IN (
      SELECT e.environment_id
      FROM guardian_environment.environments e
      WHERE e.organization_id = (
          SELECT organization_id FROM guardian_environment.organizations WHERE singleton = true
      )
  )
ORDER BY z.name_key, z.zone_id
LIMIT $2;

-- name: GetZone :one
SELECT z.zone_id::text AS zone_id,
       z.environment_id::text AS environment_id,
       z.display_name,
       z.network::text AS cidr,
       z.revision,
       z.created_at,
       z.updated_at
FROM guardian_environment.zones z
JOIN guardian_environment.environments e ON e.environment_id = z.environment_id
WHERE z.environment_id = $1
  AND z.zone_id = $2
  AND e.organization_id = (SELECT organization_id FROM guardian_environment.organizations WHERE singleton = true);

-- name: GetZoneForUpdate :one
SELECT z.zone_id::text AS zone_id,
       z.environment_id::text AS environment_id,
       z.display_name,
       z.name_key,
       z.network::text AS cidr,
       z.revision,
       z.created_at,
       z.updated_at
FROM guardian_environment.zones z
JOIN guardian_environment.environments e ON e.environment_id = z.environment_id
WHERE z.environment_id = $1
  AND z.zone_id = $2
  AND e.organization_id = (SELECT organization_id FROM guardian_environment.organizations WHERE singleton = true)
FOR UPDATE OF z;

-- name: CreateZone :one
INSERT INTO guardian_environment.zones (environment_id, display_name, name_key, network)
VALUES (
    sqlc.arg(environment_id),
    sqlc.arg(display_name),
    sqlc.arg(name_key),
    sqlc.arg(network)::cidr
)
RETURNING zone_id::text AS zone_id,
          environment_id::text AS environment_id,
          display_name,
          network::text AS cidr,
          revision,
          created_at,
          updated_at;

-- name: UpdateZone :one
UPDATE guardian_environment.zones
SET display_name = sqlc.arg(display_name),
    name_key = sqlc.arg(name_key),
    network = sqlc.arg(network)::cidr,
    revision = revision + 1,
    updated_at = clock_timestamp()
WHERE environment_id = sqlc.arg(environment_id)
  AND zone_id = sqlc.arg(zone_id)
RETURNING zone_id::text AS zone_id,
          environment_id::text AS environment_id,
          display_name,
          network::text AS cidr,
          revision,
          created_at,
          updated_at;

-- name: DeleteZone :execrows
DELETE FROM guardian_environment.zones
WHERE environment_id = $1
  AND zone_id = $2;
