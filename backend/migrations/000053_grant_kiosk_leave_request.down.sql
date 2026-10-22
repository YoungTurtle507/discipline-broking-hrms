DELETE FROM role_permissions
WHERE role_id = (SELECT id FROM roles WHERE name = 'KIOSK')
  AND permission_id = (SELECT id FROM permissions WHERE code = 'leave.request_own');
