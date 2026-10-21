-- Recreates the designation row. Employees moved off it by the up migration
-- are not moved back — there is no record of who held it before, only that
-- Employee Management was the fallback — so this restores the shape, not the
-- historical assignment.
INSERT INTO designations (title, department_id)
VALUES ('HR Works', (SELECT id FROM departments WHERE name = 'Human Resources'));
