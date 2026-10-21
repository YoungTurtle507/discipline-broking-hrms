-- A third attendance source alongside FACE (camera-verified) and MANUAL
-- (an admin correcting a missed scan): SELF, an employee self-reporting
-- their own IN/OUT with no verification at all. Only usable by an employee
-- whose record has remote_self_checkin enabled (see the next migration) —
-- e.g. someone working from home with no camera to scan against.
ALTER TYPE attendance_source ADD VALUE 'SELF';
