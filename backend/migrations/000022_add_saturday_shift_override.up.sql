-- Optional Saturday-specific timing per shift — the office runs shorter
-- hours on Saturday than Monday-Friday. Any field left null falls back to
-- that shift's regular weekday timing (see calculation.ShiftInfo.ResolvedFor).
ALTER TABLE shifts ADD COLUMN saturday_start_time time without time zone;
ALTER TABLE shifts ADD COLUMN saturday_end_time time without time zone;
ALTER TABLE shifts ADD COLUMN saturday_grace_period_minutes integer;
ALTER TABLE shifts ADD COLUMN saturday_min_working_hours numeric(4,2);
