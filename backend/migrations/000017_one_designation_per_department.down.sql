ALTER TABLE designations DROP CONSTRAINT designations_department_id_key;
ALTER TABLE designations ADD CONSTRAINT designations_department_title_key UNIQUE (department_id, title);

-- Recreates the designation row. Employees moved off it by the up migration
-- are not moved back — there is no record of who held it before — so this
-- restores the shape, not the historical assignment.
INSERT INTO designations (title, department_id)
VALUES ('Employee Management', (SELECT id FROM departments WHERE name = 'Human Resources'));
