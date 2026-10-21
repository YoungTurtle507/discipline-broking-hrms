DELETE FROM role_permissions WHERE role_id = (SELECT id FROM roles WHERE name = 'KIOSK');
DELETE FROM roles WHERE name = 'KIOSK';
DELETE FROM permissions WHERE code = 'employees.view';
