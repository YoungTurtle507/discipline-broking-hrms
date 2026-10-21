-- Two-band late-arrival streak system, replacing the old single late
-- window/streak (LateStreakLimit stays, but only as a legacy fallback until
-- the engine itself is updated to read these instead — see calculation
-- policy.go). Both *_end_minutes are measured from the employee's own shift
-- start, not a fixed clock time, so a 9AM shift and a 10AM shift each get
-- their own window.
--
-- Band 1: shift start to late_band1_end_minutes later — every
-- late_band1_streak_limit-th late arrival in this band, this month, is a
-- strike. Band 2: from there to late_band2_end_minutes later — every
-- late_band2_streak_limit-th late arrival in this band, this month, is a
-- strike (a shorter fuse than Band 1, since it's later arrivals).
ALTER TABLE calculation_settings ADD COLUMN late_band1_end_minutes INT NOT NULL DEFAULT 30;
ALTER TABLE calculation_settings ADD COLUMN late_band1_streak_limit INT NOT NULL DEFAULT 3;
ALTER TABLE calculation_settings ADD COLUMN late_band2_end_minutes INT NOT NULL DEFAULT 90;
ALTER TABLE calculation_settings ADD COLUMN late_band2_streak_limit INT NOT NULL DEFAULT 2;
