CREATE TABLE IF NOT EXISTS apps (
    id UUID PRIMARY KEY NOT NULL,
    name   TEXT NOT NULL UNIQUE,
    secret BYTEA NOT NULL
);

CREATE TABLE IF NOT EXISTS users (
    id        UUID PRIMARY KEY NOT NULL,
    email     TEXT NOT NULL UNIQUE,
    pass_hash TEXT NOT NULL,
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS admins (
    id        UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    is_admin  BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    app_id UUID NOT NULL REFERENCES apps(id) ON DELETE CASCADE,
    token TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    revoked BOOLEAN NOT NULL DEFAULT FALSE,
    ip_address INET NOT NULL
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_app_id ON refresh_tokens(app_id);