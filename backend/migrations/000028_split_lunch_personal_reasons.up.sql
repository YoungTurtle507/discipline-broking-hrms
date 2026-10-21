-- BREAK ("Lunch / Personal") is being split into two distinct reasons so the
-- timesheet can say which kind of mid-day break it was. The old BREAK value
-- stays in the enum (existing rows keep it, and Postgres can't drop an enum
-- value anyway) but is no longer offered by the app going forward.
ALTER TYPE attendance_out_reason ADD VALUE 'LUNCH';
ALTER TYPE attendance_out_reason ADD VALUE 'PERSONAL';
