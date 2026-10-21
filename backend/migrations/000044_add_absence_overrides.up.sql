-- Lets a Super Admin manually excuse a specific absent day — e.g. the
-- employee actually called in sick but never filed a leave request. Row
-- existence is the flag: present means "treat this day as excused", deleted
-- means "back to the ordinary computed outcome". An excused day is treated
-- exactly like approved leave everywhere the engine's output is consumed
-- (leave-balance penalty, Reports, Employee Performance attendance %), not
-- just suppressed on the Attendance History screen — see
-- calculation.DayContext.IsExcusedAbsence.
CREATE TABLE absence_overrides (
    employee_id BIGINT NOT NULL REFERENCES employees(id) ON DELETE CASCADE,
    override_date DATE NOT NULL,
    note TEXT NOT NULL DEFAULT '',
    set_by BIGINT REFERENCES users(id),
    set_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (employee_id, override_date)
);
