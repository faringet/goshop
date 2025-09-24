-- +goose Up

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS sessions (
    id           UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL,
    refresh_hash BYTEA       NOT NULL,
    user_agent   TEXT,
    ip           INET,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at   TIMESTAMPTZ NOT NULL,
    rotated_at   TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ
    );

CREATE INDEX IF NOT EXISTS idx_sessions_user_id     ON sessions (user_id);
CREATE INDEX IF NOT EXISTS idx_sessions_expires_at  ON sessions (expires_at);
CREATE INDEX IF NOT EXISTS idx_sessions_revoked_at  ON sessions (revoked_at);

CREATE INDEX IF NOT EXISTS idx_sessions_active_partial
    ON sessions (expires_at)
    WHERE revoked_at IS NULL AND rotated_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_sessions_active_partial;
DROP INDEX IF EXISTS idx_sessions_revoked_at;
DROP INDEX IF EXISTS idx_sessions_expires_at;
DROP INDEX IF EXISTS idx_sessions_user_id;
DROP TABLE IF EXISTS sessions;
