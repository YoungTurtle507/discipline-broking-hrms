DELETE FROM role_permissions WHERE permission_id = (SELECT id FROM permissions WHERE code = 'settings.calculation');
DELETE FROM permissions WHERE code = 'settings.calculation';
DROP TABLE IF EXISTS calculation_settings;
