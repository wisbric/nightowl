ALTER TABLE alerts DROP COLUMN IF EXISTS alert_group_id;
DROP TABLE IF EXISTS alert_groups;
DROP TABLE IF EXISTS alert_grouping_rules;
