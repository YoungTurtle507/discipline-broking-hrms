-- Leave approval becomes Super Admin only. Admin and HR keep leave.manage
-- (they can still see every request) but lose leave.approve, so the Decide
-- endpoint — now gated on leave.approve specifically instead of sharing
-- leave.manage — rejects them. Super Admin is unaffected: it already holds
-- every permission and also bypasses checks entirely via IsSuperAdmin().
DELETE FROM role_permissions
WHERE permission_id = (SELECT id FROM permissions WHERE code = 'leave.approve')
  AND role_id IN (SELECT id FROM roles WHERE name IN ('ADMIN', 'HR'));
