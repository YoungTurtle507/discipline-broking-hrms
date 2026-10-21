CREATE TYPE daily_attendance_status AS ENUM (
    'PRESENT', 'ABSENT', 'HALF_DAY', 'HOLIDAY', 'WEEKLY_OFF', 'ON_LEAVE', 'INACTIVE'
);

CREATE TABLE attendance_calculations (
    id BIGSERIAL PRIMARY KEY,
    employee_id BIGINT NOT NULL REFERENCES employees(id),
    calc_date DATE NOT NULL,
    status daily_attendance_status NOT NULL,
    first_in TIMESTAMPTZ,
    last_out TIMESTAMPTZ,
    working_minutes INT NOT NULL DEFAULT 0,
    late_minutes INT NOT NULL DEFAULT 0,
    early_leave_minutes INT NOT NULL DEFAULT 0,
    overtime_minutes INT NOT NULL DEFAULT 0,
    exceptions TEXT[] NOT NULL DEFAULT '{}',
    calculation_version TEXT NOT NULL,
    calculated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (employee_id, calc_date)
);

CREATE INDEX idx_attendance_calc_employee_date ON attendance_calculations(employee_id, calc_date);
