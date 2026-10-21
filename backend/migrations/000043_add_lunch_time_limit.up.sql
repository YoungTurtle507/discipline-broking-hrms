-- Maximum lunch break length, in minutes, before it counts as Lunch OT for
-- that day. Compared against the gap between the LUNCH-tagged OUT and the
-- next IN.
ALTER TABLE calculation_settings ADD COLUMN lunch_time_limit_minutes INT NOT NULL DEFAULT 60;
