-- Tracks which reminder thresholds (1 day before, day of, 30 minutes before)
-- have already fired for a candidate's interview, so the lazy sweep that
-- runs on every notification poll (see main.go) never sends the same
-- reminder twice. Rescheduling a candidate's interview clears their rows
-- here (see candidate.Repository.Update), so the new time gets its own
-- fresh set of reminders.
CREATE TABLE candidate_interview_reminders (
    id BIGSERIAL PRIMARY KEY,
    candidate_id BIGINT NOT NULL REFERENCES candidates(id) ON DELETE CASCADE,
    reminder_type TEXT NOT NULL,
    sent_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (candidate_id, reminder_type)
);
