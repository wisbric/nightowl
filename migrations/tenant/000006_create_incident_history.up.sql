CREATE TABLE incident_history (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    incident_id UUID NOT NULL REFERENCES incidents(id) ON DELETE CASCADE,
    changed_by  UUID REFERENCES users(id),
    change_type TEXT NOT NULL,
    diff        JSONB NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_incident_history_incident ON incident_history(incident_id, created_at DESC);
