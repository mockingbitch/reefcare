-- +goose Up
-- Identity, sessions, one-time auth tokens and the audit log (docs/database-design.md §3).

CREATE TABLE users (
    id                 uuid PRIMARY KEY DEFAULT uuidv7(),
    email              citext NOT NULL UNIQUE CHECK (char_length(email) BETWEEN 3 AND 254),
    email_verified_at  timestamptz,
    password_hash      text NOT NULL CHECK (char_length(password_hash) <= 512),
    -- Bumped on password change/reset and suspension. Sessions record the epoch they were
    -- created under and stop resolving once it changes, so "revoke all sessions" is atomic
    -- with the credential change even against logins that are already in flight.
    auth_epoch         int NOT NULL DEFAULT 1,
    display_name       text NOT NULL CHECK (char_length(display_name) BETWEEN 1 AND 60),
    username           citext UNIQUE CHECK (username ~ '^[a-z0-9_]{3,30}$'),
    bio                text CHECK (char_length(bio) <= 1000),
    experience_level   text CHECK (experience_level IN ('beginner', 'intermediate', 'advanced', 'professional')),
    locale             text NOT NULL DEFAULT 'vi' CHECK (locale IN ('vi', 'en')),
    timezone           text NOT NULL DEFAULT 'Asia/Ho_Chi_Minh', -- IANA name, validated by the service
    unit_prefs         jsonb NOT NULL DEFAULT '{}',
    notification_prefs jsonb NOT NULL DEFAULT '{}',
    status             text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'suspended', 'deactivated')),
    deactivated_at     timestamptz,
    created_at         timestamptz NOT NULL DEFAULT now(),
    updated_at         timestamptz NOT NULL DEFAULT now(),
    version            int NOT NULL DEFAULT 1, -- optimistic lock for profile edits only
    CHECK ((status = 'deactivated') = (deactivated_at IS NOT NULL))
);

-- Authorization roles. Plain users have no row; community rank is never stored here.
CREATE TABLE user_roles (
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    role       text NOT NULL CHECK (role IN ('moderator', 'administrator')),
    granted_by uuid REFERENCES users (id) ON DELETE SET NULL,
    granted_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, role)
);

-- Stored client networks must already be truncated: /24 (IPv4) or /48 (IPv6), host bits zero.
CREATE FUNCTION is_truncated_ip(p inet) RETURNS boolean
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    RETURN p = network(p)::inet AND masklen(p) <= CASE family(p) WHEN 4 THEN 24 ELSE 48 END;

CREATE TABLE user_sessions (
    id              uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id         uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    auth_epoch      int NOT NULL,
    token_hash      bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32), -- SHA-256 of the cookie token
    csrf_secret     bytea NOT NULL CHECK (octet_length(csrf_secret) = 32),
    created_at      timestamptz NOT NULL,
    last_seen_at    timestamptz NOT NULL,
    idle_expires_at timestamptz NOT NULL,
    expires_at      timestamptz NOT NULL,                                        -- absolute lifetime
    revoked_at      timestamptz,
    user_agent      text CHECK (char_length(user_agent) <= 512),
    ip_prefix       inet CHECK (ip_prefix IS NULL OR is_truncated_ip(ip_prefix)),
    CHECK (expires_at > created_at),
    CHECK (idle_expires_at <= expires_at)
);
CREATE INDEX user_sessions_active_by_user ON user_sessions (user_id) WHERE revoked_at IS NULL;
CREATE INDEX user_sessions_expires_at ON user_sessions (expires_at); -- purge job

CREATE TABLE auth_tokens (
    id         uuid PRIMARY KEY DEFAULT uuidv7(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    purpose    text NOT NULL CHECK (purpose IN ('verify_email', 'reset_password')),
    token_hash bytea NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    created_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    used_at    timestamptz, -- consumed by the user
    revoked_at timestamptz, -- superseded by a newer token or invalidated by a credential change
    CHECK (expires_at > created_at),
    CHECK (used_at IS NULL OR revoked_at IS NULL)
);
-- At most one live token per user and purpose; a concurrent second issue fails with 23505.
CREATE UNIQUE INDEX auth_tokens_one_live ON auth_tokens (user_id, purpose)
    WHERE used_at IS NULL AND revoked_at IS NULL;
CREATE INDEX auth_tokens_expires_at ON auth_tokens (expires_at); -- purge job

-- Append-only security audit trail (docs/security.md §8). Never store secrets in metadata.
CREATE TABLE audit_logs (
    id          bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    actor_id    uuid REFERENCES users (id) ON DELETE SET NULL,
    action      text NOT NULL CHECK (action ~ '^[a-z_]+(\.[a-z_]+)+$'),
    target_type text,
    target_id   uuid,
    metadata    jsonb NOT NULL DEFAULT '{}',
    request_id  text CHECK (char_length(request_id) <= 64),
    ip_prefix   inet CHECK (ip_prefix IS NULL OR is_truncated_ip(ip_prefix)),
    created_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_logs_by_actor ON audit_logs (actor_id, created_at DESC);
CREATE INDEX audit_logs_by_target ON audit_logs (target_type, target_id, created_at DESC);

-- Rows may not be deleted or changed, except actor_id becoming NULL through the
-- ON DELETE SET NULL above when an account is erased (docs/security.md §9).
-- +goose StatementBegin
CREATE FUNCTION audit_logs_append_only() RETURNS trigger
    LANGUAGE plpgsql AS
$$
BEGIN
    IF TG_OP = 'UPDATE'
       AND NEW.actor_id IS NULL
       AND (NEW.id, NEW.action, NEW.target_type, NEW.target_id, NEW.metadata, NEW.request_id, NEW.ip_prefix, NEW.created_at)
           IS NOT DISTINCT FROM
           (OLD.id, OLD.action, OLD.target_type, OLD.target_id, OLD.metadata, OLD.request_id, OLD.ip_prefix, OLD.created_at)
    THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'audit_logs is append-only' USING ERRCODE = 'insufficient_privilege';
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER audit_logs_append_only
    BEFORE UPDATE OR DELETE ON audit_logs
    FOR EACH ROW EXECUTE FUNCTION audit_logs_append_only();

-- +goose Down
DROP TABLE audit_logs;
DROP FUNCTION audit_logs_append_only();
DROP TABLE auth_tokens;
DROP TABLE user_sessions;
DROP FUNCTION is_truncated_ip(inet);
DROP TABLE user_roles;
DROP TABLE users;
