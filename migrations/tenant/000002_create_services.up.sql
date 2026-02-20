CREATE TABLE services (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name        TEXT NOT NULL,
    cluster     TEXT,
    namespace   TEXT,
    description TEXT,
    owner_id    UUID REFERENCES users(id),
    tier        TEXT DEFAULT 'standard',
    metadata    JSONB DEFAULT '{}',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(name, cluster, namespace)
);
