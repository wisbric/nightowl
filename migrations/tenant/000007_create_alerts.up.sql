CREATE TABLE alerts (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    fingerprint             TEXT NOT NULL,
    status                  TEXT NOT NULL DEFAULT 'firing',
    severity                TEXT NOT NULL DEFAULT 'warning',
    source                  TEXT NOT NULL,
    title                   TEXT NOT NULL,
    description             TEXT,
    labels                  JSONB NOT NULL DEFAULT '{}',
    annotations             JSONB NOT NULL DEFAULT '{}',
    service_id              UUID REFERENCES services(id),
    matched_incident_id     UUID REFERENCES incidents(id),
    suggested_solution      TEXT,
    acknowledged_by         UUID REFERENCES users(id),
    acknowledged_at         TIMESTAMPTZ,
    resolved_by             UUID REFERENCES users(id),
    resolved_at             TIMESTAMPTZ,
    resolved_by_agent       BOOLEAN DEFAULT false,
    agent_resolution_notes  TEXT,
    occurrence_count        INTEGER NOT NULL DEFAULT 1,
    first_fired_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_fired_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    escalation_policy_id    UUID REFERENCES escalation_policies(id),
    current_escalation_tier INTEGER DEFAULT 0,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_alerts_fingerprint ON alerts(fingerprint);
CREATE INDEX idx_alerts_status ON alerts(status) WHERE status != 'resolved';
CREATE INDEX idx_alerts_severity ON alerts(severity);
CREATE INDEX idx_alerts_created ON alerts(created_at DESC);
CREATE INDEX idx_alerts_service ON alerts(service_id);
CREATE INDEX idx_alerts_labels ON alerts USING GIN(labels);
