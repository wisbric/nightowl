CREATE TABLE users (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    external_id     TEXT NOT NULL UNIQUE,
    email           TEXT NOT NULL,
    display_name    TEXT NOT NULL,
    timezone        TEXT NOT NULL DEFAULT 'UTC',
    phone           TEXT,
    slack_user_id   TEXT,
    role            TEXT NOT NULL DEFAULT 'engineer',
    is_active       BOOLEAN NOT NULL DEFAULT true,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_external_id ON users(external_id);
