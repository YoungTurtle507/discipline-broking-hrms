-- Lets a self-service session call the mark-my-own-attendance endpoint at
-- all; the actual employee-by-employee gate is remote_self_checkin (see the
-- previous migration) — this permission alone doesn't enable it.
INSERT INTO permissions (code, description) VALUES
    ('attendance.mark_own', 'Self-report own attendance (only where enabled per employee)');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'EMPLOYEE' AND p.code = 'attendance.mark_own';
