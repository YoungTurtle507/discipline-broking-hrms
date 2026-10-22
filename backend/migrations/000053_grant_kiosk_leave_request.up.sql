-- The Leave module's apply flow uses the same shared-login + pick-your-name
-- pattern as the existing Mark Attendance flow: the KIOSK account submits a
-- leave request on behalf of whichever employee is picked, not "its own"
-- leave in any personal sense — same read as attendance.manage already
-- lets it mark attendance for anyone picked, not just itself. leave.request_own
-- already exists (seeded in 000001) and already covers this; KIOSK just
-- didn't hold it yet.
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id FROM roles r, permissions p
WHERE r.name = 'KIOSK' AND p.code = 'leave.request_own';
