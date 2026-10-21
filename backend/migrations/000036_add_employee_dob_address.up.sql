-- Date of birth backs the upcoming-birthday reminder on the dashboard
-- (dashboard.Repository.UpcomingBirthdays); address is a plain free-text
-- field on the employee profile. Both nullable/blank by default since every
-- existing employee predates these fields.
ALTER TABLE employees
    ADD COLUMN date_of_birth DATE,
    ADD COLUMN address TEXT NOT NULL DEFAULT '';
