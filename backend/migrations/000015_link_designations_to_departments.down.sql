DROP INDEX idx_designations_department;
ALTER TABLE designations DROP CONSTRAINT designations_department_title_key;
ALTER TABLE designations ALTER COLUMN department_id DROP NOT NULL;
ALTER TABLE designations DROP COLUMN department_id;
ALTER TABLE designations ADD CONSTRAINT designations_title_key UNIQUE (title);
UPDATE departments SET name = 'HR', updated_at = now() WHERE name = 'Human Resources';
-- The Dealer department, the new designation rows, and the Software Developer
-- rename are left in place: they are additive data, not a shape the schema
-- needs reversed, and removing them risks deleting rows a real employee has
-- since come to reference.
