-- Employee performance analytics (Best/Worst leaderboards by attendance,
-- punctuality, working time) is Super Admin only — nobody sees another
-- employee's standing against their peers by default, not even Admin/HR.
-- Same pattern as settings.calculation: no explicit grant for ADMIN or HR,
-- Super Admin gets it through the existing "every permission" blanket grant.
INSERT INTO permissions (code, description) VALUES
    ('analytics.view', 'View the employee performance analytics leaderboards');

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'SUPER_ADMIN' AND p.code = 'analytics.view';
