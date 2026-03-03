-- Fix: alerts.alert_group_id should SET NULL on group deletion, not RESTRICT.
ALTER TABLE alerts DROP CONSTRAINT IF EXISTS alerts_alert_group_id_fkey;
ALTER TABLE alerts ADD CONSTRAINT alerts_alert_group_id_fkey
    FOREIGN KEY (alert_group_id) REFERENCES alert_groups(id) ON DELETE SET NULL;
