-- Rename slack_message_mappings to message_mappings and add provider column.
ALTER TABLE slack_message_mappings RENAME TO message_mappings;

ALTER TABLE message_mappings ADD COLUMN provider TEXT NOT NULL DEFAULT 'slack';

-- Drop the old Slack-specific index and create new generic ones.
DROP INDEX IF EXISTS idx_slack_mappings_alert;
DROP INDEX IF EXISTS idx_slack_mappings_channel_ts;

-- Rename columns before creating indexes that reference the new names.
ALTER TABLE message_mappings RENAME COLUMN message_ts TO message_id;
ALTER TABLE message_mappings RENAME COLUMN thread_ts TO thread_id;

CREATE INDEX idx_message_mappings_alert ON message_mappings(alert_id);
CREATE INDEX idx_message_mappings_channel_msg ON message_mappings(channel_id, message_id);

-- Add unique constraint on (alert_id, provider).
ALTER TABLE message_mappings ADD CONSTRAINT uq_message_mappings_alert_provider UNIQUE (alert_id, provider);
