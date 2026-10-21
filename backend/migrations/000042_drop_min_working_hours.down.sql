ALTER TABLE shifts ADD COLUMN min_working_hours NUMERIC(4,2) NOT NULL DEFAULT 8.0;
ALTER TABLE shifts ADD COLUMN saturday_min_working_hours NUMERIC(4,2);
