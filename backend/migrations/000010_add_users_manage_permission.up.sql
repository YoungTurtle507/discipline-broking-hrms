INSERT INTO permissions (code, description) VALUES
    ('users.manage', 'Create login accounts and assign roles');

-- SUPER_ADMIN doesn't need an explicit row: HasPermission() always passes
-- for that role regardless of role_permissions, by design (see models.go).
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'ADMIN' AND p.code = 'users.manage';
