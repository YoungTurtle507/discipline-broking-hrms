-- How many minutes before an employee's own shift end still count as a
-- normal Office Out (no reason prompt) — the mirror image of a shift's own
-- start-of-day grace period.
ALTER TABLE calculation_settings ADD COLUMN end_of_shift_grace_minutes INT NOT NULL DEFAULT 10;
