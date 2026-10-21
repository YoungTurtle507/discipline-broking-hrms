-- Every department now has exactly one designation, so the two are managed
-- as a single unit. Human Resources keeps Recruitment & Onboarding; anyone on
-- Employee Management moves there first so nobody is left with a dangling
-- designation_id, then that row is removed.
UPDATE employees
SET designation_id = (
        SELECT id FROM designations
        WHERE title = 'Recruitment & Onboarding'
          AND department_id = (SELECT id FROM departments WHERE name = 'Human Resources')
    ),
    updated_at = now()
WHERE designation_id = (
        SELECT id FROM designations
        WHERE title = 'Employee Management'
          AND department_id = (SELECT id FROM departments WHERE name = 'Human Resources')
    );

DELETE FROM designations
WHERE title = 'Employee Management'
  AND department_id = (SELECT id FROM departments WHERE name = 'Human Resources');

-- Replace the (department_id, title) uniqueness with a straight uniqueness on
-- department_id — a department can now have at most one designation, not
-- just no duplicate titles within it.
ALTER TABLE designations DROP CONSTRAINT designations_department_title_key;
ALTER TABLE designations ADD CONSTRAINT designations_department_id_key UNIQUE (department_id);
