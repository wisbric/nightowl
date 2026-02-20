CREATE TABLE runbooks (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    title       TEXT NOT NULL,
    content     TEXT NOT NULL,
    category    TEXT,
    is_template BOOLEAN DEFAULT false,
    tags        TEXT[] DEFAULT '{}',
    created_by  UUID REFERENCES users(id),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_runbooks_category ON runbooks(category);
CREATE INDEX idx_runbooks_template ON runbooks(is_template) WHERE is_template = true;
