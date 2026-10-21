-- A single settings row (enforced by the id=1 check) holding every
-- calculation threshold a Super Admin can now edit from Organization. Default
-- values match this engine's own long-standing hardcoded constants exactly,
-- so installing this migration changes nothing until someone actually edits
-- a value.
CREATE TABLE calculation_settings (
    id SMALLINT PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    late_streak_limit INT NOT NULL DEFAULT 3,
    half_day_cutoff_hour INT NOT NULL DEFAULT 10,
    half_day_cutoff_minute INT NOT NULL DEFAULT 31,
    absent_cutoff_hour INT NOT NULL DEFAULT 14,
    absent_cutoff_minute INT NOT NULL DEFAULT 36,
    stay_late_pass_threshold_minutes INT NOT NULL DEFAULT 30,
    uninformed_leave_penalty_days INT NOT NULL DEFAULT 2,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by BIGINT REFERENCES users(id)
);

INSERT INTO calculation_settings (id) VALUES (1);

-- Editing these thresholds is Super Admin only — not even Admin, despite
-- Admin already holding settings.manage for Departments/Designations/Shifts.
-- No explicit grant is added for ADMIN or HR: Super Admin gets it through the
-- existing "every permission that exists" blanket grant, nobody else gets it
-- at all.
INSERT INTO permissions (code, description) VALUES
    ('settings.calculation', 'Edit the calculation engine''s configurable thresholds (late streak, cutoffs, penalties)');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'SUPER_ADMIN' AND p.code = 'settings.calculation';
