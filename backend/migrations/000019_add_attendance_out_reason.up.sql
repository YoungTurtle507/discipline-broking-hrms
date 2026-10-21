-- Between 12:00 and 15:00, marking OUT while already marked IN must say why:
-- HALF_DAY (leaving for the rest of the day) or BREAK (lunch/personal errand,
-- expected back the same day). Nullable — every other scan (a normal
-- morning IN, an evening OUT, a scan outside the window) leaves this null.
CREATE TYPE attendance_out_reason AS ENUM ('HALF_DAY', 'BREAK');

ALTER TABLE attendance_events ADD COLUMN out_reason attendance_out_reason;
