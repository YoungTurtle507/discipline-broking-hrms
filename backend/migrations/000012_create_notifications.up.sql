CREATE TYPE notification_type AS ENUM (
    'LEAVE_REQUESTED',  -- to approvers: someone applied
    'LEAVE_APPROVED',   -- to the employee: their request was approved
    'LEAVE_REJECTED'    -- to the employee: their request was rejected
);

-- One row per recipient, not per event: a leave request notifies every
-- approver, and each of them reads (or doesn't) independently.
CREATE TABLE notifications (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type notification_type NOT NULL,
    title TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    -- Cascades so deleting a leave request can't leave a notification
    -- pointing at nothing.
    leave_request_id BIGINT REFERENCES leave_requests(id) ON DELETE CASCADE,
    -- Drives both the bell's unread count and the employee's decision popup:
    -- an approval should be announced once, not on every page load.
    read_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_notifications_recipient ON notifications(user_id, created_at DESC);
CREATE INDEX idx_notifications_unread ON notifications(user_id) WHERE read_at IS NULL;
