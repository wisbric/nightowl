CREATE TABLE escalation_events (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_id        UUID NOT NULL REFERENCES alerts(id) ON DELETE CASCADE,
    policy_id       UUID NOT NULL REFERENCES escalation_policies(id),
    tier            INTEGER NOT NULL,
    action          TEXT NOT NULL,
    target_user_id  UUID REFERENCES users(id),
    notify_method   TEXT,
    notify_result   TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_escalation_events_alert ON escalation_events(alert_id, created_at);
