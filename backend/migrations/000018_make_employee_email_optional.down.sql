-- Reverting requires every existing row to already have an email — this will
-- fail if any employee was saved without one while this migration was applied.
ALTER TABLE employees ALTER COLUMN email SET NOT NULL;
