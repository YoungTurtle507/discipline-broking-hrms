-- Lunch Overtime streak: a LUNCH-tagged break running past
-- lunch_time_limit_minutes (LUNCH_OT) already gets flagged every time it
-- happens — this adds a monthly strike count on top of that, mirroring the
-- Late Streak system. Every lunch_ot_streak_limit-th Lunch OT day in a
-- month deducts lunch_ot_penalty_days from the employee's leave balance
-- (a fixed ledger entry, same mechanism as the Uninformed Leave penalty —
-- unlike Late Streak, a long lunch doesn't turn the rest of that day's
-- attendance into a Half Day on its own).
ALTER TABLE calculation_settings ADD COLUMN lunch_ot_streak_limit INT NOT NULL DEFAULT 3;
ALTER TABLE calculation_settings ADD COLUMN lunch_ot_penalty_days NUMERIC(4,2) NOT NULL DEFAULT 0.5;
