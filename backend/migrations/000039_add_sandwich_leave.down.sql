ALTER TABLE leave_requests DROP COLUMN IF EXISTS is_sandwich;
-- Postgres has no DROP VALUE for enums — SANDWICH_LEAVE_APPLIED is permanent
-- once added. Nothing to revert there.
