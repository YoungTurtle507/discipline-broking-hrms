-- This duplicate's two-tier model: Super Admin (unchanged, bypasses every
-- permission check via IsSuperAdmin()) plus one new shared, non-personal
-- login role, KIOSK, used at the physical attendance-scanning station. KIOSK
-- can view (not edit) employees, register their faces, and mark/view
-- attendance — it deliberately does NOT get employees.manage, so it can
-- never create or edit an employee record.
INSERT INTO permissions (code, description) VALUES
    ('employees.view', 'View employee records (read-only)');

INSERT INTO roles (name) VALUES ('KIOSK');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'KIOSK' AND p.code IN ('employees.view', 'attendance.manage');
