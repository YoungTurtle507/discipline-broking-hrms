-- Per-employee flag, not a role-wide feature: office employees still must
-- clock in by face scan, but someone with no camera to scan against (e.g.
-- working from home) can be individually opted into self-reporting their own
-- attendance instead. An admin toggles this on the employee's profile.
ALTER TABLE employees ADD COLUMN remote_self_checkin boolean NOT NULL DEFAULT false;
