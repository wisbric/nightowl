CREATE TABLE slack_message_mappings (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    alert_id    UUID REFERENCES alerts(id) ON DELETE CASCADE,
    incident_id UUID REFERENCES incidents(id) ON DELETE CASCADE,
    channel_id  TEXT NOT NULL,
    message_ts  TEXT NOT NULL,
    thread_ts   TEXT,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (alert_id IS NOT NULL OR incident_id IS NOT NULL)
);

CREATE INDEX idx_slack_mappings_alert ON slack_message_mappings(alert_id);
CREATE INDEX idx_slack_mappings_channel_ts ON slack_message_mappings(channel_id, message_ts);
