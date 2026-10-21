DELETE FROM role_permissions WHERE permission_id = (SELECT id FROM permissions WHERE code = 'analytics.view');
DELETE FROM permissions WHERE code = 'analytics.view';
