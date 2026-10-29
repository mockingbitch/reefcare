-- name: InsertAuditLog :exec
-- metadata may be nil; it defaults to an empty object.
INSERT INTO audit_logs (actor_id, action, target_type, target_id, metadata, request_id, ip_prefix, created_at)
VALUES (@actor_id, @action, @target_type, @target_id, COALESCE(sqlc.narg(metadata)::jsonb, '{}'),
        @request_id, @ip_prefix, @now);
