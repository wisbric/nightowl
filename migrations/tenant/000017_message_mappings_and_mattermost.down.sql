-- Reverse: rename message_mappings back to slack_message_mappings.
ALTER TABLE message_mappings DROP CONSTRAINT IF EXISTS uq_message_mappings_alert_provider;

ALTER TABLE message_mappings RENAME COLUMN message_id TO message_ts;
ALTER TABLE message_mappings RENAME COLUMN thread_id TO thread_ts;

DROP INDEX IF EXISTS idx_message_mappings_alert;
DROP INDEX IF EXISTS idx_message_mappings_channel_msg;

ALTER TABLE message_mappings DROP COLUMN IF EXISTS provider;

ALTER TABLE message_mappings RENAME TO slack_message_mappings;

CREATE INDEX idx_slack_mappings_alert ON slack_message_mappings(alert_id);
CREATE INDEX idx_slack_mappings_channel_ts ON slack_message_mappings(channel_id, message_ts);
