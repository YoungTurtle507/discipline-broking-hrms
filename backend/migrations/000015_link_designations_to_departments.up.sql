-- Designations now belong to exactly one department (Coding won't offer a
-- Research designation from another department), so the two management
-- panels collapse into one and an employee's designation choices come from
-- their selected department.
ALTER TABLE designations ADD COLUMN department_id BIGINT REFERENCES departments(id);

-- A title was unique system-wide; now it only has to be unique within its
-- own department, since e.g. "Team Lead" may reasonably exist in more than
-- one department later.
ALTER TABLE designations DROP CONSTRAINT designations_title_key;

-- The original migration here also seeded that company's specific
-- department/designation structure (Coding/Dealer/Human Resources etc.) and
-- backfilled its own pre-existing rows to match. None of that applies to
-- this duplicate's fresh database — departments/designations is empty at
-- this point, so SET NOT NULL below is a no-op-safe schema change, and this
-- duplicate seeds its own minimal default (one department/designation/shift)
-- separately via cmd/seedkiosk instead.
ALTER TABLE designations ALTER COLUMN department_id SET NOT NULL;
ALTER TABLE designations ADD CONSTRAINT designations_department_title_key UNIQUE (department_id, title);
CREATE INDEX idx_designations_department ON designations(department_id);
