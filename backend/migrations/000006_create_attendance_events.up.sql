CREATE TYPE attendance_direction AS ENUM ('IN', 'OUT');

CREATE TABLE attendance_events (
    id BIGSERIAL PRIMARY KEY,
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    direction attendance_direction NOT NULL,
    event_time TIMESTAMPTZ NOT NULL DEFAULT now(),
    confidence DOUBLE PRECISION NOT NULL,
    model_name TEXT NOT NULL,
    model_version TEXT NOT NULL,
    verified_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_attendance_events_employee ON attendance_events(employee_id);
CREATE INDEX idx_attendance_events_time ON attendance_events(event_time);

-- Rejected/failed verification attempts, kept separately from confirmed
-- events so a bad scan never looks like an attendance record — this is the
-- audit trail for "who tried to check in and failed" (plan: "Verification
-- logging").
CREATE TABLE attendance_verification_attempts (
    id BIGSERIAL PRIMARY KEY,
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    direction attendance_direction NOT NULL,
    matched BOOLEAN NOT NULL,
    confidence DOUBLE PRECISION NOT NULL,
    reason TEXT NOT NULL DEFAULT '',
    attempted_by BIGINT REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_attendance_attempts_employee ON attendance_verification_attempts(employee_id);
