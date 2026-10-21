-- Human Resources keeps exactly two designations: Employee Management and
-- Recruitment & Onboarding. HR Works was an extra generic title added when
-- the department was first split off Coding; anyone on it moves to Employee
-- Management before the row is removed, so nobody is left with a dangling
-- designation_id.
UPDATE employees
SET designation_id = (
        SELECT id FROM designations
        WHERE title = 'Employee Management'
          AND department_id = (SELECT id FROM departments WHERE name = 'Human Resources')
    ),
    updated_at = now()
WHERE designation_id = (
        SELECT id FROM designations
        WHERE title = 'HR Works'
          AND department_id = (SELECT id FROM departments WHERE name = 'Human Resources')
    );

DELETE FROM designations
WHERE title = 'HR Works'
  AND department_id = (SELECT id FROM departments WHERE name = 'Human Resources');
