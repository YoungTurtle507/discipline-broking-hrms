-- A declared Half Day (out_reason = 'HALF_DAY') on a day that's otherwise a
-- Holiday halves the Comp Off credit to 0.5 instead of a full 1 — same
-- declared-choice-at-scan-out signal Half Day already uses everywhere else,
-- not inferred from hours worked. See calculation.DailyResult.CompOffHalfDay.
ALTER TABLE attendance_calculations ADD COLUMN compoff_half_day BOOLEAN NOT NULL DEFAULT false;
