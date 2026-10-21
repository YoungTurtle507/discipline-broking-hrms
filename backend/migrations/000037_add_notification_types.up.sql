-- notification_type is a Postgres enum, and LEAVE_SUBMITTED (added to the Go
-- code earlier without a matching migration) was never actually added to it —
-- every attempt to insert that notification type has been erroring out and
-- rolling back the ENTIRE leave submission ever since, which is a real bug,
-- not the timing issue it first looked like. This backfills the missing
-- value and adds the three new interview-reminder types in the same pass.
ALTER TYPE notification_type ADD VALUE IF NOT EXISTS 'LEAVE_SUBMITTED';
ALTER TYPE notification_type ADD VALUE IF NOT EXISTS 'INTERVIEW_REMINDER_1_DAY';
ALTER TYPE notification_type ADD VALUE IF NOT EXISTS 'INTERVIEW_REMINDER_DAY_OF';
ALTER TYPE notification_type ADD VALUE IF NOT EXISTS 'INTERVIEW_REMINDER_30_MIN';
