-- A third reason for marking out between 12:00 and 15:00: leaving early for
-- the day (as opposed to HALF_DAY, a formal half-day declaration, or BREAK, a
-- lunch/personal errand with an expected return). This OUT is still treated
-- as the day's final office-out, and the normal early-leave/half-day-cutoff
-- rules decide the day's status from the time alone — selecting this reason
-- never forces a status the way HALF_DAY does.
ALTER TYPE attendance_out_reason ADD VALUE 'EARLY_LEAVE';
