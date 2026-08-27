-- name: GetAuthCredentialByUsername :one
SELECT user_id::text AS user_id,
       username,
       status,
       password_phc,
       totp_seed_envelope,
       last_totp_counter
FROM guardian_auth.users
WHERE username = $1;

-- name: GetAuthCredentialByUserID :one
SELECT user_id::text AS user_id,
       username,
       status,
       password_phc,
       totp_seed_envelope,
       last_totp_counter
FROM guardian_auth.users
WHERE user_id = $1;

-- name: ListAuthSessions :many
SELECT session_id::text AS session_id,
       s.user_id::text AS user_id,
       u.username,
       u.role,
       s.created_at,
       s.last_seen_at,
       s.expires_at,
       s.revoked_at
FROM guardian_auth.sessions s
JOIN guardian_auth.users u ON u.user_id = s.user_id
WHERE s.user_id = $1
ORDER BY s.created_at DESC, s.session_id DESC
LIMIT 200;
