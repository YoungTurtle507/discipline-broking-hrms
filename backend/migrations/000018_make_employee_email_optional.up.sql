-- Email is no longer collected up front when adding an employee; the unique
-- constraint stays (Postgres allows any number of NULLs under UNIQUE, so this
-- doesn't block multiple employees with no email on file).
ALTER TABLE employees ALTER COLUMN email DROP NOT NULL;
