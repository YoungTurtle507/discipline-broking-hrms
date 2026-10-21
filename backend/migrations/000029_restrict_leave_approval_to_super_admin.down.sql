INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name IN ('ADMIN', 'HR') AND p.code = 'leave.approve'
ON CONFLICT DO NOTHING;
