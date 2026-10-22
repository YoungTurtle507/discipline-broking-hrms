-- A manual correction submitted by the kiosk login no longer records
-- directly — it becomes a request a Super Admin must approve first. (A
-- Super Admin's own correction still records immediately; see the Go
-- handler, not this table — there's nothing to request-and-approve when
-- the approver is the one submitting it.)
CREATE TYPE manual_request_status AS ENUM ('PENDING', 'APPROVED', 'REJECTED');

CREATE TABLE attendance_manual_requests (
    id SERIAL PRIMARY KEY,
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    direction attendance_direction NOT NULL,
    event_time TIMESTAMPTZ NOT NULL,
    note TEXT NOT NULL,
    out_reason attendance_out_reason,
    status manual_request_status NOT NULL DEFAULT 'PENDING',
    requested_by BIGINT NOT NULL REFERENCES users(id),
    reviewed_by BIGINT REFERENCES users(id),
    reviewed_at TIMESTAMPTZ,
    -- Set once approval creates the real attendance_events row, so the
    -- request and the event it produced stay traceable to each other.
    created_event_id BIGINT REFERENCES attendance_events(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- The Super Admin panel's own primary view: open requests, oldest first.
CREATE INDEX idx_attendance_manual_requests_pending
    ON attendance_manual_requests (created_at)
    WHERE status = 'PENDING';
